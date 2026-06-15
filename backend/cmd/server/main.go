package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
	}

	log.Printf("Inventory Backend running on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to run Inventory server: %v", err)
	}
}
