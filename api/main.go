package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/yourorg/deployment-pipeline/api/src/api/handlers"
	"github.com/yourorg/deployment-pipeline/api/src/api/repo"
	"github.com/yourorg/deployment-pipeline/api/src/common/middleware"
	"github.com/yourorg/deployment-pipeline/api/src/services"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	railpackSvc := services.NewRailpackService()
	dockerSvc := services.NewDockerService()
	caddySvc := services.NewCaddyService(os.Getenv("CADDY_ADMIN_URL"))
	deployRepo := repo.NewDeployRepo()
	deployHandler := handlers.NewDeployHandler(deployRepo, railpackSvc, dockerSvc, caddySvc)
	logsHandler := handlers.NewLogsHandler(deployRepo)
	r := gin.New()
	r.SetTrustedProxies([]string{"172.16.0.0/12", "10.0.0.0/8"}) // Docker networks
	r.Use(middleware.Logger())
	r.Use(middleware.CORS())
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api")
	{
		api.POST("/deploy", deployHandler.Trigger)
		api.GET("/deploys", deployHandler.List)
		api.POST("/deploys/:id/rollback", deployHandler.Rollback)
		api.GET("/deploys/:id/logs", logsHandler.Stream)
	}

	log.Printf("Sream-API listening on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
