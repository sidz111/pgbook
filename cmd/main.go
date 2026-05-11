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
	pgRepo := repositories.NewPGRepository(config.DB)
	roomRepo := repositories.NewRoomRepository(config.DB)
	tenantRepo := repositories.NewTenantRepository(config.DB)
	paymentRepo := repositories.NewPaymentRepository(config.DB)
	subscriptionRepo := repositories.NewSubscriptionRepository(config.DB)

	// Get JWT secret from env
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "pgbook-default-secret"
		slog.Warn("JWT_SECRET not set, using default development secret. Set JWT_SECRET in production.")
	}

	// Initialize services
	authService := services.NewAuthService(userRepo, jwtSecret)
	seedAdmin(authService, userRepo)
	pgService := services.NewPGService(pgRepo, roomRepo, tenantRepo, subscriptionRepo)
	roomService := services.NewRoomService(roomRepo, pgRepo)
	tenantService := services.NewTenantService(tenantRepo, roomRepo, pgRepo)
	paymentService := services.NewPaymentService(paymentRepo, tenantRepo, pgRepo, roomRepo)
	subscriptionService := services.NewSubscriptionService(subscriptionRepo, pgRepo)

	// Setup Gin router
	r := gin.Default()

	// Serve static frontend assets and templates
	r.Static("/static", "./web/static")
	r.LoadHTMLGlob("web/templates/*.html")

	// Recovery and middleware
	r.Use(gin.Recovery())
	r.Use(cors.Default())
	limiter := rate.NewLimiter(rate.Every(time.Second), 100) // 100 requests per second
	r.Use(middleware.RateLimitMiddleware(limiter))
	r.Use(middleware.HTTPSRedirectMiddleware())

	// Register API routes
	handlers.RegisterRoutes(r, jwtSecret, authService, pgService, roomService, tenantService, paymentService, subscriptionService)

	// Frontend view routes
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{"Page": "index"})
	})
	r.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", gin.H{"Page": "login"})
	})
	r.GET("/register", func(c *gin.Context) {
		c.HTML(http.StatusOK, "register.html", gin.H{"Page": "register"})
	})
	r.GET("/dashboard", func(c *gin.Context) {
		c.HTML(http.StatusOK, "dashboard.html", gin.H{"Page": "dashboard"})
	})
	r.GET("/owner", func(c *gin.Context) {
		c.HTML(http.StatusOK, "owner.html", gin.H{"Page": "owner"})
	})
	r.GET("/owner-profile", func(c *gin.Context) {
		c.HTML(http.StatusOK, "owner-profile.html", gin.H{"Page": "owner-profile"})
	})
	r.GET("/pg-manage", func(c *gin.Context) {
		c.HTML(http.StatusOK, "pg-manage.html", gin.H{"Page": "pg-manage"})
	})
	r.GET("/tenant-requests", func(c *gin.Context) {
		c.HTML(http.StatusOK, "tenant-requests.html", gin.H{"Page": "tenant-requests"})
	})
	r.GET("/tenant", func(c *gin.Context) {
		c.HTML(http.StatusOK, "tenant.html", gin.H{"Page": "tenant"})
	})
	r.GET("/tenant-profile", func(c *gin.Context) {
		c.HTML(http.StatusOK, "tenant-profile.html", gin.H{"Page": "tenant-profile"})
	})
	r.GET("/admin", func(c *gin.Context) {
		c.HTML(http.StatusOK, "admin.html", gin.H{"Page": "admin"})
	})

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

func seedAdmin(authService services.AuthService, userRepo repositories.UserRepository) {
	adminEmail := os.Getenv("ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = "admin@pgbook.local"
	}

	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		adminPassword = "Admin@123"
	}

	ctx := context.Background()
	if userRepo.EmailExists(ctx, adminEmail) {
		slog.Info("Admin user already exists", "email", adminEmail)
		return
	}

	adminUser := &models.User{
		Email:    adminEmail,
		Password: adminPassword,
		Role:     models.RoleAdmin,
	}

	if err := authService.Register(ctx, adminUser); err != nil {
		slog.Error("Failed to seed admin user", "error", err)
		return
	}

	slog.Info("Seeded admin user", "email", adminEmail)
}
