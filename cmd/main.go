package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sidz111/pgbook/config"
	"github.com/sidz111/pgbook/internals/handlers"
	"github.com/sidz111/pgbook/internals/middleware"
	"github.com/sidz111/pgbook/internals/models"
	"github.com/sidz111/pgbook/internals/repositories"
	"github.com/sidz111/pgbook/internals/services"
	"golang.org/x/time/rate"
)

func main() {
	// Initialize logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
	slog.SetDefault(logger)

	slog.Info("Welcome to PGBook - Your Ultimate PG Management Solution!")

	if err := config.ConnectDB(); err != nil {
		slog.Error("Error connecting to database", "error", err)
		return
	}
	slog.Info("Successfully connected to the database!")

	// Auto-migrate models
	if err := config.DB.AutoMigrate(&models.User{}, &models.PG{}, &models.Room{}, &models.Tenant{}, &models.Payment{}, &models.Subscription{}); err != nil {
		slog.Error("Error migrating database", "error", err)
		return
	}
	slog.Info("Database migration completed!")

	// Initialize repositories
	userRepo := repositories.NewUserRepository(config.DB)

	// Get JWT secret from env
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		slog.Error("JWT_SECRET not set")
		return
	}

	// Initialize services
	authService := services.NewAuthService(userRepo, jwtSecret)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(authService)

	// Setup Gin router
	r := gin.Default()

	// Recovery middleware
	r.Use(gin.Recovery())

	// CORS middleware
	r.Use(cors.Default())

	// Rate limiting middleware for auth routes
	limiter := rate.NewLimiter(rate.Every(time.Minute), 10) // 10 requests per minute
	r.Use(middleware.RateLimitMiddleware(limiter))

	// HTTPS enforcement middleware
	r.Use(middleware.HTTPSRedirectMiddleware())

	// API versioning
	v1 := r.Group("/v1")
	{
		// Health check
		v1.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// Auth routes
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/refresh", authHandler.RefreshToken)
			auth.POST("/logout", middleware.AuthMiddleware(jwtSecret), authHandler.Logout)
		}

		// Protected routes (add more as needed)
		protected := v1.Group("/")
		protected.Use(middleware.AuthMiddleware(jwtSecret))
		{
			// Add other handlers here
		}
	}

	// Start server
	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	go func() {
		slog.Info("Starting server on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed to start", "error", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	slog.Info("Server exited")
}
