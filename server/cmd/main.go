package main

import (
	"context"
	"log"
	"os"
	"time"
	
	"github.com/YogendrasinghRathod/server/internal/auth"
	"github.com/YogendrasinghRathod/server/pkg/routes"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Print("No .env file found")
	}

	// Validate required environment variables
	requiredVars := []string{"JWT_SECRET", "STORAGE_PATH"}
	for _, envVar := range requiredVars {
		if os.Getenv(envVar) == "" {
			log.Fatalf("%s environment variable not set", envVar)
		}
	}

	// Initialize database connection
	db, err := sqlx.Connect("postgres", "user=postgres dbname=fileshare password=yogi1234 sslmode=disable")
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "Yogendra@14",
		DB:       0,
	})

	// Verify Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := redisClient.Ping(ctx).Result(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Create auth handler
	authHandler, err := auth.NewAuthHandler(db)
	if err != nil {
		log.Fatalf("Failed to create auth handler: %v", err)
	}

	// Initialize Gin router
	router := gin.Default()

	// Setup routes (now with correct parameters)
	routes.SetupRoutes(router, db, redisClient, authHandler)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}