package main

import (
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/marines-dev/inventory-portal/internal/database"
)

func main() {
	log.Println("Initializing database connection for seeding...")
	db := database.InitDB()

	// Clear existing inventory
	log.Println("Clearing existing inventory...")
	db.Exec("TRUNCATE TABLE inventory RESTART IDENTITY CASCADE")

	acmeID := uuid.MustParse("10e31e63-386d-465e-9b10-3d72abc56b95")
	betaID := uuid.MustParse("2f13615c-1dbe-4189-9826-8e0b358532b7")

	items := []database.InventoryItem{
		{
			ID:           uuid.MustParse("d1111111-1111-1111-1111-111111111111"),
			Name:         "Engine Oil 15W-40",
			PartNumber:   "PN-OIL-15W40",
			Quantity:     150,
			Unit:         "Liters",
			Location:     "Warehouse A - Shelf 2",
			MinimumStock: 50,
			CompanyID:    &acmeID,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           uuid.MustParse("d2222222-2222-2222-2222-222222222222"),
			Name:         "Fuel Filter Element",
			PartNumber:   "PN-FLT-ELEM",
			Quantity:     12,
			Unit:         "pcs",
			Location:     "Warehouse A - Cabinet B",
			MinimumStock: 15,
			CompanyID:    &acmeID,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           uuid.MustParse("d3333333-3333-3333-3333-333333333333"),
			Name:         "Water Pump Impeller",
			PartNumber:   "PN-PMP-IMP",
			Quantity:     8,
			Unit:         "pcs",
			Location:     "Warehouse B - Shelf 1",
			MinimumStock: 5,
			CompanyID:    &betaID,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
	}

	log.Println("Inserting inventory items...")
	for _, item := range items {
		if err := db.Create(&item).Error; err != nil {
			log.Fatalf("Failed to seed inventory item %s: %v", item.Name, err)
		}
		log.Printf("Successfully seeded inventory item %s", item.Name)
	}

	log.Println("Database seeding completed successfully!")
}
