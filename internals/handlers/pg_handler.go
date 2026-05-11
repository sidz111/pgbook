package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sidz111/pgbook/internals/models"
	"github.com/sidz111/pgbook/internals/services"
)

type PGHandler struct {
	pgService           services.PGService
	roomService         services.RoomService
	tenantService       services.TenantService
	subscriptionService services.SubscriptionService
	logger              *slog.Logger
}

func NewPGHandler(
	pgService services.PGService,
	roomService services.RoomService,
	tenantService services.TenantService,
	subscriptionService services.SubscriptionService,
) *PGHandler {
	return &PGHandler{
		pgService:           pgService,
		roomService:         roomService,
		tenantService:       tenantService,
		subscriptionService: subscriptionService,
		logger:              slog.Default(),
	}
}

// CreatePG - POST /v1/pg/create
// CreatePGRequest defines request structure
type CreatePGRequest struct {
	Name       string `json:"name" binding:"required"`
	OwnerName  string `json:"owner_name" binding:"required"`
	Phone      string `json:"phone" binding:"required"`
	Address    string `json:"address" binding:"required"`
	ScannerURL string `json:"scanner_url"`
}

func (h *PGHandler) CreatePG(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req CreatePGRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userUUID, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	pg := &models.PG{
		UserID:     userUUID,
		Name:       req.Name,
		OwnerName:  req.OwnerName,
		Phone:      req.Phone,
		Address:    req.Address,
		ScannerURL: req.ScannerURL,
	}

	if err := h.pgService.CreatePG(c.Request.Context(), pg); err != nil {
		h.logger.Error("Failed to create PG", "error", err, "user_id", userID)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "PG created successfully",
		"pg_id":   pg.ID,
	})
}

// GetPGByOwner - GET /v1/pg/my-pg
func (h *PGHandler) GetPGByOwner(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userUUID, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	pg, err := h.pgService.GetPGByOwner(c.Request.Context(), userUUID)
	if err != nil {
		h.logger.Error("Failed to get PG", "error", err, "user_id", userID)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, pg)
}

// GetPGDashboard - GET /v1/pg/:pg_id/dashboard
func (h *PGHandler) GetPGDashboard(c *gin.Context) {
	pgID, err := uuid.Parse(c.Param("pg_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid PG ID"})
		return
	}

	// Verify ownership or admin role
	if !h.verifyPGAccess(c, pgID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	dashboard, err := h.pgService.GetDashboardData(c.Request.Context(), pgID)
	if err != nil {
		h.logger.Error("Failed to get dashboard", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, dashboard)
}

// UpdatePG - PUT /v1/pg/:pg_id
type UpdatePGRequest struct {
	Name       string `json:"name"`
	OwnerName  string `json:"owner_name"`
	Phone      string `json:"phone"`
	Address    string `json:"address"`
	ScannerURL string `json:"scanner_url"`
}

func (h *PGHandler) UpdatePG(c *gin.Context) {
	pgID, err := uuid.Parse(c.Param("pg_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid PG ID"})
		return
	}

	if !h.verifyPGAccess(c, pgID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	var req UpdatePGRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pg, err := h.pgService.GetPGByID(c.Request.Context(), pgID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "PG not found"})
		return
	}

	if req.Name != "" {
		pg.Name = req.Name
	}
	if req.OwnerName != "" {
		pg.OwnerName = req.OwnerName
	}
	if req.Phone != "" {
		pg.Phone = req.Phone
	}
	if req.Address != "" {
		pg.Address = req.Address
	}
	if req.ScannerURL != "" {
		pg.ScannerURL = req.ScannerURL
	}

	if err := h.pgService.UpdatePG(c.Request.Context(), pg); err != nil {
		h.logger.Error("Failed to update PG", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "PG updated successfully"})
}

// GetRoomOccupancy - GET /v1/pg/:pg_id/rooms/occupancy
func (h *PGHandler) GetRoomOccupancy(c *gin.Context) {
	pgID, err := uuid.Parse(c.Param("pg_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid PG ID"})
		return
	}

	if !h.verifyPGAccess(c, pgID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	occupancy, err := h.roomService.GetRoomOccupancyDetails(c.Request.Context(), pgID)
	if err != nil {
		h.logger.Error("Failed to get occupancy", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, occupancy)
}

// GetPGStatistics - GET /v1/pg/:pg_id/statistics
func (h *PGHandler) GetPGStatistics(c *gin.Context) {
	pgID, err := uuid.Parse(c.Param("pg_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid PG ID"})
		return
	}

	if !h.verifyPGAccess(c, pgID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	stats, err := h.pgService.GetPGStatistics(c.Request.Context(), pgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// ListAllPGs - GET /v1/pg/list (Admin only)
func (h *PGHandler) ListAllPGs(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	pgs, err := h.pgService.GetAllPGsWithDetails(c.Request.Context(), limit, offset)
	if err != nil {
		h.logger.Error("Failed to list PGs", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"pgs":    pgs,
		"limit":  limit,
		"offset": offset,
	})
}

// ListAllAvailablePGs - GET /v1/pgs/available (Public - for tenants to see all PGs)
func (h *PGHandler) ListAllAvailablePGs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	pgs, err := h.pgService.GetAllPGs(c.Request.Context(), limit, offset)
	if err != nil {
		h.logger.Error("Failed to list available PGs", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"pgs":    pgs,
		"limit":  limit,
		"offset": offset,
	})
}

// Helper: verifyPGAccess checks if user has access to PG
func (h *PGHandler) verifyPGAccess(c *gin.Context, pgID uuid.UUID) bool {
	role, _ := c.Get("role")
	userID, _ := c.Get("userID")

	// Admin has access to all PGs
	if role == "admin" {
		return true
	}

	// Owner check
	if role == "pg_owner" {
		pg, err := h.pgService.GetPGByID(c.Request.Context(), pgID)
		if err != nil {
			return false
		}

		userUUID, err := uuid.Parse(userID.(string))
		if err != nil {
			return false
		}

		return pg.UserID == userUUID
	}

	return false
}
