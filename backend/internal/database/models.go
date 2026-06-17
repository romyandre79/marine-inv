package database

import (
	"time"

	"github.com/google/uuid"
)

type InventoryItem struct {
	ID           uuid.UUID  `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	Name         string     `gorm:"not null;size:255" json:"name"`
	PartNumber   string     `gorm:"size:100" json:"part_number"`
	Quantity     int        `gorm:"not null;default:0" json:"quantity"`
	Unit         string     `gorm:"size:50;default:'pcs'" json:"unit"`
	Location     string     `gorm:"size:255" json:"location"`
	MinimumStock int        `gorm:"not null;default:0" json:"minimum_stock"`
	CompanyID    *uuid.UUID `gorm:"type:uuid" json:"company_id"`
	CreatedAt    time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

func (InventoryItem) TableName() string {
	return "inventory"
}

type MasterItem struct {
	ID          uuid.UUID  `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	Name        string     `gorm:"not null;size:255" json:"name"`
	PartNumber  string     `gorm:"size:100" json:"part_number"`
	Unit        string     `gorm:"size:50;default:'pcs'" json:"unit"`
	Description string     `gorm:"type:text" json:"description"`
	CompanyID   *uuid.UUID `gorm:"type:uuid" json:"company_id"`
	CreatedAt   time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

func (MasterItem) TableName() string {
	return "master_items"
}

type MasterWarehouse struct {
	ID        uuid.UUID  `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	Name      string     `gorm:"not null;size:255" json:"name"`
	Code      string     `gorm:"size:100" json:"code"`
	Address   string     `gorm:"type:text" json:"address"`
	VesselID  *uuid.UUID `gorm:"type:uuid" json:"vessel_id"`
	CompanyID *uuid.UUID `gorm:"type:uuid" json:"company_id"`
	CreatedAt time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

func (MasterWarehouse) TableName() string {
	return "master_warehouses"
}

type MasterUnit struct {
	ID        uuid.UUID  `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	Name      string     `gorm:"not null;size:255" json:"name"`
	Code      string     `gorm:"not null;size:100" json:"code"`
	CompanyID *uuid.UUID `gorm:"type:uuid" json:"company_id"`
	CreatedAt time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

func (MasterUnit) TableName() string {
	return "master_units"
}

type StockTransfer struct {
	ID                 uuid.UUID  `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	SourceWarehouse    string     `gorm:"size:255;not null" json:"source_warehouse"`
	TargetWarehouse    string     `gorm:"size:255;not null" json:"target_warehouse"`
	ItemName           string     `gorm:"size:255;not null" json:"item_name"`
	PartNumber         string     `gorm:"size:100" json:"part_number"`
	Quantity           int        `gorm:"not null" json:"quantity"`
	Unit               string     `gorm:"size:50;default:'pcs'" json:"unit"`
	Status             string     `gorm:"size:50;not null;default:'pending'" json:"status"` // 'pending', 'approved', 'rejected'
	RequestedBy        string     `gorm:"size:255;not null" json:"requested_by"`
	RequestedRole      string     `gorm:"size:50;not null" json:"requested_role"`
	ApprovedRejectedBy *string    `gorm:"size:255" json:"approved_rejected_by,omitempty"`
	Comments           string     `gorm:"type:text" json:"comments"`
	CompanyID          *uuid.UUID `gorm:"type:uuid" json:"company_id"`
	CreatedAt          time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt          time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

func (StockTransfer) TableName() string {
	return "stock_transfers"
}
