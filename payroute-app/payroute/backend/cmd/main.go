package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/payroute/backend/internal/database"
	"github.com/payroute/backend/internal/handlers"
)

func main() {
	godotenv.Load()
	database.Connect()

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:  []string{"*"},
		AllowMethods:  []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:  []string{"Origin", "Content-Type", "Idempotency-Key", "X-Webhook-Signature"},
		ExposeHeaders: []string{"Content-Length"},
	}))
	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
		api.GET("/accounts", handlers.GetAccounts)
		api.GET("/fx/quote", handlers.GetFXQuote)
		api.POST("/payments", handlers.CreatePayment)
		api.GET("/payments", handlers.ListPayments)
		api.GET("/payments/:id", handlers.GetPayment)
		api.POST("/webhooks/provider", handlers.HandleWebhook)
		api.POST("/webhooks/simulate", handlers.SimulateWebhook)
	}

	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	staticPath := os.Getenv("STATIC_FILES_PATH")
	if staticPath == "" {
		staticPath = "../frontend/build"
	}

	r.NoRoute(func(c *gin.Context) {
		path := filepath.Join(staticPath, c.Request.URL.Path)
		if _, err := os.Stat(path); err == nil {
			c.File(path)
			return
		}
		c.File(filepath.Join(staticPath, "index.html"))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("PayRoute backend running on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
