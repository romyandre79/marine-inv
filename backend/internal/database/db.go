package database

import (
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
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

func Paginate(c *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if c.Query("all") == "true" {
			return db
		}
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		if page <= 0 {
			page = 1
		}

		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
		switch {
		case limit > 100:
			limit = 100
		case limit <= 0:
			limit = 10
		}

		offset := (page - 1) * limit
		return db.Offset(offset).Limit(limit)
	}
}

type PaginationMeta struct {
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalPages int   `json:"total_pages"`
}

func GetPaginationMeta(c *gin.Context, total int64) PaginationMeta {
	if c.Query("all") == "true" {
		return PaginationMeta{
			Total:      total,
			Page:       1,
			Limit:      int(total),
			TotalPages: 1,
		}
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page <= 0 {
		page = 1
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	switch {
	case limit > 100:
		limit = 100
	case limit <= 0:
		limit = 10
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	return PaginationMeta{
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}
}
