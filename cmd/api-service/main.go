package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/meowiky/WAC-Physio_Bukovska-Biely-API/api"
	"github.com/meowiky/WAC-Physio_Bukovska-Biely-API/internal/physio"
)

func main() {
	log.Printf("Server started")
	port := os.Getenv("PHYSIO_API_PORT")
	if port == "" {
		port = "8080"
	}
	environment := os.Getenv("PHYSIO_API_ENVIRONMENT")
	if !strings.EqualFold(environment, "production") { // case insensitive comparison
		gin.SetMode(gin.DebugMode)
	}

	server, err := physio.NewServer(context.Background())
	if err != nil {
		log.Fatalf("failed to initialize mongo connection: %v", err)
	}
	defer func() {
		if err := server.Disconnect(context.Background()); err != nil {
			log.Printf("failed to disconnect mongo client: %v", err)
		}
	}()

	engine := gin.New()
	engine.Use(gin.Recovery())
	corsMiddleware := cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "PUT", "POST", "DELETE", "PATCH"},
		AllowHeaders:     []string{"Origin", "Authorization", "Content-Type"},
		ExposeHeaders:    []string{""},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	})
	engine.Use(corsMiddleware)
	engine.GET("/openapi", api.HandleOpenApi)
	engine.GET("/swagger", api.HandleSwaggerUI)

	physio.NewRouterWithGinEngine(engine, physio.ApiHandleFunctions{
		AmbulancesAPI:             server,
		AvailabilityAPI:           server,
		PatientsAPI:               server,
		RehabilitationPlansAPI:    server,
		RehabilitationSessionsAPI: server,
		TherapistsAPI:             server,
	})

	if err := engine.Run(":" + port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
