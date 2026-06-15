package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/marines-dev/inventory-portal/internal/database"
)

func createDatabaseIfNotExists() error {
	_ = godotenv.Load()

	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")

	if host == "" { host = "localhost" }
	if port == "" { port = "5200" }
	if user == "" { user = "postgres" }
	if dbname == "" { dbname = "dev-inventory" }

	defaultDSN := fmt.Sprintf("postgres://%s:%s@%s:%s/postgres?sslmode=disable",
		user, password, host, port)

	db, err := sql.Open("postgres", defaultDSN)
	if err != nil {
		return err
	}
	defer db.Close()

	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT datname FROM pg_catalog.pg_database WHERE datname = $1)", dbname).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		log.Printf("Database '%s' does not exist. Creating database now...", dbname)
		_, err = db.Exec(fmt.Sprintf("CREATE DATABASE \"%s\"", dbname))
		if err != nil {
			return err
		}
		log.Println("Database created successfully.")
	}

	return nil
}

func main() {
	log.Println("Checking database existence...")
	if err := createDatabaseIfNotExists(); err != nil {
		log.Fatalf("Database existence check failed: %v", err)
	}

	log.Println("Initializing database connection for migrations...")
	db := database.InitDB()

	log.Println("Enabling uuid-ossp extension if missing...")
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"").Error; err != nil {
		log.Printf("Warning: Failed to enable uuid-ossp extension: %v", err)
	}

	log.Println("Running GORM AutoMigrate for Inventory schemas...")
	err := db.AutoMigrate(&database.InventoryItem{}, &database.MasterItem{}, &database.MasterWarehouse{}, &database.MasterUnit{})
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	log.Println("Migration completed successfully!")
}
