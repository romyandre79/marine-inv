package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/marines-dev/inventory-portal/internal/database"
	"github.com/marines-dev/inventory-portal/internal/middleware"
	"github.com/marines-dev/inventory-portal/internal/modules/inventory"
	"github.com/marines-dev/inventory-portal/internal/modules/masteritems"
	"github.com/marines-dev/inventory-portal/internal/modules/transfers"
	"github.com/marines-dev/inventory-portal/internal/modules/units"
	"github.com/marines-dev/inventory-portal/internal/modules/warehouses"
)

func main() {
	// Load environment variables from .env
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "3014"
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "22c3b650de1658b27a286d4e00f10dee9232cacf847e9a6c387a1e616aed94f6"
	}

	// Initialize DB Connection
	database.InitDB()

	r := gin.Default()

	// CORS Middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Tenant-ID")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "UP", "service": "Inventory Management System"})
	})

	// Protected routes
	api := r.Group("/api/v1")
	api.Use(middleware.JWTAuth(secret))
	{
		inventory.RegisterRoutes(api)
		masteritems.RegisterRoutes(api)
		warehouses.RegisterRoutes(api)
		units.RegisterRoutes(api)
		transfers.RegisterRoutes(api)
	}

	log.Printf("Inventory Backend running on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to run Inventory server: %v", err)
	}
}
