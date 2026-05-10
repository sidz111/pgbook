package handlers

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/sidz111/pgbook/internals/middleware"
	"github.com/sidz111/pgbook/internals/services"
	"github.com/sidz111/pgbook/internals/utils"
)

// RegisterRoutes registers all API routes
func RegisterRoutes(
	router *gin.Engine,
	jwtSecret string,
	authService services.AuthService,
	pgService services.PGService,
	roomService services.RoomService,
	tenantService services.TenantService,
	paymentService services.PaymentService,
	subscriptionService services.SubscriptionService,
) {
	logger := slog.Default()

	// Initialize file upload service
	fileUploadConfig := utils.FileStorageConfig{
		StorageType: "local",
		LocalPath:   os.Getenv("UPLOAD_DIR"),
		MaxFileSize: 5 * 1024 * 1024, // 5MB
	}
	fileUploadService := utils.NewFileUploadService(fileUploadConfig)

	// Initialize handlers
	authHandler := NewAuthHandler(authService)
	pgHandler := NewPGHandler(pgService, roomService, tenantService, subscriptionService)
	roomHandler := NewRoomHandler(roomService, pgService)
	tenantHandler := NewTenantHandler(tenantService, roomService, pgService, fileUploadService)
	paymentHandler := NewPaymentHandler(paymentService, tenantService, pgService)
	subscriptionHandler := NewSubscriptionHandler(subscriptionService, pgService)

	// Public routes (no authentication)
	public := router.Group("/v1")
	{
		public.POST("/auth/register", authHandler.Register)
		public.POST("/auth/login", authHandler.Login)
		public.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})
	}

	// Protected routes (authentication required)
	protected := router.Group("/v1")
	protected.Use(middleware.AuthMiddleware(jwtSecret))
	{
		// Auth routes
		auth := protected.Group("/auth")
		{
			auth.POST("/refresh", authHandler.RefreshToken)
			auth.POST("/logout", authHandler.Logout)
		}

		// PG Routes
		pg := protected.Group("/pg")
		{
			pg.POST("/create", authHandler.checkOwnerRole(pgHandler.CreatePG))
			pg.GET("/my-pg", pgHandler.GetPGByOwner)
			pg.GET("/:pg_id", authHandler.checkOwnerOrAdmin(func(c *gin.Context) {
				pgHandler.GetPGDashboard(c)
			}))
			pg.PUT("/:pg_id", authHandler.checkOwnerOrAdmin(pgHandler.UpdatePG))
			pg.GET("/:pg_id/statistics", authHandler.checkOwnerOrAdmin(pgHandler.GetPGStatistics))
			pg.GET("/:pg_id/rooms/occupancy", authHandler.checkOwnerOrAdmin(pgHandler.GetRoomOccupancy))

			// Room routes under PG
			rooms := pg.Group("/:pg_id/rooms")
			{
				rooms.POST("/create", authHandler.checkOwnerOrAdmin(roomHandler.CreateRoom))
				rooms.GET("", authHandler.checkOwnerOrAdmin(roomHandler.GetRoomsByPG))
				rooms.GET("/:room_id", authHandler.checkOwnerOrAdmin(roomHandler.GetRoomByID))
				rooms.PUT("/:room_id", authHandler.checkOwnerOrAdmin(roomHandler.UpdateRoom))
				rooms.DELETE("/:room_id", authHandler.checkOwnerOrAdmin(roomHandler.DeleteRoom))
				rooms.GET("/:room_id/capacity", authHandler.checkOwnerOrAdmin(roomHandler.GetRoomCapacityStatus))
			}

			// Tenant routes under PG
			tenants := pg.Group("/:pg_id/tenants")
			{
				tenants.POST("", authHandler.checkOwnerOrAdmin(tenantHandler.CreateTenant))
				tenants.GET("", authHandler.checkOwnerOrAdmin(tenantHandler.ListTenants))
			}

			// Payment routes under PG
			payments := pg.Group("/:pg_id/payments")
			{
				payments.POST("", authHandler.checkOwnerOrAdmin(paymentHandler.CreatePayment))
				payments.GET("/pending", authHandler.checkOwnerOrAdmin(paymentHandler.GetPendingPayments))
				payments.GET("/stats", authHandler.checkOwnerOrAdmin(paymentHandler.GetPaymentStats))
				payments.GET("/monthly", authHandler.checkOwnerOrAdmin(paymentHandler.GetMonthlyStats))
				payments.GET("/upi-qr", paymentHandler.GetUPIQRCode)
			}

			// Subscription routes under PG
			subscriptions := pg.Group("/:pg_id/subscription")
			{
				subscriptions.POST("", authHandler.checkOwnerOrAdmin(subscriptionHandler.CreateSubscription))
				subscriptions.GET("/active", subscriptionHandler.GetActiveSubscription)
				subscriptions.GET("/history", authHandler.checkOwnerOrAdmin(subscriptionHandler.GetSubscriptionsByPG))
			}

			// Admin only routes
			adminPG := pg.Group("")
			adminPG.Use(authHandler.adminOnly)
			{
				adminPG.GET("/list", pgHandler.ListAllPGs)
			}
		}

		// Tenant Routes
		tenant := protected.Group("/tenant")
		{
			tenant.GET("/:tenant_id", tenantHandler.GetTenantDetails)
			tenant.GET("/:tenant_id/status", tenantHandler.GetTenantStatus)
			tenant.GET("/:tenant_id/payments", paymentHandler.GetTenantPayments)

			// File uploads
			files := tenant.Group("/:tenant_id")
			{
				files.POST("/upload-photo", tenantHandler.UploadProfilePhoto)
				files.POST("/upload-id-proof", tenantHandler.UploadIDProof)
			}

			// Notice period
			notice := tenant.Group("/:tenant_id")
			{
				notice.POST("/notice", tenantHandler.InitiateNotice)
				notice.POST("/cancel-notice", tenantHandler.CancelNotice)
			}
		}

		// Payment Routes
		payment := protected.Group("/payment")
		{
			payment.POST("/:payment_id/verify-cash", paymentHandler.VerifyCashPayment)
			payment.POST("/:payment_id/verify-upi", paymentHandler.VerifyUPIPayment)
			payment.POST("/:payment_id/reject", paymentHandler.RejectPayment)
		}

		// Subscription Routes
		subscription := protected.Group("/subscription")
		{
			subscription.GET("/:sub_id", subscriptionHandler.GetSubscriptionByID)

			// Admin only
			adminSub := subscription.Group("")
			adminSub.Use(authHandler.adminOnly)
			{
				adminSub.POST("/:sub_id/approve", subscriptionHandler.ApproveSubscription)
				adminSub.POST("/:sub_id/reject", subscriptionHandler.RejectSubscription)
				adminSub.GET("/pending", subscriptionHandler.GetPendingSubscriptions)
			}
		}

		// Document access (secure)
		documents := protected.Group("/documents")
		{
			documents.GET("/:filename", NewDocumentHandler(fileUploadService, tenantService, pgService).ServeDocument)
		}
	}

	logger.Info("Routes registered successfully")
}

// Helper methods for role-based access

// checkOwnerRole middleware
func (h *AuthHandler) checkOwnerRole(handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || role != "pg_owner" {
			c.JSON(http.StatusForbidden, gin.H{"error": "PG owner access required"})
			c.Abort()
			return
		}
		handler(c)
	}
}

// checkOwnerOrAdmin middleware
func (h *AuthHandler) checkOwnerOrAdmin(handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || (role != "pg_owner" && role != "admin") {
			c.JSON(http.StatusForbidden, gin.H{"error": "owner or admin access required"})
			c.Abort()
			return
		}
		handler(c)
	}
}

// adminOnly middleware
func (h *AuthHandler) adminOnly(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		c.Abort()
		return
	}
	c.Next()
}
