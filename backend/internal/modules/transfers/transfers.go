package transfers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/marines-dev/inventory-portal/internal/audit"
	"github.com/marines-dev/inventory-portal/internal/database"
)

func RegisterRoutes(api *gin.RouterGroup) {
	stGroup := api.Group("/stock-transfers")
	{
		stGroup.GET("", ListTransfers)
		stGroup.POST("", CreateTransfer)
		stGroup.POST("/:id/approve", ApproveTransfer)
		stGroup.POST("/:id/reject", RejectTransfer)
	}
}

func ListTransfers(c *gin.Context) {
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
}

func CreateTransfer(c *gin.Context) {
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
}

func ApproveTransfer(c *gin.Context) {
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
}

func RejectTransfer(c *gin.Context) {
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
}
