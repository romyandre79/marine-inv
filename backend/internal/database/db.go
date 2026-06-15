package database

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB() *gorm.DB {
	// Try loading env
	_ = godotenv.Load()

	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")
	sslmode := os.Getenv("DB_SSLMODE")
	maxOpenStr := os.Getenv("DB_MAX_OPEN")
	maxIdleStr := os.Getenv("DB_MAX_IDLE")
	maxLifetimeStr := os.Getenv("DB_MAX_LIFETIME")

	if host == "" { host = "localhost" }
	if port == "" { port = "5200" }
	if user == "" { user = "postgres" }
	if dbname == "" { dbname = "dev-inventory" }
	if sslmode == "" { sslmode = "disable" }

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Jakarta",
		host, port, user, password, dbname, sslmode)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatalf("Failed to retrieve sql.DB instance: %v", err)
	}

	maxOpen := 25
	if maxOpenStr != "" {
		if val, err := strconv.Atoi(maxOpenStr); err == nil {
			maxOpen = val
		}
	}

	maxIdle := 10
	if maxIdleStr != "" {
		if val, err := strconv.Atoi(maxIdleStr); err == nil {
			maxIdle = val
		}
	}

	maxLifetime := 30
	if maxLifetimeStr != "" {
		if val, err := strconv.Atoi(maxLifetimeStr); err == nil {
			maxLifetime = val
		}
	}

	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(time.Duration(maxLifetime) * time.Minute)

	log.Printf("Database connection successfully established for INVENTORY (sslmode=%s, max_open=%d, max_idle=%d, max_lifetime=%dm).",
		sslmode, maxOpen, maxIdle, maxLifetime)
	return DB
}
