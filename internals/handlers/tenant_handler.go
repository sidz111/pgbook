package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sidz111/pgbook/internals/models"
	"github.com/sidz111/pgbook/internals/services"
	"github.com/sidz111/pgbook/internals/utils"
)

type TenantHandler struct {
	tenantService     services.TenantService
	roomService       services.RoomService
	pgService         services.PGService
	fileUploadService *utils.FileUploadService
	logger            *slog.Logger
}

func NewTenantHandler(
	tenantService services.TenantService,
	roomService services.RoomService,
	pgService services.PGService,
	fileUploadService *utils.FileUploadService,
) *TenantHandler {
	return &TenantHandler{
		tenantService:     tenantService,
		roomService:       roomService,
		pgService:         pgService,
		fileUploadService: fileUploadService,
		logger:            slog.Default(),
	}
}

// CreateTenantRequest defines request structure
type CreateTenantRequest struct {
	UserID      string `json:"user_id"`
	FirstName   string `json:"first_name" binding:"required"`
	LastName    string `json:"last_name"`
	Phone       string `json:"phone" binding:"required"`
	RoomID      string `json:"room_id" binding:"required"`
	PGID        string `json:"pg_id" binding:"required"`
	JoiningDate string `json:"joining_date"`  // ISO 8601 format
	IDProofType string `json:"id_proof_type"` // Aadhaar, PAN, Passport, etc.
}

// TenantSelfRegisterRequest defines request structure for self-registration
type TenantSelfRegisterRequest struct {
	FirstName   string `json:"first_name" binding:"required"`
	LastName    string `json:"last_name"`
	Phone       string `json:"phone" binding:"required"`
	PGID        string `json:"pg_id" binding:"required"`
	JoiningDate string `json:"joining_date"`  // ISO 8601 format
	IDProofType string `json:"id_proof_type"` // Aadhaar, PAN, Passport, etc.
}

// SelfRegisterTenant - POST /v1/tenant/self-register
func (h *TenantHandler) SelfRegisterTenant(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req TenantSelfRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pgID, err := uuid.Parse(req.PGID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid PG ID"})
		return
	}

	userUUID, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	joiningDate := time.Now()
	if req.JoiningDate != "" {
		if parsedDate, err := time.Parse("2006-01-02", req.JoiningDate); err == nil {
			joiningDate = parsedDate
		}
	}

	tenant := &models.Tenant{
		UserID:           userUUID,
		FirstName:        req.FirstName,
		LastName:         req.LastName,
		Phone:            req.Phone,
		PGID:             pgID,
		JoiningDate:      joiningDate,
		IDProofType:      req.IDProofType,
		Status:           "pending_approval", // New status for approval
		NoticePeriodDays: 30,
	}

	if err := h.tenantService.CreateTenant(c.Request.Context(), tenant); err != nil {
		h.logger.Error("Failed to self-register tenant", "error", err, "user_id", userID)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":   "Tenant registration submitted for approval",
		"tenant_id": tenant.ID,
	})
}
func (h *TenantHandler) ApproveTenant(c *gin.Context) {
	pgID, err := uuid.Parse(c.Param("pg_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid PG ID"})
		return
	}

	tenantID, err := uuid.Parse(c.Param("tenant_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant ID"})
		return
	}

	if !verifyPGOwnerOrAdmin(c, h.pgService, pgID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	var req struct {
		RoomID string `json:"room_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	roomID, err := uuid.Parse(req.RoomID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid room ID"})
		return
	}

	if err := h.tenantService.ApproveTenant(c.Request.Context(), tenantID, roomID); err != nil {
		h.logger.Error("Failed to approve tenant", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tenant approved and room allocated"})
}

// CreateTenant - POST /v1/tenant/create
func (h *TenantHandler) CreateTenant(c *gin.Context) {
	var req CreateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	roomID, err := uuid.Parse(req.RoomID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid room ID"})
		return
	}

	pgID, err := uuid.Parse(req.PGID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid PG ID"})
		return
	}

	if !verifyPGOwnerOrAdmin(c, h.pgService, pgID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	room, err := h.roomService.GetRoomByID(c.Request.Context(), roomID)
	if err != nil || room.PGID != pgID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "room not found or doesn't belong to PG"})
		return
	}

	joiningDate := time.Now()
	if req.JoiningDate != "" {
		if parsedDate, err := time.Parse("2006-01-02", req.JoiningDate); err == nil {
			joiningDate = parsedDate
		}
	}

	userID := uuid.Nil
	role := ""
	if req.UserID != "" {
		userID, err = uuid.Parse(req.UserID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
			return
		}
	} else {
		currentUser, currentRole, roleErr := getAuthUser(c)
		if roleErr != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated request"})
			return
		}
		userID = currentUser
		role = currentRole
		if role != models.RoleTenant {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required for PG owner or admin tenant creation"})
			return
		}
	}

	tenant := &models.Tenant{
		UserID:           userID,
		FirstName:        req.FirstName,
		LastName:         req.LastName,
		Phone:            req.Phone,
		RoomID:           &roomID,
		PGID:             pgID,
		JoiningDate:      joiningDate,
		IDProofType:      req.IDProofType,
		NoticePeriodDays: 30,
	}

	if err := h.tenantService.CreateTenant(c.Request.Context(), tenant); err != nil {
		h.logger.Error("Failed to create tenant", "error", err, "pg_id", pgID)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":   "Tenant created successfully",
		"tenant_id": tenant.ID,
	})
}

// UploadIDProof - POST /v1/tenant/:tenant_id/upload-id-proof
// Upload ID proof document with security verification
func (h *TenantHandler) UploadIDProof(c *gin.Context) {
	tenantID, err := uuid.Parse(c.Param("tenant_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant ID"})
		return
	}

	tenant, err := h.tenantService.GetTenantByID(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		return
	}

	// Verify access: Tenant, PG Owner, or Admin
	if !verifyTenantOrPGAccess(c, tenant, h.pgService) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	// Get file from request
	file, err := c.FormFile("id_proof")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read file"})
		return
	}

	// Upload file
	filePath, err := h.fileUploadService.UploadTenantDocument(file, "id_proof")
	if err != nil {
		h.logger.Error("Failed to upload ID proof", "error", err, "tenant_id", tenantID)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update tenant with ID proof path (NOT the actual ID number)
	// Important: We only store the file reference, not the ID number itself
	tenant.IDProofURL = filePath
	if err := h.tenantService.UpdateTenant(c.Request.Context(), tenant); err != nil {
		h.logger.Error("Failed to update tenant with ID proof", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save ID proof reference"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "ID proof uploaded successfully",
		"file_path": filePath,
	})
}

// UploadProfilePhoto - POST /v1/tenant/:tenant_id/upload-photo
func (h *TenantHandler) UploadProfilePhoto(c *gin.Context) {
	tenantID, err := uuid.Parse(c.Param("tenant_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant ID"})
		return
	}

	tenant, err := h.tenantService.GetTenantByID(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		return
	}

	// Verify access
	if !verifyTenantOrPGAccess(c, tenant, h.pgService) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	// Get file from request
	file, err := c.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read file"})
		return
	}

	// Upload file
	filePath, err := h.fileUploadService.UploadTenantDocument(file, "photo")
	if err != nil {
		h.logger.Error("Failed to upload photo", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update tenant profile photo
	if err := h.tenantService.UpdateProfilePhoto(c.Request.Context(), tenantID, filePath); err != nil {
		h.logger.Error("Failed to update profile photo", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save photo"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Profile photo uploaded successfully",
		"file_path": filePath,
	})
}

// GetTenantDetails - GET /v1/tenant/:tenant_id
func (h *TenantHandler) GetTenantDetails(c *gin.Context) {
	tenantID, err := uuid.Parse(c.Param("tenant_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant ID"})
		return
	}

	tenant, err := h.tenantService.GetTenantByID(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		return
	}

	if !verifyTenantOrPGAccess(c, tenant, h.pgService) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	c.JSON(http.StatusOK, tenant)
}

// InitiateNotice - POST /v1/tenant/:tenant_id/notice
type InitiateNoticeRequest struct {
	NoticePeriodDays int `json:"notice_period_days" binding:"required"`
}

func (h *TenantHandler) InitiateNotice(c *gin.Context) {
	tenantID, err := uuid.Parse(c.Param("tenant_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant ID"})
		return
	}

	tenant, err := h.tenantService.GetTenantByID(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		return
	}

	// Only tenant or PG owner can initiate notice
	if !verifyTenantOrPGAccess(c, tenant, h.pgService) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	var req InitiateNoticeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.tenantService.InitiateNotice(c.Request.Context(), tenantID, req.NoticePeriodDays); err != nil {
		h.logger.Error("Failed to initiate notice", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Notice initiated successfully",
	})
}

// CancelNotice - POST /v1/tenant/:tenant_id/cancel-notice
func (h *TenantHandler) CancelNotice(c *gin.Context) {
	tenantID, err := uuid.Parse(c.Param("tenant_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant ID"})
		return
	}

	tenant, err := h.tenantService.GetTenantByID(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		return
	}

	if !verifyTenantOrPGAccess(c, tenant, h.pgService) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	if err := h.tenantService.CancelNotice(c.Request.Context(), tenantID); err != nil {
		h.logger.Error("Failed to cancel notice", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Notice cancelled successfully",
	})
}

// GetTenantStatus - GET /v1/tenant/:tenant_id/status
func (h *TenantHandler) GetTenantStatus(c *gin.Context) {
	tenantID, err := uuid.Parse(c.Param("tenant_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant ID"})
		return
	}

	status, err := h.tenantService.GetTenantStatus(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}

// ListTenants - GET /v1/pg/:pg_id/tenants
func (h *TenantHandler) ListTenants(c *gin.Context) {
	pgID, err := uuid.Parse(c.Param("pg_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid PG ID"})
		return
	}

	if !verifyPGOwnerOrAdmin(c, h.pgService, pgID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	tenants, err := h.tenantService.GetTenantsByPG(c.Request.Context(), pgID)
	if err != nil {
		h.logger.Error("Failed to list tenants", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tenants": tenants,
		"count":   len(tenants),
	})
}
