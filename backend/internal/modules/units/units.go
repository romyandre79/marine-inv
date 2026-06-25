package units

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
	muGroup := api.Group("/master-units")
	{
		muGroup.GET("", ListUnits)
		muGroup.GET("/export", ExportUnits)
		muGroup.POST("/import", ImportUnits)
		muGroup.POST("", CreateUnit)
		muGroup.PUT("/:id", UpdateUnit)
		muGroup.DELETE("/:id", DeleteUnit)
	}
}

func hasUnitReadAccess(c *gin.Context) bool {
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
		if p == "inventory:read" || p == "master_units:read" {
			return true
		}
	}
	return false
}

func hasUnitWriteAccess(c *gin.Context) bool {
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
		if p == "inventory:create" || p == "master_units:create" || p == "inventory:update" || p == "master_units:update" {
			return true
		}
	}
	return false
}

func hasUnitDeleteAccess(c *gin.Context) bool {
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
		if p == "inventory:delete" || p == "master_units:delete" {
			return true
		}
	}
	return false
}

func ListUnits(c *gin.Context) {
	if !hasUnitReadAccess(c) {
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
}

func ExportUnits(c *gin.Context) {
	if !hasUnitReadAccess(c) {
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
}

func ImportUnits(c *gin.Context) {
	if !hasUnitWriteAccess(c) {
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
}

func CreateUnit(c *gin.Context) {
	if !hasUnitWriteAccess(c) {
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
}

func UpdateUnit(c *gin.Context) {
	if !hasUnitWriteAccess(c) {
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
}

func DeleteUnit(c *gin.Context) {
	if !hasUnitDeleteAccess(c) {
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
}
