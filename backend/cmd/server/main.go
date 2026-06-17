package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/marines-dev/inventory-portal/internal/audit"
	"github.com/marines-dev/inventory-portal/internal/database"
	"github.com/marines-dev/inventory-portal/internal/middleware"
)

func hasPermission(c *gin.Context, permission string) bool {
	role, _ := c.Get("role")
	if role == "super_admin" {
		return true
	}
	perms, exists := c.Get("permissions")
	if !exists {
		return false
	}
	permList, ok := perms.([]string)
	if !ok {
		return false
	}
	for _, p := range permList {
		if p == permission {
			return true
		}
	}
	return false
}

func main() {
	// Load environment variables from .env
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "3014"
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "22c3b650de1658b27a286d4e00f10dee9232cacf847e9a6c387a1e616aed94f6"
	}

	// Initialize DB Connection
	database.InitDB()

	r := gin.Default()

	// CORS Middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Tenant-ID")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "UP", "service": "Inventory Management System"})
	})

	// Protected routes
	api := r.Group("/api/v1")
	api.Use(middleware.JWTAuth(secret))
	{
		api.GET("/inventory", func(c *gin.Context) {
			if !hasPermission(c, "inventory:read") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
				return
			}
			companyID := c.Query("company_id")
			query := database.DB
			if companyID != "" {
				parsed, err := uuid.Parse(companyID)
				if err == nil {
					query = query.Where("company_id = ?", parsed)
				}
			}
			var list []database.InventoryItem
			if err := query.Order("name asc").Find(&list).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to fetch inventory items"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
		})

		api.POST("/inventory", func(c *gin.Context) {
			if !hasPermission(c, "inventory:create") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
				return
			}
			var item database.InventoryItem
			if err := c.ShouldBindJSON(&item); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
				return
			}
			item.ID = uuid.New()
			if err := database.DB.Create(&item).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to create inventory item"})
				return
			}
			authHeader := c.GetHeader("Authorization")
			entityIDStr := item.ID.String()
			detailsBytes, _ := json.Marshal(item)
			go audit.SendAuditLog(authHeader, "create_inventory_item", "inventory", &entityIDStr, string(detailsBytes))
			c.JSON(http.StatusCreated, gin.H{"success": true, "data": item})
		})

		api.PUT("/inventory/:id", func(c *gin.Context) {
			if !hasPermission(c, "inventory:update") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
				return
			}
			idStr := c.Param("id")
			id, err := uuid.Parse(idStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid ID format"})
				return
			}
			var existing database.InventoryItem
			if err := database.DB.First(&existing, id).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Inventory item not found"})
				return
			}
			var input database.InventoryItem
			if err := c.ShouldBindJSON(&input); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
				return
			}
			existing.Name = input.Name
			existing.PartNumber = input.PartNumber
			existing.Quantity = input.Quantity
			existing.Unit = input.Unit
			existing.Location = input.Location
			existing.MinimumStock = input.MinimumStock
			existing.CompanyID = input.CompanyID
			if err := database.DB.Save(&existing).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to update inventory item"})
				return
			}
			authHeader := c.GetHeader("Authorization")
			entityIDStr := existing.ID.String()
			detailsBytes, _ := json.Marshal(existing)
			go audit.SendAuditLog(authHeader, "update_inventory_item", "inventory", &entityIDStr, string(detailsBytes))
			c.JSON(http.StatusOK, gin.H{"success": true, "data": existing})
		})

		api.DELETE("/inventory/:id", func(c *gin.Context) {
			if !hasPermission(c, "inventory:delete") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
				return
			}
			idStr := c.Param("id")
			id, err := uuid.Parse(idStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid ID format"})
				return
			}
			if err := database.DB.Delete(&database.InventoryItem{}, id).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to delete inventory item"})
				return
			}
			authHeader := c.GetHeader("Authorization")
			go audit.SendAuditLog(authHeader, "delete_inventory_item", "inventory", &idStr, fmt.Sprintf(`{"id": "%s"}`, idStr))
			c.JSON(http.StatusOK, gin.H{"success": true, "message": "Inventory item deleted successfully"})
		})

		// Master Items CRUD API
		api.GET("/master-items", func(c *gin.Context) {
			role, _ := c.Get("role")
			isAdmin := role == "super_admin" || role == "company_admin" || role == "admin"
			if !isAdmin && !hasPermission(c, "inventory:read") && !hasPermission(c, "master_items:read") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
				return
			}
			companyID := c.Query("company_id")
			query := database.DB
			if companyID != "" {
				parsed, err := uuid.Parse(companyID)
				if err == nil {
					query = query.Where("company_id = ?", parsed)
				}
			}
			var list []database.MasterItem
			if err := query.Order("name asc").Find(&list).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to fetch master items"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
		})

		api.POST("/master-items", func(c *gin.Context) {
			role, _ := c.Get("role")
			isAdmin := role == "super_admin" || role == "company_admin" || role == "admin"
			if !isAdmin && !hasPermission(c, "inventory:create") && !hasPermission(c, "master_items:create") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
				return
			}
			var item database.MasterItem
			if err := c.ShouldBindJSON(&item); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
				return
			}
			item.ID = uuid.New()
			if err := database.DB.Create(&item).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to create master item"})
				return
			}
			authHeader := c.GetHeader("Authorization")
			entityIDStr := item.ID.String()
			detailsBytes, _ := json.Marshal(item)
			go audit.SendAuditLog(authHeader, "create_master_item", "master_item", &entityIDStr, string(detailsBytes))
			c.JSON(http.StatusCreated, gin.H{"success": true, "data": item})
		})

		api.PUT("/master-items/:id", func(c *gin.Context) {
			role, _ := c.Get("role")
			isAdmin := role == "super_admin" || role == "company_admin" || role == "admin"
			if !isAdmin && !hasPermission(c, "inventory:update") && !hasPermission(c, "master_items:update") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
				return
			}
			idStr := c.Param("id")
			id, err := uuid.Parse(idStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid ID format"})
				return
			}
			var existing database.MasterItem
			if err := database.DB.First(&existing, id).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Master item not found"})
				return
			}
			var input database.MasterItem
			if err := c.ShouldBindJSON(&input); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
				return
			}
			existing.Name = input.Name
			existing.PartNumber = input.PartNumber
			existing.Unit = input.Unit
			existing.Description = input.Description
			existing.CompanyID = input.CompanyID
			if err := database.DB.Save(&existing).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to update master item"})
				return
			}
			authHeader := c.GetHeader("Authorization")
			entityIDStr := existing.ID.String()
			detailsBytes, _ := json.Marshal(existing)
			go audit.SendAuditLog(authHeader, "update_master_item", "master_item", &entityIDStr, string(detailsBytes))
			c.JSON(http.StatusOK, gin.H{"success": true, "data": existing})
		})

		api.DELETE("/master-items/:id", func(c *gin.Context) {
			role, _ := c.Get("role")
			isAdmin := role == "super_admin" || role == "company_admin" || role == "admin"
			if !isAdmin && !hasPermission(c, "inventory:delete") && !hasPermission(c, "master_items:delete") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
				return
			}
			idStr := c.Param("id")
			id, err := uuid.Parse(idStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid ID format"})
				return
			}
			if err := database.DB.Delete(&database.MasterItem{}, id).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to delete master item"})
				return
			}
			authHeader := c.GetHeader("Authorization")
			go audit.SendAuditLog(authHeader, "delete_master_item", "master_item", &idStr, fmt.Sprintf(`{"id": "%s"}`, idStr))
			c.JSON(http.StatusOK, gin.H{"success": true, "message": "Master item deleted successfully"})
		})

		// Master Warehouses CRUD API
		api.GET("/master-warehouses", func(c *gin.Context) {
			role, _ := c.Get("role")
			isAdmin := role == "super_admin" || role == "company_admin" || role == "admin"
			if !isAdmin && !hasPermission(c, "inventory:read") && !hasPermission(c, "master_warehouses:read") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
				return
			}
			companyID := c.Query("company_id")
			query := database.DB
			if companyID != "" {
				parsed, err := uuid.Parse(companyID)
				if err == nil {
					query = query.Where("company_id = ?", parsed)
				}
			}
			var list []database.MasterWarehouse
			if err := query.Order("name asc").Find(&list).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to fetch master warehouses"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
		})

		api.POST("/master-warehouses", func(c *gin.Context) {
			role, _ := c.Get("role")
			isAdmin := role == "super_admin" || role == "company_admin" || role == "admin"
			if !isAdmin && !hasPermission(c, "inventory:create") && !hasPermission(c, "master_warehouses:create") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
				return
			}
			var item database.MasterWarehouse
			if err := c.ShouldBindJSON(&item); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
				return
			}
			item.ID = uuid.New()
			if err := database.DB.Create(&item).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to create master warehouse"})
				return
			}
			authHeader := c.GetHeader("Authorization")
			entityIDStr := item.ID.String()
			detailsBytes, _ := json.Marshal(item)
			go audit.SendAuditLog(authHeader, "create_master_warehouse", "master_warehouse", &entityIDStr, string(detailsBytes))
			c.JSON(http.StatusCreated, gin.H{"success": true, "data": item})
		})

		api.PUT("/master-warehouses/:id", func(c *gin.Context) {
			role, _ := c.Get("role")
			isAdmin := role == "super_admin" || role == "company_admin" || role == "admin"
			if !isAdmin && !hasPermission(c, "inventory:update") && !hasPermission(c, "master_warehouses:update") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
				return
			}
			idStr := c.Param("id")
			id, err := uuid.Parse(idStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid ID format"})
				return
			}
			var existing database.MasterWarehouse
			if err := database.DB.First(&existing, id).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Master warehouse not found"})
				return
			}
			var input database.MasterWarehouse
			if err := c.ShouldBindJSON(&input); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
				return
			}
			existing.Name = input.Name
			existing.Code = input.Code
			existing.Address = input.Address
			existing.VesselID = input.VesselID
			existing.CompanyID = input.CompanyID
			if err := database.DB.Save(&existing).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to update master warehouse"})
				return
			}
			authHeader := c.GetHeader("Authorization")
			entityIDStr := existing.ID.String()
			detailsBytes, _ := json.Marshal(existing)
			go audit.SendAuditLog(authHeader, "update_master_warehouse", "master_warehouse", &entityIDStr, string(detailsBytes))
			c.JSON(http.StatusOK, gin.H{"success": true, "data": existing})
		})

		api.DELETE("/master-warehouses/:id", func(c *gin.Context) {
			role, _ := c.Get("role")
			isAdmin := role == "super_admin" || role == "company_admin" || role == "admin"
			if !isAdmin && !hasPermission(c, "inventory:delete") && !hasPermission(c, "master_warehouses:delete") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
				return
			}
			idStr := c.Param("id")
			id, err := uuid.Parse(idStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid ID format"})
				return
			}
			if err := database.DB.Delete(&database.MasterWarehouse{}, id).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to delete master warehouse"})
				return
			}
			authHeader := c.GetHeader("Authorization")
			go audit.SendAuditLog(authHeader, "delete_master_warehouse", "master_warehouse", &idStr, fmt.Sprintf(`{"id": "%s"}`, idStr))
			c.JSON(http.StatusOK, gin.H{"success": true, "message": "Master warehouse deleted successfully"})
		})

		// Master Units CRUD API
		api.GET("/master-units", func(c *gin.Context) {
			role, _ := c.Get("role")
			isAdmin := role == "super_admin" || role == "company_admin" || role == "admin"
			if !isAdmin && !hasPermission(c, "inventory:read") && !hasPermission(c, "master_units:read") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
				return
			}
			query := database.DB
			var list []database.MasterUnit
			if err := query.Order("name asc").Find(&list).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to fetch master units"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
		})

		api.POST("/master-units", func(c *gin.Context) {
			role, _ := c.Get("role")
			isAdmin := role == "super_admin" || role == "company_admin" || role == "admin"
			if !isAdmin && !hasPermission(c, "inventory:create") && !hasPermission(c, "master_units:create") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
				return
			}
			var item database.MasterUnit
			if err := c.ShouldBindJSON(&item); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
				return
			}
			item.ID = uuid.New()
			if err := database.DB.Create(&item).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to create master unit"})
				return
			}
			authHeader := c.GetHeader("Authorization")
			entityIDStr := item.ID.String()
			detailsBytes, _ := json.Marshal(item)
			go audit.SendAuditLog(authHeader, "create_master_unit", "master_unit", &entityIDStr, string(detailsBytes))
			c.JSON(http.StatusCreated, gin.H{"success": true, "data": item})
		})

		api.PUT("/master-units/:id", func(c *gin.Context) {
			role, _ := c.Get("role")
			isAdmin := role == "super_admin" || role == "company_admin" || role == "admin"
			if !isAdmin && !hasPermission(c, "inventory:update") && !hasPermission(c, "master_units:update") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
				return
			}
			idStr := c.Param("id")
			id, err := uuid.Parse(idStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid ID format"})
				return
			}
			var existing database.MasterUnit
			if err := database.DB.First(&existing, id).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Master unit not found"})
				return
			}
			var input database.MasterUnit
			if err := c.ShouldBindJSON(&input); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
				return
			}
			existing.Name = input.Name
			existing.Code = input.Code
			existing.CompanyID = input.CompanyID
			if err := database.DB.Save(&existing).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to update master unit"})
				return
			}
			authHeader := c.GetHeader("Authorization")
			entityIDStr := existing.ID.String()
			detailsBytes, _ := json.Marshal(existing)
			go audit.SendAuditLog(authHeader, "update_master_unit", "master_unit", &entityIDStr, string(detailsBytes))
			c.JSON(http.StatusOK, gin.H{"success": true, "data": existing})
		})

		api.DELETE("/master-units/:id", func(c *gin.Context) {
			role, _ := c.Get("role")
			isAdmin := role == "super_admin" || role == "company_admin" || role == "admin"
			if !isAdmin && !hasPermission(c, "inventory:delete") && !hasPermission(c, "master_units:delete") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
				return
			}
			idStr := c.Param("id")
			id, err := uuid.Parse(idStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid ID format"})
				return
			}
			if err := database.DB.Delete(&database.MasterUnit{}, id).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to delete master unit"})
				return
			}
			authHeader := c.GetHeader("Authorization")
			go audit.SendAuditLog(authHeader, "delete_master_unit", "master_unit", &idStr, fmt.Sprintf(`{"id": "%s"}`, idStr))
			c.JSON(http.StatusOK, gin.H{"success": true, "message": "Master unit deleted successfully"})
		})

		// Stock Transfers API
		api.GET("/stock-transfers", func(c *gin.Context) {
			companyID := c.Query("company_id")
			query := database.DB
			if companyID != "" {
				parsed, err := uuid.Parse(companyID)
				if err == nil {
					query = query.Where("company_id = ?", parsed)
				}
			}
			var list []database.StockTransfer
			if err := query.Order("created_at desc").Find(&list).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to fetch stock transfers"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
		})

		api.POST("/stock-transfers", func(c *gin.Context) {
			var transfer database.StockTransfer
			if err := c.ShouldBindJSON(&transfer); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
				return
			}
			transfer.ID = uuid.New()
			transfer.Status = "pending"

			emailVal, _ := c.Get("email")
			roleVal, _ := c.Get("role")

			transfer.RequestedBy = fmt.Sprintf("%v", emailVal)
			transfer.RequestedRole = fmt.Sprintf("%v", roleVal)

			if err := database.DB.Create(&transfer).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to create transfer request"})
				return
			}
			c.JSON(http.StatusCreated, gin.H{"success": true, "data": transfer})
		})

		api.POST("/stock-transfers/:id/approve", func(c *gin.Context) {
			idStr := c.Param("id")
			id, err := uuid.Parse(idStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid ID format"})
				return
			}

			tx := database.DB.Begin()
			var transfer database.StockTransfer
			if err := tx.First(&transfer, id).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Transfer request not found"})
				return
			}

			if transfer.Status != "pending" {
				tx.Rollback()
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Transfer request is not pending"})
				return
			}

			// Validate approval role flow: Admin -> Operator or Operator -> Admin
			roleVal, _ := c.Get("role")
			emailVal, _ := c.Get("email")
			approverRole := fmt.Sprintf("%v", roleVal)
			approverEmail := fmt.Sprintf("%v", emailVal)

			isAdminApprover := approverRole == "super_admin" || approverRole == "company_admin" || approverRole == "admin"
			isRequesterAdmin := transfer.RequestedRole == "super_admin" || transfer.RequestedRole == "company_admin" || transfer.RequestedRole == "admin"

			if isRequesterAdmin && isAdminApprover {
				tx.Rollback()
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Admin-requested transfer must be approved by an Operator"})
				return
			}
			if !isRequesterAdmin && !isAdminApprover {
				tx.Rollback()
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Operator-requested transfer must be approved by an Admin"})
				return
			}

			// 1. Find and decrement source stock
			var srcItem database.InventoryItem
			srcQuery := tx.Where("location = ? AND name = ? AND part_number = ?", transfer.SourceWarehouse, transfer.ItemName, transfer.PartNumber)
			if transfer.CompanyID != nil {
				srcQuery = srcQuery.Where("company_id = ?", transfer.CompanyID)
			} else {
				srcQuery = srcQuery.Where("company_id IS NULL")
			}

			if err := srcQuery.First(&srcItem).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Source item not found in warehouse " + transfer.SourceWarehouse})
				return
			}

			if srcItem.Quantity < transfer.Quantity {
				tx.Rollback()
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Insufficient stock in source warehouse"})
				return
			}

			srcItem.Quantity -= transfer.Quantity
			if err := tx.Save(&srcItem).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to update source stock"})
				return
			}

			// 2. Increment or create target stock
			var destItem database.InventoryItem
			destQuery := tx.Where("location = ? AND name = ? AND part_number = ?", transfer.TargetWarehouse, transfer.ItemName, transfer.PartNumber)
			if transfer.CompanyID != nil {
				destQuery = destQuery.Where("company_id = ?", transfer.CompanyID)
			} else {
				destQuery = destQuery.Where("company_id IS NULL")
			}

			err = destQuery.First(&destItem).Error
			if err == nil {
				// Target item exists, increment quantity
				destItem.Quantity += transfer.Quantity
				if err := tx.Save(&destItem).Error; err != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to update target stock"})
					return
				}
			} else {
				// Target item does not exist, create it
				newItem := database.InventoryItem{
					ID:           uuid.New(),
					Name:         transfer.ItemName,
					PartNumber:   transfer.PartNumber,
					Quantity:     transfer.Quantity,
					Unit:         transfer.Unit,
					Location:     transfer.TargetWarehouse,
					MinimumStock: 5,
					CompanyID:    transfer.CompanyID,
				}
				if err := tx.Create(&newItem).Error; err != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to create target stock item"})
					return
				}
			}

			// 3. Mark transfer approved
			transfer.Status = "approved"
			transfer.ApprovedRejectedBy = &approverEmail
			if err := tx.Save(&transfer).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to update transfer status"})
				return
			}

			tx.Commit()

			authHeader := c.GetHeader("Authorization")
			detailsBytes, _ := json.Marshal(transfer)
			go audit.SendAuditLog(authHeader, "approve_stock_transfer", "stock_transfer", &idStr, string(detailsBytes))

			c.JSON(http.StatusOK, gin.H{"success": true, "data": transfer})
		})

		api.POST("/stock-transfers/:id/reject", func(c *gin.Context) {
			idStr := c.Param("id")
			id, err := uuid.Parse(idStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid ID format"})
				return
			}

			var input struct {
				Comments string `json:"comments" binding:"required"`
			}
			if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.Comments) == "" {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Comment/reason is required for rejection"})
				return
			}

			tx := database.DB.Begin()
			var transfer database.StockTransfer
			if err := tx.First(&transfer, id).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Transfer request not found"})
				return
			}

			if transfer.Status != "pending" {
				tx.Rollback()
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Transfer request is not pending"})
				return
			}

			// Validate approval role flow: Admin -> Operator or Operator -> Admin
			roleVal, _ := c.Get("role")
			emailVal, _ := c.Get("email")
			approverRole := fmt.Sprintf("%v", roleVal)
			approverEmail := fmt.Sprintf("%v", emailVal)

			isAdminApprover := approverRole == "super_admin" || approverRole == "company_admin" || approverRole == "admin"
			isRequesterAdmin := transfer.RequestedRole == "super_admin" || transfer.RequestedRole == "company_admin" || transfer.RequestedRole == "admin"

			if isRequesterAdmin && isAdminApprover {
				tx.Rollback()
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Admin-requested transfer must be rejected by an Operator"})
				return
			}
			if !isRequesterAdmin && !isAdminApprover {
				tx.Rollback()
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Operator-requested transfer must be rejected by an Admin"})
				return
			}

			transfer.Status = "rejected"
			transfer.ApprovedRejectedBy = &approverEmail
			transfer.Comments = input.Comments

			if err := tx.Save(&transfer).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to update transfer status"})
				return
			}

			tx.Commit()

			authHeader := c.GetHeader("Authorization")
			detailsBytes, _ := json.Marshal(transfer)
			go audit.SendAuditLog(authHeader, "reject_stock_transfer", "stock_transfer", &idStr, string(detailsBytes))

			c.JSON(http.StatusOK, gin.H{"success": true, "data": transfer})
		})
	}

	log.Printf("Inventory Backend running on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to run Inventory server: %v", err)
	}
}
