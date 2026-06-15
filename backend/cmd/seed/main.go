package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/marines-dev/inventory-portal/internal/database"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	log.Println("Initializing database connection for seeding...")
	db := database.InitDB()

	// Connect to dev-mms to fetch PTMI's UUID
	var ptmiID uuid.UUID
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	sslmode := os.Getenv("DB_SSLMODE")

	if host == "" { host = "marines.web.id" }
	if port == "" { port = "5200" }
	if user == "" { user = "postgres" }
	if password == "" { password = "m4r1n3s" }
	if sslmode == "" { sslmode = "disable" }

	mmsDsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=dev-mms sslmode=%s TimeZone=Asia/Jakarta",
		host, port, user, password, sslmode)
	mmsDB, err := gorm.Open(postgres.Open(mmsDsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err == nil {
		type Company struct {
			ID   uuid.UUID
			Code string
		}
		var comp Company
		if err := mmsDB.Table("companies").Where("code = ?", "PTMI").First(&comp).Error; err == nil {
			ptmiID = comp.ID
			log.Printf("Successfully fetched PTMI ID from MMS database: %s", ptmiID)
		} else {
			log.Printf("Failed to query PTMI company from MMS database: %v", err)
		}
	} else {
		log.Printf("Failed to connect to MMS database to fetch PTMI ID: %v", err)
	}

	// Fallback to random if not found in DB
	if ptmiID == uuid.Nil {
		ptmiID = uuid.New()
		log.Printf("Using fallback random ID for PTMI: %s", ptmiID)
	}

	// Clear existing inventory
	log.Println("Clearing existing inventory and master tables...")
	db.Exec("TRUNCATE TABLE inventory, master_items, master_warehouses, master_units RESTART IDENTITY CASCADE")

	acmeID := uuid.MustParse("10e31e63-386d-465e-9b10-3d72abc56b95")
	betaID := uuid.MustParse("2f13615c-1dbe-4189-9826-8e0b358532b7")

	// 1. Seed Units
	units := []database.MasterUnit{
		{ID: uuid.New(), Name: "Pieces", Code: "pcs", CompanyID: nil},
		{ID: uuid.New(), Name: "Liters", Code: "ltr", CompanyID: nil},
		{ID: uuid.New(), Name: "Cans", Code: "can", CompanyID: nil},
		{ID: uuid.New(), Name: "Drums", Code: "drum", CompanyID: nil},
		{ID: uuid.New(), Name: "Boxes", Code: "box", CompanyID: nil},
		{ID: uuid.New(), Name: "Bags", Code: "bag", CompanyID: nil},
		{ID: uuid.New(), Name: "Kilograms", Code: "kg", CompanyID: nil},
		{ID: uuid.New(), Name: "Meters", Code: "m", CompanyID: nil},
		{ID: uuid.New(), Name: "Rolls", Code: "roll", CompanyID: nil},
		{ID: uuid.New(), Name: "Sets", Code: "set", CompanyID: nil},
		{ID: uuid.New(), Name: "Pairs", Code: "pair", CompanyID: nil},
		{ID: uuid.New(), Name: "Units", Code: "unit", CompanyID: nil},
		{ID: uuid.New(), Name: "Cylinders", Code: "cyl", CompanyID: nil},
		{ID: uuid.New(), Name: "Packages", Code: "pkg", CompanyID: nil},
		{ID: uuid.New(), Name: "Bottles", Code: "btl", CompanyID: nil},
		{ID: uuid.New(), Name: "Pails", Code: "pail", CompanyID: nil},
		{ID: uuid.New(), Name: "Tubes", Code: "tub", CompanyID: nil},
		{ID: uuid.New(), Name: "Cartons", Code: "ctn", CompanyID: nil},
		{ID: uuid.New(), Name: "Barrels", Code: "bbl", CompanyID: nil},
	}
	log.Println("Inserting master units...")
	for _, u := range units {
		if err := db.Create(&u).Error; err != nil {
			log.Printf("Failed to seed unit %s: %v", u.Code, err)
		}
	}

	vesselCrapolla := uuid.MustParse("c1111111-1111-1111-1111-111111111111")
	vesselKelud := uuid.MustParse("c2222222-2222-2222-2222-222222222222")
	vesselDobonsolo := uuid.MustParse("c3333333-3333-3333-3333-333333333333")
	vesselSinabung := uuid.MustParse("c4444444-4444-4444-4444-444444444444")

	// 2. Seed Warehouses
	warehouses := []database.MasterWarehouse{
		// ACME Company - MSC CRAPOLLA
		{ID: uuid.New(), Name: "Main Engine Room", Code: "WH-ENG-01", Address: "Lower Deck - Section 4", VesselID: &vesselCrapolla, CompanyID: &acmeID},
		{ID: uuid.New(), Name: "Steering Gear Room", Code: "WH-STEER-01", Address: "Aft Peak Deck", VesselID: &vesselCrapolla, CompanyID: &acmeID},
		{ID: uuid.New(), Name: "Paint Locker", Code: "WH-PAINT-01", Address: "Forecastle Deck", VesselID: &vesselCrapolla, CompanyID: &acmeID},
		{ID: uuid.New(), Name: "Bow Thruster Room", Code: "WH-BOW-01", Address: "Lower Bow Section", VesselID: &vesselCrapolla, CompanyID: &acmeID},

		// ACME Company - KM Kelud
		{ID: uuid.New(), Name: "Deck Storage A", Code: "WH-DECK-A", Address: "Main Deck - Portside", VesselID: &vesselKelud, CompanyID: &acmeID},
		{ID: uuid.New(), Name: "Galley Store", Code: "WH-GALLEY-01", Address: "Accommodation Deck 2", VesselID: &vesselKelud, CompanyID: &acmeID},
		{ID: uuid.New(), Name: "Workshop Room", Code: "WH-WORK-01", Address: "Engine Deck 2", VesselID: &vesselKelud, CompanyID: &acmeID},

		// BETA Company - KM Dobonsolo
		{ID: uuid.New(), Name: "Shore Warehouse East", Code: "WH-SHORE-E", Address: "Port Terminal Building 2", VesselID: &vesselDobonsolo, CompanyID: &betaID},

		// BETA Company - KM Sinabung
		{ID: uuid.New(), Name: "Shore Warehouse West", Code: "WH-SHORE-W", Address: "Port Terminal Building 5", VesselID: &vesselSinabung, CompanyID: &betaID},

		// PTMI Company (No vessel, global central shore store)
		{ID: uuid.New(), Name: "PTMI Central Warehouse", Code: "WH-PTMI-01", Address: "Jakarta Head Office Ground Floor", VesselID: nil, CompanyID: &ptmiID},
	}
	log.Println("Inserting master warehouses...")
	for _, w := range warehouses {
		if err := db.Create(&w).Error; err != nil {
			log.Printf("Failed to seed warehouse %s: %v", w.Name, err)
		}
	}

	// 3. Seed Master Items
	mItems := []database.MasterItem{
		// ACME items
		{ID: uuid.New(), Name: "Engine Oil 15W-40", PartNumber: "PN-OIL-15W40", Unit: "ltr", Description: "High performance diesel engine oil", CompanyID: &acmeID},
		{ID: uuid.New(), Name: "Fuel Filter Element", PartNumber: "PN-FLT-ELEM", Unit: "pcs", Description: "Primary fuel filtration cartridge", CompanyID: &acmeID},
		{ID: uuid.New(), Name: "Oil Filter Cartridge", PartNumber: "PN-FLT-OIL", Unit: "pcs", Description: "Engine lube oil filtration cartridge", CompanyID: &acmeID},
		{ID: uuid.New(), Name: "Air Filter Element", PartNumber: "PN-FLT-AIR", Unit: "pcs", Description: "Main generator engine air intake filter", CompanyID: &acmeID},
		{ID: uuid.New(), Name: "Centrifugal Pump Gasket", PartNumber: "PN-GSK-PMP", Unit: "pcs", Description: "Neoprene gasket for cooling water pump", CompanyID: &acmeID},
		{ID: uuid.New(), Name: "LED Floodlight 50W", PartNumber: "PN-LGT-LED50", Unit: "pcs", Description: "IP67 outdoor deck illumination floodlight", CompanyID: &acmeID},
		{ID: uuid.New(), Name: "Pressure Gauge 0-10 Bar", PartNumber: "PN-GAU-10B", Unit: "pcs", Description: "Glycerin-filled dial gauge for fuel line", CompanyID: &acmeID},
		{ID: uuid.New(), Name: "Hydraulic Valve Seal", PartNumber: "PN-SEL-HYD", Unit: "set", Description: "NBR o-ring kit for hydraulic actuators", CompanyID: &acmeID},
		{ID: uuid.New(), Name: "Welding Electrodes", PartNumber: "PN-WLD-ROD", Unit: "box", Description: "E6013 mild steel welding rods (5kg)", CompanyID: &acmeID},
		{ID: uuid.New(), Name: "Safety Helmet", PartNumber: "PN-PPE-HELM", Unit: "pcs", Description: "High-density polyethylene industrial hard hat", CompanyID: &acmeID},
		{ID: uuid.New(), Name: "Fire Extinguisher CO2 5kg", PartNumber: "PN-SAF-CO2", Unit: "pcs", Description: "Portable CO2 fire extinguisher class B/C", CompanyID: &acmeID},
		{ID: uuid.New(), Name: "Cotton Rags", PartNumber: "PN-CSM-RAG", Unit: "kg", Description: "General purpose cleaning cotton rags", CompanyID: &acmeID},
		{ID: uuid.New(), Name: "Anchor Shackle", PartNumber: "PN-RIG-SHK", Unit: "pcs", Description: "Galvanized bow shackle 25-ton capacity", CompanyID: &acmeID},

		// BETA items
		{ID: uuid.New(), Name: "Water Pump Impeller", PartNumber: "PN-PMP-IMP", Unit: "pcs", Description: "Flexible water pump impeller replacement", CompanyID: &betaID},
		{ID: uuid.New(), Name: "Engine Oil 15W-40", PartNumber: "PN-OIL-15W40", Unit: "ltr", Description: "High performance diesel engine oil", CompanyID: &betaID},
		{ID: uuid.New(), Name: "Welding Electrodes", PartNumber: "PN-WLD-ROD", Unit: "box", Description: "E6013 mild steel welding rods (5kg)", CompanyID: &betaID},
		{ID: uuid.New(), Name: "Oil Filter Cartridge", PartNumber: "PN-FLT-OIL", Unit: "pcs", Description: "Engine lube oil filtration cartridge", CompanyID: &betaID},
		{ID: uuid.New(), Name: "Cotton Rags", PartNumber: "PN-CSM-RAG", Unit: "kg", Description: "General purpose cleaning cotton rags", CompanyID: &betaID},
		{ID: uuid.New(), Name: "Anchor Shackle", PartNumber: "PN-RIG-SHK", Unit: "pcs", Description: "Galvanized bow shackle 25-ton capacity", CompanyID: &betaID},
		{ID: uuid.New(), Name: "LED Floodlight 50W", PartNumber: "PN-LGT-LED50", Unit: "pcs", Description: "IP67 outdoor deck illumination floodlight", CompanyID: &betaID},
		{ID: uuid.New(), Name: "Fire Extinguisher CO2 5kg", PartNumber: "PN-SAF-CO2", Unit: "pcs", Description: "Portable CO2 fire extinguisher class B/C", CompanyID: &betaID},

		// PTMI items
		{ID: uuid.New(), Name: "Engine Oil 15W-40", PartNumber: "PN-OIL-15W40", Unit: "ltr", Description: "High performance diesel engine oil", CompanyID: &ptmiID},
		{ID: uuid.New(), Name: "Fuel Filter Element", PartNumber: "PN-FLT-ELEM", Unit: "pcs", Description: "Primary fuel filtration cartridge", CompanyID: &ptmiID},
		{ID: uuid.New(), Name: "Safety Helmet", PartNumber: "PN-PPE-HELM", Unit: "pcs", Description: "High-density polyethylene industrial hard hat", CompanyID: &ptmiID},
		{ID: uuid.New(), Name: "LED Floodlight 50W", PartNumber: "PN-LGT-LED50", Unit: "pcs", Description: "IP67 outdoor deck illumination floodlight", CompanyID: &ptmiID},
	}
	log.Println("Inserting master items...")
	for _, mi := range mItems {
		if err := db.Create(&mi).Error; err != nil {
			log.Printf("Failed to seed master item %s: %v", mi.Name, err)
		}
	}

	// 4. Seed Inventory Stock
	items := []database.InventoryItem{
		// ACME Company - MSC CRAPOLLA (Main Engine Room)
		{ID: uuid.New(), Name: "Engine Oil 15W-40", PartNumber: "PN-OIL-15W40", Quantity: 250, Unit: "ltr", Location: "Main Engine Room", MinimumStock: 50, CompanyID: &acmeID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "Fuel Filter Element", PartNumber: "PN-FLT-ELEM", Quantity: 24, Unit: "pcs", Location: "Main Engine Room", MinimumStock: 15, CompanyID: &acmeID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "Oil Filter Cartridge", PartNumber: "PN-FLT-OIL", Quantity: 18, Unit: "pcs", Location: "Main Engine Room", MinimumStock: 10, CompanyID: &acmeID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "Air Filter Element", PartNumber: "PN-FLT-AIR", Quantity: 12, Unit: "pcs", Location: "Main Engine Room", MinimumStock: 5, CompanyID: &acmeID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "Pressure Gauge 0-10 Bar", PartNumber: "PN-GAU-10B", Quantity: 5, Unit: "pcs", Location: "Main Engine Room", MinimumStock: 2, CompanyID: &acmeID, CreatedAt: time.Now(), UpdatedAt: time.Now()},

		// ACME Company - MSC CRAPOLLA (Steering Gear Room)
		{ID: uuid.New(), Name: "Hydraulic Valve Seal", PartNumber: "PN-SEL-HYD", Quantity: 8, Unit: "set", Location: "Steering Gear Room", MinimumStock: 2, CompanyID: &acmeID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "Centrifugal Pump Gasket", PartNumber: "PN-GSK-PMP", Quantity: 15, Unit: "pcs", Location: "Steering Gear Room", MinimumStock: 5, CompanyID: &acmeID, CreatedAt: time.Now(), UpdatedAt: time.Now()},

		// ACME Company - MSC CRAPOLLA (Paint Locker)
		{ID: uuid.New(), Name: "Cotton Rags", PartNumber: "PN-CSM-RAG", Quantity: 50, Unit: "kg", Location: "Paint Locker", MinimumStock: 10, CompanyID: &acmeID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "Safety Helmet", PartNumber: "PN-PPE-HELM", Quantity: 10, Unit: "pcs", Location: "Paint Locker", MinimumStock: 2, CompanyID: &acmeID, CreatedAt: time.Now(), UpdatedAt: time.Now()},

		// ACME Company - MSC CRAPOLLA (Bow Thruster Room)
		{ID: uuid.New(), Name: "Centrifugal Pump Gasket", PartNumber: "PN-GSK-PMP", Quantity: 10, Unit: "pcs", Location: "Bow Thruster Room", MinimumStock: 3, CompanyID: &acmeID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "Pressure Gauge 0-10 Bar", PartNumber: "PN-GAU-10B", Quantity: 3, Unit: "pcs", Location: "Bow Thruster Room", MinimumStock: 1, CompanyID: &acmeID, CreatedAt: time.Now(), UpdatedAt: time.Now()},

		// ACME Company - KM Kelud (Deck Storage A)
		{ID: uuid.New(), Name: "LED Floodlight 50W", PartNumber: "PN-LGT-LED50", Quantity: 15, Unit: "pcs", Location: "Deck Storage A", MinimumStock: 5, CompanyID: &acmeID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "Safety Helmet", PartNumber: "PN-PPE-HELM", Quantity: 40, Unit: "pcs", Location: "Deck Storage A", MinimumStock: 10, CompanyID: &acmeID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "Fire Extinguisher CO2 5kg", PartNumber: "PN-SAF-CO2", Quantity: 8, Unit: "pcs", Location: "Deck Storage A", MinimumStock: 4, CompanyID: &acmeID, CreatedAt: time.Now(), UpdatedAt: time.Now()},

		// ACME Company - KM Kelud (Galley Store)
		{ID: uuid.New(), Name: "Cotton Rags", PartNumber: "PN-CSM-RAG", Quantity: 25, Unit: "kg", Location: "Galley Store", MinimumStock: 5, CompanyID: &acmeID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "Fire Extinguisher CO2 5kg", PartNumber: "PN-SAF-CO2", Quantity: 4, Unit: "pcs", Location: "Galley Store", MinimumStock: 1, CompanyID: &acmeID, CreatedAt: time.Now(), UpdatedAt: time.Now()},

		// ACME Company - KM Kelud (Workshop Room)
		{ID: uuid.New(), Name: "Welding Electrodes", PartNumber: "PN-WLD-ROD", Quantity: 20, Unit: "box", Location: "Workshop Room", MinimumStock: 5, CompanyID: &acmeID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "Safety Helmet", PartNumber: "PN-PPE-HELM", Quantity: 12, Unit: "pcs", Location: "Workshop Room", MinimumStock: 3, CompanyID: &acmeID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "Hydraulic Valve Seal", PartNumber: "PN-SEL-HYD", Quantity: 5, Unit: "set", Location: "Workshop Room", MinimumStock: 1, CompanyID: &acmeID, CreatedAt: time.Now(), UpdatedAt: time.Now()},

		// BETA Company - KM Dobonsolo (Shore Warehouse East)
		{ID: uuid.New(), Name: "Water Pump Impeller", PartNumber: "PN-PMP-IMP", Quantity: 12, Unit: "pcs", Location: "Shore Warehouse East", MinimumStock: 5, CompanyID: &betaID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "Welding Electrodes", PartNumber: "PN-WLD-ROD", Quantity: 15, Unit: "box", Location: "Shore Warehouse East", MinimumStock: 5, CompanyID: &betaID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "Engine Oil 15W-40", PartNumber: "PN-OIL-15W40", Quantity: 100, Unit: "ltr", Location: "Shore Warehouse East", MinimumStock: 20, CompanyID: &betaID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "Oil Filter Cartridge", PartNumber: "PN-FLT-OIL", Quantity: 10, Unit: "pcs", Location: "Shore Warehouse East", MinimumStock: 3, CompanyID: &betaID, CreatedAt: time.Now(), UpdatedAt: time.Now()},

		// BETA Company - KM Sinabung (Shore Warehouse West)
		{ID: uuid.New(), Name: "Cotton Rags", PartNumber: "PN-CSM-RAG", Quantity: 120, Unit: "kg", Location: "Shore Warehouse West", MinimumStock: 30, CompanyID: &betaID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "Anchor Shackle", PartNumber: "PN-RIG-SHK", Quantity: 6, Unit: "pcs", Location: "Shore Warehouse West", MinimumStock: 2, CompanyID: &betaID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "LED Floodlight 50W", PartNumber: "PN-LGT-LED50", Quantity: 8, Unit: "pcs", Location: "Shore Warehouse West", MinimumStock: 2, CompanyID: &betaID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "Fire Extinguisher CO2 5kg", PartNumber: "PN-SAF-CO2", Quantity: 6, Unit: "pcs", Location: "Shore Warehouse West", MinimumStock: 2, CompanyID: &betaID, CreatedAt: time.Now(), UpdatedAt: time.Now()},

		// PTMI Company (PTMI Central Warehouse)
		{ID: uuid.New(), Name: "Engine Oil 15W-40", PartNumber: "PN-OIL-15W40", Quantity: 500, Unit: "ltr", Location: "PTMI Central Warehouse", MinimumStock: 100, CompanyID: &ptmiID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "Fuel Filter Element", PartNumber: "PN-FLT-ELEM", Quantity: 100, Unit: "pcs", Location: "PTMI Central Warehouse", MinimumStock: 20, CompanyID: &ptmiID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "Safety Helmet", PartNumber: "PN-PPE-HELM", Quantity: 150, Unit: "pcs", Location: "PTMI Central Warehouse", MinimumStock: 30, CompanyID: &ptmiID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: uuid.New(), Name: "LED Floodlight 50W", PartNumber: "PN-LGT-LED50", Quantity: 50, Unit: "pcs", Location: "PTMI Central Warehouse", MinimumStock: 10, CompanyID: &ptmiID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}

	log.Println("Inserting inventory items...")
	for _, item := range items {
		if err := db.Create(&item).Error; err != nil {
			log.Fatalf("Failed to seed inventory item %s: %v", item.Name, err)
		}
		log.Printf("Successfully seeded inventory item %s for location %s", item.Name, item.Location)
	}

	log.Println("Database seeding completed successfully!")
}
