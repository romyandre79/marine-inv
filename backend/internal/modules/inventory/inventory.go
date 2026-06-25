package inventory

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/marines-dev/inventory-portal/internal/audit"
	"github.com/marines-dev/inventory-portal/internal/database"
	"github.com/marines-dev/inventory-portal/internal/excel"
	"github.com/marines-dev/inventory-portal/internal/middleware"
)

func RegisterRoutes(api *gin.RouterGroup) {
	invGroup := api.Group("/inventory")
	{
		invGroup.GET("", middleware.RequirePermission("inventory:read"), ListInventory)
		invGroup.GET("/export", middleware.RequirePermission("inventory:read"), ExportInventory)
		invGroup.POST("/import", middleware.RequirePermission("inventory:create"), ImportInventory) // wait, the original has permission check for both inventory:create or inventory:update. Let's handle it inside.
		invGroup.POST("", middleware.RequirePermission("inventory:create"), CreateInventory)
		invGroup.PUT("/:id", middleware.RequirePermission("inventory:update"), UpdateInventory)
		invGroup.DELETE("/:id", middleware.RequirePermission("inventory:delete"), DeleteInventory)
	}
}

func ListInventory(c *gin.Context) {
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
}

func ExportInventory(c *gin.Context) {
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
}

func ImportInventory(c *gin.Context) {
	// The original main.go checks: hasPermission(c, "inventory:create") || hasPermission(c, "inventory:update")
	// Since RegisterRoutes applied RequirePermission("inventory:create") on /import, we're safe, but let's be fully aligned
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
}

func CreateInventory(c *gin.Context) {
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
}

func UpdateInventory(c *gin.Context) {
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
}

func DeleteInventory(c *gin.Context) {
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
}
