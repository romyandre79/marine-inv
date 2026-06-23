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
	"github.com/marines-dev/inventory-portal/internal/excel"
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
			search := c.Query("search")
			query := database.DB
			if companyID != "" {
				parsed, err := uuid.Parse(companyID)
				if err == nil {
					query = query.Where("company_id = ?", parsed)
				}
			}
			if search != "" {
				s := "%" + search + "%"
				query = query.Where("name LIKE ? OR part_number LIKE ? OR location LIKE ?", s, s, s)
			}
			var list []database.InventoryItem
			var total int64
			query.Model(&database.InventoryItem{}).Count(&total)

			if err := query.Order("name asc").Scopes(database.Paginate(c)).Find(&list).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to fetch inventory items"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "data": list, "meta": database.GetPaginationMeta(c, total)})
		})

		api.GET("/inventory/export", func(c *gin.Context) {
			if !hasPermission(c, "inventory:read") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
				return
			}
			companyID := c.Query("company_id")
			search := c.Query("search")
			query := database.DB
			if companyID != "" {
				parsed, err := uuid.Parse(companyID)
				if err == nil {
					query = query.Where("company_id = ?", parsed)
				}
			}
			if search != "" {
				s := "%" + search + "%"
				query = query.Where("name LIKE ? OR part_number LIKE ? OR location LIKE ?", s, s, s)
			}
			var list []database.InventoryItem
			if err := query.Order("name asc").Find(&list).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to fetch inventory items"})
				return
			}

			headers := []string{"ID", "Name", "PartNumber", "Quantity", "Unit", "Location", "MinimumStock", "CompanyID"}
			var data []map[string]interface{}
			for _, item := range list {
				var compID string
				if item.CompanyID != nil {
					compID = item.CompanyID.String()
				}
				data = append(data, map[string]interface{}{
					"ID":           item.ID.String(),
					"Name":         item.Name,
					"PartNumber":   item.PartNumber,
					"Quantity":     item.Quantity,
					"Unit":         item.Unit,
					"Location":     item.Location,
					"MinimumStock": item.MinimumStock,
					"CompanyID":    compID,
				})
			}

			c.Header("Content-Disposition", "attachment; filename=inventory.xlsx")
			c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")

			if err := excel.ExportToExcel("Inventory", headers, data, c.Writer); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to generate Excel: " + err.Error()})
				return
			}
		})

		api.POST("/inventory/import", func(c *gin.Context) {
			if !hasPermission(c, "inventory:create") && !hasPermission(c, "inventory:update") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
				return
			}
			file, _, err := c.Request.FormFile("file")
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "File is required"})
				return
			}
			defer file.Close()

			records, err := excel.ParseExcel(file, "Inventory")
			if err != nil {
				records, err = excel.ParseExcel(file, "")
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Failed to parse excel file: " + err.Error()})
					return
				}
			}

			tx := database.DB.Begin()
			var importedCount int
			for _, record := range records {
				name := record["Name"]
				if name == "" {
					continue
				}

				var qty int
				fmt.Sscanf(record["Quantity"], "%d", &qty)
				var minStock int
				fmt.Sscanf(record["MinimumStock"], "%d", &minStock)

				var compID *uuid.UUID
				if record["CompanyID"] != "" {
					parsed, err := uuid.Parse(record["CompanyID"])
					if err == nil {
						compID = &parsed
					}
				}

				item := database.InventoryItem{
					Name:         name,
					PartNumber:   record["PartNumber"],
					Quantity:     qty,
					Unit:         record["Unit"],
					Location:     record["Location"],
					MinimumStock: minStock,
					CompanyID:    compID,
				}

				var existing database.InventoryItem
				idStr := record["ID"]
				hasExisting := false

				if idStr != "" {
					parsedID, err := uuid.Parse(idStr)
					if err == nil {
						if err := tx.Where("id = ?", parsedID).First(&existing).Error; err == nil {
							hasExisting = true
						}
					}
				}

				if !hasExisting {
					q := tx.Where("name = ? AND part_number = ? AND location = ?", item.Name, item.PartNumber, item.Location)
					if item.CompanyID != nil {
						q = q.Where("company_id = ?", item.CompanyID)
					} else {
						q = q.Where("company_id IS NULL")
					}
					if err := q.First(&existing).Error; err == nil {
						hasExisting = true
					}
				}

				if hasExisting {
					existing.Quantity = item.Quantity
					existing.Unit = item.Unit
					existing.MinimumStock = item.MinimumStock
					if err := tx.Save(&existing).Error; err != nil {
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to update item: " + err.Error()})
						return
					}
				} else {
					if idStr != "" {
						parsedID, err := uuid.Parse(idStr)
						if err == nil {
							item.ID = parsedID
						} else {
							item.ID = uuid.New()
						}
					} else {
						item.ID = uuid.New()
					}
					if err := tx.Create(&item).Error; err != nil {
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to create item: " + err.Error()})
						return
					}
				}
				importedCount++
			}

			tx.Commit()
			c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("Successfully imported %d inventory items", importedCount)})
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
			search := c.Query("search")
			query := database.DB
			if companyID != "" {
				parsed, err := uuid.Parse(companyID)
				if err == nil {
					query = query.Where("company_id = ?", parsed)
				}
			}
			if search != "" {
				s := "%" + search + "%"
				query = query.Where("name LIKE ? OR part_number LIKE ? OR description LIKE ?", s, s, s)
			}
			var list []database.MasterItem
			var total int64
			query.Model(&database.MasterItem{}).Count(&total)

			if err := query.Order("name asc").Scopes(database.Paginate(c)).Find(&list).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to fetch master items"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "data": list, "meta": database.GetPaginationMeta(c, total)})
		})

		api.GET("/master-items/export", func(c *gin.Context) {
			role, _ := c.Get("role")
			isAdmin := role == "super_admin" || role == "company_admin" || role == "admin"
			if !isAdmin && !hasPermission(c, "inventory:read") && !hasPermission(c, "master_items:read") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
				return
			}
			companyID := c.Query("company_id")
			search := c.Query("search")
			query := database.DB
			if companyID != "" {
				parsed, err := uuid.Parse(companyID)
				if err == nil {
					query = query.Where("company_id = ?", parsed)
				}
			}
			if search != "" {
				s := "%" + search + "%"
				query = query.Where("name LIKE ? OR part_number LIKE ? OR description LIKE ?", s, s, s)
			}
			var list []database.MasterItem
			if err := query.Order("name asc").Find(&list).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to fetch master items"})
				return
			}

			headers := []string{"ID", "Name", "PartNumber", "Unit", "Description", "CompanyID"}
			var data []map[string]interface{}
			for _, item := range list {
				var compID string
				if item.CompanyID != nil {
					compID = item.CompanyID.String()
				}
				data = append(data, map[string]interface{}{
					"ID":          item.ID.String(),
					"Name":        item.Name,
					"PartNumber":  item.PartNumber,
					"Unit":        item.Unit,
					"Description": item.Description,
					"CompanyID":   compID,
				})
			}

			c.Header("Content-Disposition", "attachment; filename=master-items.xlsx")
			c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")

			if err := excel.ExportToExcel("MasterItems", headers, data, c.Writer); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to generate Excel: " + err.Error()})
				return
			}
		})

		api.POST("/master-items/import", func(c *gin.Context) {
			role, _ := c.Get("role")
			isAdmin := role == "super_admin" || role == "company_admin" || role == "admin"
			if !isAdmin && !hasPermission(c, "inventory:create") && !hasPermission(c, "master_items:create") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
				return
			}
			file, _, err := c.Request.FormFile("file")
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "File is required"})
				return
			}
			defer file.Close()

			records, err := excel.ParseExcel(file, "MasterItems")
			if err != nil {
				records, err = excel.ParseExcel(file, "")
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Failed to parse excel file: " + err.Error()})
					return
				}
			}

			tx := database.DB.Begin()
			var importedCount int
			for _, record := range records {
				name := record["Name"]
				if name == "" {
					continue
				}

				var compID *uuid.UUID
				if record["CompanyID"] != "" {
					parsed, err := uuid.Parse(record["CompanyID"])
					if err == nil {
						compID = &parsed
					}
				}

				item := database.MasterItem{
					Name:        name,
					PartNumber:  record["PartNumber"],
					Unit:        record["Unit"],
					Description: record["Description"],
					CompanyID:   compID,
				}

				var existing database.MasterItem
				idStr := record["ID"]
				hasExisting := false

				if idStr != "" {
					parsedID, err := uuid.Parse(idStr)
					if err == nil {
						if err := tx.Where("id = ?", parsedID).First(&existing).Error; err == nil {
							hasExisting = true
						}
					}
				}

				if !hasExisting {
					q := tx.Where("name = ? AND part_number = ?", item.Name, item.PartNumber)
					if item.CompanyID != nil {
						q = q.Where("company_id = ?", item.CompanyID)
					} else {
						q = q.Where("company_id IS NULL")
					}
					if err := q.First(&existing).Error; err == nil {
						hasExisting = true
					}
				}

				if hasExisting {
					existing.Unit = item.Unit
					existing.Description = item.Description
					if err := tx.Save(&existing).Error; err != nil {
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to update item: " + err.Error()})
						return
					}
				} else {
					if idStr != "" {
						parsedID, err := uuid.Parse(idStr)
						if err == nil {
							item.ID = parsedID
						} else {
							item.ID = uuid.New()
						}
					} else {
						item.ID = uuid.New()
					}
					if err := tx.Create(&item).Error; err != nil {
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to create item: " + err.Error()})
						return
					}
				}
				importedCount++
			}

			tx.Commit()
			c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("Successfully imported %d master items", importedCount)})
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
			search := c.Query("search")
			query := database.DB
			if companyID != "" {
				parsed, err := uuid.Parse(companyID)
				if err == nil {
					query = query.Where("company_id = ?", parsed)
				}
			}
			if search != "" {
				s := "%" + search + "%"
				query = query.Where("name LIKE ? OR code LIKE ? OR address LIKE ?", s, s, s)
			}
			var list []database.MasterWarehouse
			var total int64
			query.Model(&database.MasterWarehouse{}).Count(&total)

			if err := query.Order("name asc").Scopes(database.Paginate(c)).Find(&list).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to fetch master warehouses"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "data": list, "meta": database.GetPaginationMeta(c, total)})
		})

		api.GET("/master-warehouses/export", func(c *gin.Context) {
			role, _ := c.Get("role")
			isAdmin := role == "super_admin" || role == "company_admin" || role == "admin"
			if !isAdmin && !hasPermission(c, "inventory:read") && !hasPermission(c, "master_warehouses:read") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
				return
			}
			companyID := c.Query("company_id")
			search := c.Query("search")
			query := database.DB
			if companyID != "" {
				parsed, err := uuid.Parse(companyID)
				if err == nil {
					query = query.Where("company_id = ?", parsed)
				}
			}
			if search != "" {
				s := "%" + search + "%"
				query = query.Where("name LIKE ? OR code LIKE ? OR address LIKE ?", s, s, s)
			}
			var list []database.MasterWarehouse
			if err := query.Order("name asc").Find(&list).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to fetch master warehouses"})
				return
			}

			headers := []string{"ID", "Name", "Code", "Address", "VesselID", "CompanyID"}
			var data []map[string]interface{}
			for _, item := range list {
				var compID string
				if item.CompanyID != nil {
					compID = item.CompanyID.String()
				}
				var vID string
				if item.VesselID != nil {
					vID = item.VesselID.String()
				}
				data = append(data, map[string]interface{}{
					"ID":        item.ID.String(),
					"Name":      item.Name,
					"Code":      item.Code,
					"Address":   item.Address,
					"VesselID":  vID,
					"CompanyID": compID,
				})
			}

			c.Header("Content-Disposition", "attachment; filename=master-warehouses.xlsx")
			c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")

			if err := excel.ExportToExcel("MasterWarehouses", headers, data, c.Writer); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to generate Excel: " + err.Error()})
				return
			}
		})

		api.POST("/master-warehouses/import", func(c *gin.Context) {
			role, _ := c.Get("role")
			isAdmin := role == "super_admin" || role == "company_admin" || role == "admin"
			if !isAdmin && !hasPermission(c, "inventory:create") && !hasPermission(c, "master_warehouses:create") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
				return
			}
			file, _, err := c.Request.FormFile("file")
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "File is required"})
				return
			}
			defer file.Close()

			records, err := excel.ParseExcel(file, "MasterWarehouses")
			if err != nil {
				records, err = excel.ParseExcel(file, "")
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Failed to parse excel file: " + err.Error()})
					return
				}
			}

			tx := database.DB.Begin()
			var importedCount int
			for _, record := range records {
				name := record["Name"]
				if name == "" {
					continue
				}

				var compID *uuid.UUID
				if record["CompanyID"] != "" {
					parsed, err := uuid.Parse(record["CompanyID"])
					if err == nil {
						compID = &parsed
					}
				}

				var vID *uuid.UUID
				if record["VesselID"] != "" {
					parsed, err := uuid.Parse(record["VesselID"])
					if err == nil {
						vID = &parsed
					}
				}

				item := database.MasterWarehouse{
					Name:      name,
					Code:      record["Code"],
					Address:   record["Address"],
					VesselID:  vID,
					CompanyID: compID,
				}

				var existing database.MasterWarehouse
				idStr := record["ID"]
				hasExisting := false

				if idStr != "" {
					parsedID, err := uuid.Parse(idStr)
					if err == nil {
						if err := tx.Where("id = ?", parsedID).First(&existing).Error; err == nil {
							hasExisting = true
						}
					}
				}

				if !hasExisting {
					q := tx.Where("name = ? OR code = ?", item.Name, item.Code)
					if item.CompanyID != nil {
						q = q.Where("company_id = ?", item.CompanyID)
					} else {
						q = q.Where("company_id IS NULL")
					}
					if err := q.First(&existing).Error; err == nil {
						hasExisting = true
					}
				}

				if hasExisting {
					existing.Address = item.Address
					existing.VesselID = item.VesselID
					if err := tx.Save(&existing).Error; err != nil {
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to update warehouse: " + err.Error()})
						return
					}
				} else {
					if idStr != "" {
						parsedID, err := uuid.Parse(idStr)
						if err == nil {
							item.ID = parsedID
						} else {
							item.ID = uuid.New()
						}
					} else {
						item.ID = uuid.New()
					}
					if err := tx.Create(&item).Error; err != nil {
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to create warehouse: " + err.Error()})
						return
					}
				}
				importedCount++
			}

			tx.Commit()
			c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("Successfully imported %d warehouses", importedCount)})
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
			search := c.Query("search")
			query := database.DB.Model(&database.MasterUnit{})
			if search != "" {
				s := "%" + search + "%"
				query = query.Where("name LIKE ? OR code LIKE ?", s, s)
			}
			var list []database.MasterUnit
			var total int64
			query.Count(&total)

			if err := query.Order("name asc").Scopes(database.Paginate(c)).Find(&list).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to fetch master units"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "data": list, "meta": database.GetPaginationMeta(c, total)})
		})

		api.GET("/master-units/export", func(c *gin.Context) {
			role, _ := c.Get("role")
			isAdmin := role == "super_admin" || role == "company_admin" || role == "admin"
			if !isAdmin && !hasPermission(c, "inventory:read") && !hasPermission(c, "master_units:read") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
				return
			}
			search := c.Query("search")
			query := database.DB
			if search != "" {
				s := "%" + search + "%"
				query = query.Where("name LIKE ? OR code LIKE ?", s, s)
			}
			var list []database.MasterUnit
			if err := query.Order("name asc").Find(&list).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to fetch master units"})
				return
			}

			headers := []string{"ID", "Name", "Code", "CompanyID"}
			var data []map[string]interface{}
			for _, item := range list {
				var compID string
				if item.CompanyID != nil {
					compID = item.CompanyID.String()
				}
				data = append(data, map[string]interface{}{
					"ID":        item.ID.String(),
					"Name":      item.Name,
					"Code":      item.Code,
					"CompanyID": compID,
				})
			}

			c.Header("Content-Disposition", "attachment; filename=master-units.xlsx")
			c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")

			if err := excel.ExportToExcel("MasterUnits", headers, data, c.Writer); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to generate Excel: " + err.Error()})
				return
			}
		})

		api.POST("/master-units/import", func(c *gin.Context) {
			role, _ := c.Get("role")
			isAdmin := role == "super_admin" || role == "company_admin" || role == "admin"
			if !isAdmin && !hasPermission(c, "inventory:create") && !hasPermission(c, "master_units:create") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
				return
			}
			file, _, err := c.Request.FormFile("file")
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "File is required"})
				return
			}
			defer file.Close()

			records, err := excel.ParseExcel(file, "MasterUnits")
			if err != nil {
				records, err = excel.ParseExcel(file, "")
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Failed to parse excel file: " + err.Error()})
					return
				}
			}

			tx := database.DB.Begin()
			var importedCount int
			for _, record := range records {
				name := record["Name"]
				if name == "" {
					continue
				}

				var compID *uuid.UUID
				if record["CompanyID"] != "" {
					parsed, err := uuid.Parse(record["CompanyID"])
					if err == nil {
						compID = &parsed
					}
				}

				item := database.MasterUnit{
					Name:      name,
					Code:      record["Code"],
					CompanyID: compID,
				}

				var existing database.MasterUnit
				idStr := record["ID"]
				hasExisting := false

				if idStr != "" {
					parsedID, err := uuid.Parse(idStr)
					if err == nil {
						if err := tx.Where("id = ?", parsedID).First(&existing).Error; err == nil {
							hasExisting = true
						}
					}
				}

				if !hasExisting {
					q := tx.Where("name = ? OR code = ?", item.Name, item.Code)
					if item.CompanyID != nil {
						q = q.Where("company_id = ?", item.CompanyID)
					} else {
						q = q.Where("company_id IS NULL")
					}
					if err := q.First(&existing).Error; err == nil {
						hasExisting = true
					}
				}

				if hasExisting {
					if err := tx.Save(&existing).Error; err != nil {
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to update unit: " + err.Error()})
						return
					}
				} else {
					if idStr != "" {
						parsedID, err := uuid.Parse(idStr)
						if err == nil {
							item.ID = parsedID
						} else {
							item.ID = uuid.New()
						}
					} else {
						item.ID = uuid.New()
					}
					if err := tx.Create(&item).Error; err != nil {
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to create unit: " + err.Error()})
						return
					}
				}
				importedCount++
			}

			tx.Commit()
			c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("Successfully imported %d units", importedCount)})
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
			search := c.Query("search")
			query := database.DB
			if companyID != "" {
				parsed, err := uuid.Parse(companyID)
				if err == nil {
					query = query.Where("company_id = ?", parsed)
				}
			}
			if search != "" {
				s := "%" + search + "%"
				query = query.Where("item_name LIKE ? OR source_warehouse LIKE ? OR target_warehouse LIKE ? OR requested_by LIKE ?", s, s, s, s)
			}
			var list []database.StockTransfer
			var total int64
			query.Model(&database.StockTransfer{}).Count(&total)

			if err := query.Order("created_at desc").Scopes(database.Paginate(c)).Find(&list).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to fetch stock transfers"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "data": list, "meta": database.GetPaginationMeta(c, total)})
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
