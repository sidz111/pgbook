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

type PGService interface {
	// CRUD Operations
	CreatePG(ctx context.Context, pg *models.PG) error
	GetPGByID(ctx context.Context, id uuid.UUID) (*models.PG, error)
	GetPGByOwner(ctx context.Context, ownerID uuid.UUID) (*models.PG, error)
	UpdatePG(ctx context.Context, pg *models.PG) error
	DeletePG(ctx context.Context, id uuid.UUID) error
	UpdateOwnerPhoto(ctx context.Context, pgID uuid.UUID, photoURL string) error

	// PG Management
	GetAllPGs(ctx context.Context, limit int, offset int) ([]models.PG, error)
	GetPGStatistics(ctx context.Context, pgID uuid.UUID) (map[string]interface{}, error)
	GetDashboardData(ctx context.Context, pgID uuid.UUID) (map[string]interface{}, error)

	// Subscription Management
	ActivateTrial(ctx context.Context, pgID uuid.UUID) error
	CheckSubscriptionStatus(ctx context.Context, pgID uuid.UUID) (bool, error)
}

type pgService struct {
	pgRepo           repositories.PGRepository
	roomRepo         repositories.RoomRepository
	tenantRepo       repositories.TenantRepository
	subscriptionRepo repositories.SubscriptionRepository
	logger           *slog.Logger
}

func NewPGService(
	pgRepo repositories.PGRepository,
	roomRepo repositories.RoomRepository,
	tenantRepo repositories.TenantRepository,
	subscriptionRepo repositories.SubscriptionRepository,
) PGService {
	return &pgService{
		pgRepo:           pgRepo,
		roomRepo:         roomRepo,
		tenantRepo:       tenantRepo,
		subscriptionRepo: subscriptionRepo,
		logger:           slog.Default(),
	}
}

// CreatePG creates a new PG with basic validation
func (s *pgService) CreatePG(ctx context.Context, pg *models.PG) error {
	// Validation
	if pg.UserID == uuid.Nil {
		return errors.New("user_id is required")
	}
	if pg.Name == "" {
		return errors.New("PG name is required")
	}
	if pg.Phone == "" {
		return errors.New("phone number is required")
	}
	if pg.Address == "" {
		return errors.New("address is required")
	}

	// Check if owner already has a PG
	existingPG, err := s.pgRepo.GetPGByUserID(ctx, pg.UserID)
	if err == nil && existingPG != nil {
		return errors.New("owner already has a PG registered")
	}

	pg.ID = uuid.New()
	if err := s.pgRepo.CreatePG(ctx, pg); err != nil {
		s.logger.Error("Failed to create PG", "error", err, "user_id", pg.UserID)
		return errors.New("failed to create PG")
	}

	// Auto-activate 1-month free trial subscription
	if err := s.ActivateTrial(ctx, pg.ID); err != nil {
		s.logger.Error("Failed to activate trial", "error", err, "pg_id", pg.ID)
		// Don't fail the PG creation, just log the trial activation error
	}

	s.logger.Info("PG created successfully", "pg_id", pg.ID, "owner_id", pg.UserID)
	return nil
}

// GetPGByID retrieves PG with all relationships
func (s *pgService) GetPGByID(ctx context.Context, id uuid.UUID) (*models.PG, error) {
	if id == uuid.Nil {
		return nil, errors.New("invalid PG ID")
	}

	pg, err := s.pgRepo.GetPGByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get PG", "error", err, "pg_id", id)
		return nil, errors.New("PG not found")
	}

	return pg, nil
}

// GetPGByOwner retrieves PG for a specific owner
func (s *pgService) GetPGByOwner(ctx context.Context, ownerID uuid.UUID) (*models.PG, error) {
	if ownerID == uuid.Nil {
		return nil, errors.New("invalid owner ID")
	}

	pg, err := s.pgRepo.GetPGByUserID(ctx, ownerID)
	if err != nil {
		s.logger.Error("Failed to get PG by owner", "error", err, "owner_id", ownerID)
		return nil, errors.New("PG not found for owner")
	}

	return pg, nil
}

// UpdatePG updates PG details
func (s *pgService) UpdatePG(ctx context.Context, pg *models.PG) error {
	if pg.ID == uuid.Nil {
		return errors.New("PG ID is required")
	}

	// Retrieve existing PG to prevent changes to critical fields
	existingPG, err := s.pgRepo.GetPGByID(ctx, pg.ID)
	if err != nil {
		return errors.New("PG not found")
	}

	// Preserve critical fields
	pg.UserID = existingPG.UserID
	pg.CreatedAt = existingPG.CreatedAt

	if err := s.pgRepo.UpdatePG(ctx, pg); err != nil {
		s.logger.Error("Failed to update PG", "error", err, "pg_id", pg.ID)
		return errors.New("failed to update PG")
	}

	s.logger.Info("PG updated successfully", "pg_id", pg.ID)
	return nil
}

// DeletePG deletes a PG (soft delete)
func (s *pgService) DeletePG(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return errors.New("invalid PG ID")
	}

	// Check if PG has active tenants
	pg, err := s.pgRepo.GetPGByID(ctx, id)
	if err != nil {
		return errors.New("PG not found")
	}

	if len(pg.Tenants) > 0 {
		return errors.New("cannot delete PG with active tenants")
	}

	if err := s.pgRepo.DeletePG(ctx, id); err != nil {
		s.logger.Error("Failed to delete PG", "error", err, "pg_id", id)
		return errors.New("failed to delete PG")
	}

	s.logger.Info("PG deleted successfully", "pg_id", id)
	return nil
}

// UpdateOwnerPhoto updates owner's profile photo
func (s *pgService) UpdateOwnerPhoto(ctx context.Context, pgID uuid.UUID, photoURL string) error {
	if pgID == uuid.Nil {
		return errors.New("invalid PG ID")
	}
	if photoURL == "" {
		return errors.New("photo URL is required")
	}

	if err := s.pgRepo.UpdateOwnerPhoto(ctx, pgID, photoURL); err != nil {
		s.logger.Error("Failed to update owner photo", "error", err, "pg_id", pgID)
		return errors.New("failed to update photo")
	}

	s.logger.Info("Owner photo updated", "pg_id", pgID)
	return nil
}

// GetAllPGs retrieves all PGs with pagination
func (s *pgService) GetAllPGs(ctx context.Context, limit int, offset int) ([]models.PG, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	pgs, err := s.pgRepo.GetAllPGs(ctx, limit, offset)
	if err != nil {
		s.logger.Error("Failed to get all PGs", "error", err)
		return nil, errors.New("failed to fetch PGs")
	}

	return pgs, nil
}

// GetPGStatistics provides PG statistics
func (s *pgService) GetPGStatistics(ctx context.Context, pgID uuid.UUID) (map[string]interface{}, error) {
	if pgID == uuid.Nil {
		return nil, errors.New("invalid PG ID")
	}

	stats, err := s.pgRepo.GetPGStatistics(ctx, pgID)
	if err != nil {
		s.logger.Error("Failed to get PG statistics", "error", err, "pg_id", pgID)
		return nil, errors.New("failed to fetch statistics")
	}

	result := make(map[string]interface{})
	for k, v := range stats {
		result[k] = v
	}

	return result, nil
}

// GetDashboardData provides comprehensive dashboard data for PG owner
func (s *pgService) GetDashboardData(ctx context.Context, pgID uuid.UUID) (map[string]interface{}, error) {
	if pgID == uuid.Nil {
		return nil, errors.New("invalid PG ID")
	}

	pg, err := s.pgRepo.GetPGByID(ctx, pgID)
	if err != nil {
		return nil, errors.New("PG not found")
	}

	// Get statistics
	stats, err := s.pgRepo.GetPGStatistics(ctx, pgID)
	if err != nil {
		s.logger.Error("Failed to get statistics", "error", err)
		stats = make(map[string]int64)
	}

	dashboardData := map[string]interface{}{
		"pg_id":                 pgID,
		"pg_name":               pg.Name,
		"owner_name":            pg.OwnerName,
		"is_subscribed":         pg.IsSubscribed,
		"trial_ends_at":         pg.TrialEndsAt,
		"total_rooms":           stats["total_rooms"],
		"active_tenants":        stats["active_tenants"],
		"total_rooms_available": stats["total_rooms"] - stats["active_tenants"],
	}

	return dashboardData, nil
}

// ActivateTrial activates 30-day trial for new PG
func (s *pgService) ActivateTrial(ctx context.Context, pgID uuid.UUID) error {
	if pgID == uuid.Nil {
		return errors.New("invalid PG ID")
	}

	pg, err := s.pgRepo.GetPGByID(ctx, pgID)
	if err != nil {
		return errors.New("PG not found")
	}

	if pg.IsSubscribed {
		return errors.New("PG already has active subscription")
	}

	// Create 1-month free trial subscription
	now := time.Now()
	trialSubscription := &models.Subscription{
		ID:         uuid.New(),
		PGID:       pgID,
		PlanName:   "Free Trial",
		Amount:     0,
		ProofURL:   "",
		Status:     "active",
		StartDate:  now,
		ExpiryDate: now.AddDate(0, 1, 0), // 1 month from now
		VerifiedAt: &now,
		VerifiedBy: "system",
	}

	if err := s.subscriptionRepo.CreateSubscription(ctx, trialSubscription); err != nil {
		s.logger.Error("Failed to create trial subscription", "error", err, "pg_id", pgID)
		return errors.New("failed to activate trial")
	}

	// Mark PG as subscribed
	pg.IsSubscribed = true
	pg.TrialEndsAt = &trialSubscription.ExpiryDate
	if err := s.pgRepo.UpdatePG(ctx, pg); err != nil {
		s.logger.Error("Failed to update PG subscription status", "error", err, "pg_id", pgID)
	}

	s.logger.Info("Trial activated for PG", "pg_id", pgID)
	return nil
}

// CheckSubscriptionStatus checks if PG subscription is active
func (s *pgService) CheckSubscriptionStatus(ctx context.Context, pgID uuid.UUID) (bool, error) {
	if pgID == uuid.Nil {
		return false, errors.New("invalid PG ID")
	}

	pg, err := s.pgRepo.GetPGByID(ctx, pgID)
	if err != nil {
		return false, errors.New("PG not found")
	}

	return pg.IsSubscribed, nil
}
