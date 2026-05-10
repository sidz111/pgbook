package services

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/sidz111/pgbook/internals/models"
	"github.com/sidz111/pgbook/internals/repositories"
)

type TenantService interface {
	// CRUD Operations
	CreateTenant(ctx context.Context, tenant *models.Tenant) error
	GetTenantByID(ctx context.Context, id uuid.UUID) (*models.Tenant, error)
	GetTenantByUserID(ctx context.Context, userID uuid.UUID) (*models.Tenant, error)
	GetTenantByDocumentURL(ctx context.Context, documentURL string) (*models.Tenant, error)
	GetTenantsByPG(ctx context.Context, pgID uuid.UUID) ([]models.Tenant, error)
	UpdateTenant(ctx context.Context, tenant *models.Tenant) error
	UpdateProfilePhoto(ctx context.Context, tenantID uuid.UUID, photoURL string) error
	ApproveTenant(ctx context.Context, tenantID uuid.UUID, roomID uuid.UUID) error

	// Notice Period & Exit Management
	InitiateNotice(ctx context.Context, tenantID uuid.UUID, noticePeriodDays int) error
	CancelNotice(ctx context.Context, tenantID uuid.UUID) error
	GetRemainingNoticeDays(ctx context.Context, tenantID uuid.UUID) (int, error)
	OffboardTenant(ctx context.Context, tenantID uuid.UUID) error
	GetTenantsOnNotice(ctx context.Context, pgID uuid.UUID) ([]models.Tenant, error)
	ProcessExpiredNotices(ctx context.Context) error

	// Analytics
	GetTenantStatus(ctx context.Context, tenantID uuid.UUID) (map[string]interface{}, error)
	GetPGTenantStats(ctx context.Context, pgID uuid.UUID) (map[string]interface{}, error)
	GetTenantHistory(ctx context.Context, pgID uuid.UUID) ([]models.Tenant, error)
}

type tenantService struct {
	tenantRepo repositories.TenantRepository
	roomRepo   repositories.RoomRepository
	pgRepo     repositories.PGRepository
	logger     *slog.Logger
}

func NewTenantService(
	tenantRepo repositories.TenantRepository,
	roomRepo repositories.RoomRepository,
	pgRepo repositories.PGRepository,
) TenantService {
	return &tenantService{
		tenantRepo: tenantRepo,
		roomRepo:   roomRepo,
		pgRepo:     pgRepo,
		logger:     slog.Default(),
	}
}

// CreateTenant creates a new tenant with room occupancy update
func (s *tenantService) CreateTenant(ctx context.Context, tenant *models.Tenant) error {
	// Validation
	if tenant.UserID == uuid.Nil {
		return errors.New("user_id is required")
	}
	if tenant.PGID == uuid.Nil {
		return errors.New("PG_id is required")
	}
	// For pending approval tenants, room assignment happens later
	if tenant.Status == "pending_approval" {
		// Verify PG exists
		_, err := s.pgRepo.GetPGByID(ctx, tenant.PGID)
		if err != nil {
			return errors.New("PG not found")
		}

		tenant.ID = uuid.New()
		tenant.IsActive = true
		tenant.IsOnNotice = false
		return s.tenantRepo.CreateTenant(ctx, tenant)
	}

	// For direct creation, room is required
	if tenant.RoomID == nil || *tenant.RoomID == uuid.Nil {
		return errors.New("room_id is required")
	}

	// Verify room exists and has capacity
	room, err := s.roomRepo.GetRoomByID(ctx, *tenant.RoomID)
	if err != nil {
		return errors.New("room not found")
	}

	if room.Occupied >= room.Capacity {
		return errors.New("room is at full capacity")
	}

	// Verify PG exists
	_, err = s.pgRepo.GetPGByID(ctx, tenant.PGID)
	if err != nil {
		return errors.New("PG not found")
	}

	tenant.ID = uuid.New()
	tenant.IsActive = true
	tenant.IsOnNotice = false

	if err := s.tenantRepo.CreateTenant(ctx, tenant); err != nil {
		s.logger.Error("Failed to create tenant", "error", err, "user_id", tenant.UserID)
		return errors.New("failed to create tenant")
	}

	s.logger.Info("Tenant created successfully", "tenant_id", tenant.ID, "pg_id", tenant.PGID)
	return nil
}

// GetTenantByID retrieves tenant details
func (s *tenantService) GetTenantByID(ctx context.Context, id uuid.UUID) (*models.Tenant, error) {
	if id == uuid.Nil {
		return nil, errors.New("invalid tenant ID")
	}

	tenant, err := s.tenantRepo.GetTenantByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get tenant", "error", err, "tenant_id", id)
		return nil, errors.New("tenant not found")
	}

	return tenant, nil
}

// GetTenantByUserID retrieves tenant for a specific user
func (s *tenantService) GetTenantByUserID(ctx context.Context, userID uuid.UUID) (*models.Tenant, error) {
	if userID == uuid.Nil {
		return nil, errors.New("invalid user ID")
	}

	tenant, err := s.tenantRepo.GetTenantByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get tenant by user", "error", err, "user_id", userID)
		return nil, errors.New("tenant not found for user")
	}

	return tenant, nil
}

// GetTenantsByPG retrieves all active tenants for a PG
func (s *tenantService) GetTenantsByPG(ctx context.Context, pgID uuid.UUID) ([]models.Tenant, error) {
	if pgID == uuid.Nil {
		return nil, errors.New("invalid PG ID")
	}

	tenants, err := s.tenantRepo.GetTenantsByPGID(ctx, pgID)
	if err != nil {
		s.logger.Error("Failed to get tenants", "error", err, "pg_id", pgID)
		return nil, errors.New("failed to fetch tenants")
	}

	return tenants, nil
}

// UpdateTenant updates tenant details
func (s *tenantService) UpdateTenant(ctx context.Context, tenant *models.Tenant) error {
	if tenant.ID == uuid.Nil {
		return errors.New("tenant ID is required")
	}

	// Retrieve existing tenant
	existingTenant, err := s.tenantRepo.GetTenantByID(ctx, tenant.ID)
	if err != nil {
		return errors.New("tenant not found")
	}

	// Preserve critical fields
	tenant.UserID = existingTenant.UserID
	tenant.PGID = existingTenant.PGID
	tenant.RoomID = existingTenant.RoomID
	tenant.JoiningDate = existingTenant.JoiningDate
	tenant.CreatedAt = existingTenant.CreatedAt

	if err := s.tenantRepo.UpdateTenant(ctx, tenant); err != nil {
		s.logger.Error("Failed to update tenant", "error", err, "tenant_id", tenant.ID)
		return errors.New("failed to update tenant")
	}

	s.logger.Info("Tenant updated successfully", "tenant_id", tenant.ID)
	return nil
}

// UpdateProfilePhoto updates tenant's profile photo
func (s *tenantService) UpdateProfilePhoto(ctx context.Context, tenantID uuid.UUID, photoURL string) error {
	if tenantID == uuid.Nil {
		return errors.New("invalid tenant ID")
	}
	if photoURL == "" {
		return errors.New("photo URL is required")
	}

	if err := s.tenantRepo.UpdateProfilePhoto(ctx, tenantID, photoURL); err != nil {
		s.logger.Error("Failed to update profile photo", "error", err, "tenant_id", tenantID)
		return errors.New("failed to update photo")
	}

	s.logger.Info("Tenant photo updated", "tenant_id", tenantID)
	return nil
}

// ApproveTenant approves a tenant and assigns a room
func (s *tenantService) ApproveTenant(ctx context.Context, tenantID uuid.UUID, roomID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return errors.New("invalid tenant ID")
	}
	if roomID == uuid.Nil {
		return errors.New("invalid room ID")
	}

	// Get tenant
	tenant, err := s.tenantRepo.GetTenantByID(ctx, tenantID)
	if err != nil {
		s.logger.Error("Failed to get tenant", "error", err, "tenant_id", tenantID)
		return errors.New("tenant not found")
	}

	if tenant.Status != "pending_approval" {
		return errors.New("tenant is not pending approval")
	}

	// Check if room is available
	room, err := s.roomRepo.GetRoomByID(ctx, roomID)
	if err != nil {
		s.logger.Error("Failed to get room", "error", err, "room_id", roomID)
		return errors.New("room not found")
	}

	if room.Occupied >= room.Capacity {
		return errors.New("room is at full capacity")
	}

	// Check room capacity with current tenants
	occupiedCount, err := s.tenantRepo.CountTenantsInRoom(ctx, roomID)
	if err != nil {
		s.logger.Error("Failed to count tenants in room", "error", err, "room_id", roomID)
		return errors.New("failed to check room capacity")
	}

	if occupiedCount >= int(room.Capacity) {
		return errors.New("room is at full capacity")
	}

	// Update tenant status and assign room
	tenant.Status = "active"
	tenant.RoomID = &roomID
	now := time.Now()
	tenant.ActualJoiningDate = &now

	if err := s.tenantRepo.UpdateTenant(ctx, tenant); err != nil {
		s.logger.Error("Failed to approve tenant", "error", err, "tenant_id", tenantID)
		return errors.New("failed to approve tenant")
	}

	// Update room occupancy
	if err := s.roomRepo.UpdateOccupancy(ctx, roomID, true); err != nil {
		s.logger.Warn("Failed to update room occupancy", "error", err, "room_id", roomID)
	}

	s.logger.Info("Tenant approved and room assigned", "tenant_id", tenantID, "room_id", roomID)
	return nil
}

// InitiateNotice initiates exit notice period
func (s *tenantService) InitiateNotice(ctx context.Context, tenantID uuid.UUID, noticePeriodDays int) error {
	if tenantID == uuid.Nil {
		return errors.New("invalid tenant ID")
	}

	tenant, err := s.tenantRepo.GetTenantByID(ctx, tenantID)
	if err != nil {
		return errors.New("tenant not found")
	}

	if !tenant.IsActive {
		return errors.New("cannot initiate notice for inactive tenant")
	}

	if tenant.IsOnNotice {
		return errors.New("tenant already on notice period")
	}

	// Validate notice period
	if noticePeriodDays <= 0 {
		noticePeriodDays = tenant.NoticePeriodDays
	}

	if noticePeriodDays > 180 {
		return errors.New("notice period cannot exceed 180 days")
	}

	// Calculate exit date
	exitDate := time.Now().AddDate(0, 0, noticePeriodDays)

	if err := s.tenantRepo.SetNoticePeriod(ctx, tenantID, exitDate); err != nil {
		s.logger.Error("Failed to initiate notice", "error", err, "tenant_id", tenantID)
		return errors.New("failed to initiate notice")
	}

	s.logger.Info("Notice initiated", "tenant_id", tenantID, "exit_date", exitDate)
	return nil
}

// CancelNotice cancels exit notice period
func (s *tenantService) CancelNotice(ctx context.Context, tenantID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return errors.New("invalid tenant ID")
	}

	tenant, err := s.tenantRepo.GetTenantByID(ctx, tenantID)
	if err != nil {
		return errors.New("tenant not found")
	}

	if !tenant.IsOnNotice {
		return errors.New("tenant is not on notice period")
	}

	if err := s.tenantRepo.CancelNotice(ctx, tenantID); err != nil {
		s.logger.Error("Failed to cancel notice", "error", err, "tenant_id", tenantID)
		return errors.New("failed to cancel notice")
	}

	s.logger.Info("Notice cancelled", "tenant_id", tenantID)
	return nil
}

func (s *tenantService) GetTenantByDocumentURL(ctx context.Context, documentURL string) (*models.Tenant, error) {
	if documentURL == "" {
		return nil, errors.New("document URL is required")
	}

	tenant, err := s.tenantRepo.GetTenantByDocumentURL(ctx, documentURL)
	if err != nil {
		s.logger.Error("Failed to find tenant by document URL", "error", err, "url", documentURL)
		return nil, errors.New("document owner not found")
	}

	return tenant, nil
}

// GetRemainingNoticeDays returns remaining days in notice period
func (s *tenantService) GetRemainingNoticeDays(ctx context.Context, tenantID uuid.UUID) (int, error) {
	if tenantID == uuid.Nil {
		return 0, errors.New("invalid tenant ID")
	}

	tenant, err := s.tenantRepo.GetTenantByID(ctx, tenantID)
	if err != nil {
		return 0, errors.New("tenant not found")
	}

	return tenant.GetRemainingDays(), nil
}

// OffboardTenant deactivates tenant and updates room occupancy
func (s *tenantService) OffboardTenant(ctx context.Context, tenantID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return errors.New("invalid tenant ID")
	}

	tenant, err := s.tenantRepo.GetTenantByID(ctx, tenantID)
	if err != nil {
		return errors.New("tenant not found")
	}

	if !tenant.IsActive {
		return errors.New("tenant is already inactive")
	}

	if err := s.tenantRepo.DeactivateTenant(ctx, tenantID); err != nil {
		s.logger.Error("Failed to offboard tenant", "error", err, "tenant_id", tenantID)
		return errors.New("failed to offboard tenant")
	}

	s.logger.Info("Tenant offboarded successfully", "tenant_id", tenantID)
	return nil
}

// GetTenantsOnNotice returns all tenants on notice period for a PG
func (s *tenantService) GetTenantsOnNotice(ctx context.Context, pgID uuid.UUID) ([]models.Tenant, error) {
	if pgID == uuid.Nil {
		return nil, errors.New("invalid PG ID")
	}

	tenants, err := s.tenantRepo.GetTenantsOnNotice(ctx, pgID)
	if err != nil {
		s.logger.Error("Failed to get tenants on notice", "error", err, "pg_id", pgID)
		return nil, errors.New("failed to fetch tenants on notice")
	}

	return tenants, nil
}

// ProcessExpiredNotices automatically offboards tenants whose notice period has expired
func (s *tenantService) ProcessExpiredNotices(ctx context.Context) error {
	if err := s.tenantRepo.ProcessExpiries(ctx); err != nil {
		s.logger.Error("Failed to process expired notices", "error", err)
		return errors.New("failed to process expired notices")
	}

	s.logger.Info("Expired notices processed successfully")
	return nil
}

// GetTenantStatus provides comprehensive tenant status information
func (s *tenantService) GetTenantStatus(ctx context.Context, tenantID uuid.UUID) (map[string]interface{}, error) {
	if tenantID == uuid.Nil {
		return nil, errors.New("invalid tenant ID")
	}

	tenant, err := s.tenantRepo.GetTenantByID(ctx, tenantID)
	if err != nil {
		return nil, errors.New("tenant not found")
	}

	status := map[string]interface{}{
		"tenant_id":          tenant.ID,
		"first_name":         tenant.FirstName,
		"last_name":          tenant.LastName,
		"is_active":          tenant.IsActive,
		"is_on_notice":       tenant.IsOnNotice,
		"joining_date":       tenant.JoiningDate,
		"exit_date":          tenant.ExitDate,
		"remaining_days":     tenant.GetRemainingDays(),
		"notice_period_days": tenant.NoticePeriodDays,
	}

	return status, nil
}

// GetPGTenantStats provides tenant statistics for a PG
func (s *tenantService) GetPGTenantStats(ctx context.Context, pgID uuid.UUID) (map[string]interface{}, error) {
	if pgID == uuid.Nil {
		return nil, errors.New("invalid PG ID")
	}

	activeTenants, err := s.tenantRepo.GetTenantsByPGID(ctx, pgID)
	if err != nil {
		return nil, errors.New("failed to fetch active tenants")
	}

	tenantsOnNotice, err := s.tenantRepo.GetTenantsOnNotice(ctx, pgID)
	if err != nil {
		return nil, errors.New("failed to fetch tenants on notice")
	}

	stats := map[string]interface{}{
		"total_active_tenants":   len(activeTenants),
		"tenants_on_notice":      len(tenantsOnNotice),
		"active_tenants_count":   len(activeTenants) - len(tenantsOnNotice),
		"notice_expiry_upcoming": s.getUpcomingExpiryCount(tenantsOnNotice),
	}

	return stats, nil
}

// GetTenantHistory retrieves all tenants (active and inactive) for a PG
func (s *tenantService) GetTenantHistory(ctx context.Context, pgID uuid.UUID) ([]models.Tenant, error) {
	if pgID == uuid.Nil {
		return nil, errors.New("invalid PG ID")
	}

	// Note: This should be implemented in the repository to fetch both active and inactive
	// For now, we only get active tenants
	tenants, err := s.tenantRepo.GetTenantsByPGID(ctx, pgID)
	if err != nil {
		return nil, errors.New("failed to fetch tenant history")
	}

	return tenants, nil
}

// Helper function to get count of tenants with notice expiring within 7 days
func (s *tenantService) getUpcomingExpiryCount(tenants []models.Tenant) int {
	count := 0
	sevenDaysFromNow := time.Now().AddDate(0, 0, 7)

	for _, tenant := range tenants {
		if tenant.ExitDate != nil && tenant.ExitDate.Before(sevenDaysFromNow) && tenant.ExitDate.After(time.Now()) {
			count++
		}
	}

	return count
}
