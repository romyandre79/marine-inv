package masteritems

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/marines-dev/inventory-portal/internal/audit"
	"github.com/marines-dev/inventory-portal/internal/database"
	"github.com/marines-dev/inventory-portal/internal/excel"
)

func RegisterRoutes(api *gin.RouterGroup) {
	miGroup := api.Group("/master-items")
	{
		// Note: The original checks role == super_admin/company_admin/admin OR permission inventory:read/master_items:read.
		// Since we want to simplify, we can check role in the handler or route. Let's just check inside.
		miGroup.GET("", ListMasterItems)
		miGroup.GET("/export", ExportMasterItems)
		miGroup.POST("/import", ImportMasterItems)
		miGroup.POST("", CreateMasterItem)
		miGroup.PUT("/:id", UpdateMasterItem)
		miGroup.DELETE("/:id", DeleteMasterItem)
	}
}

func hasMasterItemReadAccess(c *gin.Context) bool {
	role, _ := c.Get("role")
	if role == "super_admin" || role == "company_admin" || role == "admin" {
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
		if p == "inventory:read" || p == "master_items:read" {
			return true
		}
	}
	return false
}

func hasMasterItemWriteAccess(c *gin.Context) bool {
	role, _ := c.Get("role")
	if role == "super_admin" || role == "company_admin" || role == "admin" {
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
		if p == "inventory:create" || p == "master_items:create" || p == "inventory:update" || p == "master_items:update" {
			return true
		}
	}
	return false
}

func hasMasterItemDeleteAccess(c *gin.Context) bool {
	role, _ := c.Get("role")
	if role == "super_admin" || role == "company_admin" || role == "admin" {
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
		if p == "inventory:delete" || p == "master_items:delete" {
			return true
		}
	}
	return false
}

func ListMasterItems(c *gin.Context) {
	if !hasMasterItemReadAccess(c) {
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
}

func ExportMasterItems(c *gin.Context) {
	if !hasMasterItemReadAccess(c) {
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
}

func ImportMasterItems(c *gin.Context) {
	if !hasMasterItemWriteAccess(c) {
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
}

func CreateMasterItem(c *gin.Context) {
	if !hasMasterItemWriteAccess(c) {
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
}

func UpdateMasterItem(c *gin.Context) {
	if !hasMasterItemWriteAccess(c) {
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
}

func DeleteMasterItem(c *gin.Context) {
	if !hasMasterItemDeleteAccess(c) {
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
}
