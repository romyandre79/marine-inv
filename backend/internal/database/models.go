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

