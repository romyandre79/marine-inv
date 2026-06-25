package warehouses

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
	mwGroup := api.Group("/master-warehouses")
	{
		mwGroup.GET("", ListWarehouses)
		mwGroup.GET("/export", ExportWarehouses)
		mwGroup.POST("/import", ImportWarehouses)
		mwGroup.POST("", CreateWarehouse)
		mwGroup.PUT("/:id", UpdateWarehouse)
		mwGroup.DELETE("/:id", DeleteWarehouse)
	}
}

func hasWarehouseReadAccess(c *gin.Context) bool {
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
		if p == "inventory:read" || p == "master_warehouses:read" {
			return true
		}
	}
	return false
}

func hasWarehouseWriteAccess(c *gin.Context) bool {
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
		if p == "inventory:create" || p == "master_warehouses:create" || p == "inventory:update" || p == "master_warehouses:update" {
			return true
		}
	}
	return false
}

func hasWarehouseDeleteAccess(c *gin.Context) bool {
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
		if p == "inventory:delete" || p == "master_warehouses:delete" {
			return true
		}
	}
	return false
}

func ListWarehouses(c *gin.Context) {
	if !hasWarehouseReadAccess(c) {
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
}

func ExportWarehouses(c *gin.Context) {
	if !hasWarehouseReadAccess(c) {
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
}

func ImportWarehouses(c *gin.Context) {
	if !hasWarehouseWriteAccess(c) {
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
}

func CreateWarehouse(c *gin.Context) {
	if !hasWarehouseWriteAccess(c) {
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
}

func UpdateWarehouse(c *gin.Context) {
	if !hasWarehouseWriteAccess(c) {
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
}

func DeleteWarehouse(c *gin.Context) {
	if !hasWarehouseDeleteAccess(c) {
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
}
