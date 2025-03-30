package routes

import (
	"github.com/YogendrasinghRathod/server/internal/auth"
	"github.com/YogendrasinghRathod/server/internal/file"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9" // Updated to v9
	"github.com/jmoiron/sqlx"
)

func SetupRoutes(
	router *gin.Engine,
	db *sqlx.DB,
	redisClient *redis.Client, // Now using v9 client type
	authHandler *auth.AuthHandler,
) {
	// Initialize file handler
	fileHandler := file.NewFileHandler(
		"./uploads",
		db,
		redisClient,
	)

	// Public routes
	public := router.Group("/")
	{
		public.POST("/login", authHandler.Login)
		public.POST("/register", authHandler.Register)
		public.GET("/share/:token", fileHandler.ServeSharedFile)
	}

	// Protected routes
	protected := router.Group("/")
	protected.Use(authHandler.AuthMiddleware())
	{
		protected.POST("/upload", fileHandler.Upload)
		protected.GET("/files", fileHandler.GetUserFiles)
		protected.GET("/files/:file_id/download", fileHandler.Download)
		protected.POST("/files/:file_id/share", fileHandler.CreateShareLink)
	}
}