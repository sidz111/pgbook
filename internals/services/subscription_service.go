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

type SubscriptionService interface {
	// CRUD Operations
	CreateSubscription(ctx context.Context, subscription *models.Subscription) error
	GetSubscriptionByID(ctx context.Context, id uuid.UUID) (*models.Subscription, error)
	GetSubscriptionsByPG(ctx context.Context, pgID uuid.UUID) ([]models.Subscription, error)

	// Admin Operations
	ApproveSubscription(ctx context.Context, subID uuid.UUID, months int, adminName string) error
	RejectSubscription(ctx context.Context, subID uuid.UUID) error

	// Subscription Management
	GetActiveSubscription(ctx context.Context, pgID uuid.UUID) (*models.Subscription, error)
	IsSubscriptionActive(ctx context.Context, pgID uuid.UUID) (bool, error)
	GetSubscriptionStatus(ctx context.Context, subID uuid.UUID) (string, error)
	RenewSubscription(ctx context.Context, pgID uuid.UUID, months int, adminName string) error

	// Cron/Utility Operations
	GetPendingSubscriptions(ctx context.Context) ([]models.Subscription, error)
	GetExpiredSubscriptions(ctx context.Context) ([]models.Subscription, error)
	ProcessExpiredSubscriptions(ctx context.Context) error
	GetSubscriptionExpiringSoon(ctx context.Context, daysThreshold int) ([]models.Subscription, error)

	// Analytics
	GetSubscriptionStats(ctx context.Context) (map[string]interface{}, error)
}

type subscriptionService struct {
	subscriptionRepo repositories.SubscriptionRepository
	pgRepo           repositories.PGRepository
	logger           *slog.Logger
}

func NewSubscriptionService(
	subscriptionRepo repositories.SubscriptionRepository,
	pgRepo repositories.PGRepository,
) SubscriptionService {
	return &subscriptionService{
		subscriptionRepo: subscriptionRepo,
		pgRepo:           pgRepo,
		logger:           slog.Default(),
	}
}

// CreateSubscription creates a new subscription request
func (s *subscriptionService) CreateSubscription(ctx context.Context, subscription *models.Subscription) error {
	// Validation
	if subscription.PGID == uuid.Nil {
		return errors.New("PG_id is required")
	}
	if subscription.Amount <= 0 {
		return errors.New("subscription amount must be greater than 0")
	}
	if subscription.ProofURL == "" {
		return errors.New("proof URL (payment screenshot) is required")
	}

	// Verify PG exists
	_, err := s.pgRepo.GetPGByID(ctx, subscription.PGID)
	if err != nil {
		return errors.New("PG not found")
	}

	subscription.ID = uuid.New()
	subscription.Status = "pending"
	subscription.StartDate = nil
	subscription.ExpiryDate = nil

	// Set default plan name if not provided
	if subscription.PlanName == "" {
		subscription.PlanName = "Monthly"
	}

	if err := s.subscriptionRepo.CreateSubscription(ctx, subscription); err != nil {
		s.logger.Error("Failed to create subscription", "error", err, "pg_id", subscription.PGID)
		return errors.New("failed to create subscription")
	}

	s.logger.Info("Subscription created", "subscription_id", subscription.ID, "pg_id", subscription.PGID, "status", "pending")
	return nil
}

// GetSubscriptionByID retrieves subscription details
func (s *subscriptionService) GetSubscriptionByID(ctx context.Context, id uuid.UUID) (*models.Subscription, error) {
	if id == uuid.Nil {
		return nil, errors.New("invalid subscription ID")
	}

	subscription, err := s.subscriptionRepo.GetSubscriptionByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get subscription", "error", err, "subscription_id", id)
		return nil, errors.New("subscription not found")
	}

	return subscription, nil
}

// GetSubscriptionsByPG retrieves all subscriptions for a PG
func (s *subscriptionService) GetSubscriptionsByPG(ctx context.Context, pgID uuid.UUID) ([]models.Subscription, error) {
	if pgID == uuid.Nil {
		return nil, errors.New("invalid PG ID")
	}

	subscriptions, err := s.subscriptionRepo.GetSubscriptionsByPGID(ctx, pgID)
	if err != nil {
		s.logger.Error("Failed to get subscriptions", "error", err, "pg_id", pgID)
		return nil, errors.New("failed to fetch subscriptions")
	}

	return subscriptions, nil
}

// ApproveSubscription approves a pending subscription
func (s *subscriptionService) ApproveSubscription(ctx context.Context, subID uuid.UUID, months int, adminName string) error {
	if subID == uuid.Nil {
		return errors.New("invalid subscription ID")
	}
	if months <= 0 || months > 36 {
		return errors.New("subscription duration must be between 1 and 36 months")
	}
	if adminName == "" {
		return errors.New("admin name is required")
	}

	// Verify subscription exists and is pending
	subscription, err := s.subscriptionRepo.GetSubscriptionByID(ctx, subID)
	if err != nil {
		return errors.New("subscription not found")
	}

	if subscription.Status != "pending" {
		return errors.New("only pending subscriptions can be approved")
	}

	if err := s.subscriptionRepo.ApproveSubscription(ctx, subID, months, adminName); err != nil {
		s.logger.Error("Failed to approve subscription", "error", err, "subscription_id", subID)
		return errors.New("failed to approve subscription")
	}

	s.logger.Info("Subscription approved", "subscription_id", subID, "months", months, "admin", adminName)
	return nil
}

// RejectSubscription rejects a subscription request
func (s *subscriptionService) RejectSubscription(ctx context.Context, subID uuid.UUID) error {
	if subID == uuid.Nil {
		return errors.New("invalid subscription ID")
	}

	subscription, err := s.subscriptionRepo.GetSubscriptionByID(ctx, subID)
	if err != nil {
		return errors.New("subscription not found")
	}

	if subscription.Status != "pending" {
		return errors.New("only pending subscriptions can be rejected")
	}

	if err := s.subscriptionRepo.RejectSubscription(ctx, subID); err != nil {
		s.logger.Error("Failed to reject subscription", "error", err, "subscription_id", subID)
		return errors.New("failed to reject subscription")
	}

	s.logger.Info("Subscription rejected", "subscription_id", subID)
	return nil
}

// GetActiveSubscription retrieves current active subscription for a PG
func (s *subscriptionService) GetActiveSubscription(ctx context.Context, pgID uuid.UUID) (*models.Subscription, error) {
	if pgID == uuid.Nil {
		return nil, errors.New("invalid PG ID")
	}

	subscriptions, err := s.subscriptionRepo.GetSubscriptionsByPGID(ctx, pgID)
	if err != nil {
		return nil, errors.New("failed to fetch subscriptions")
	}

	now := time.Now()
	for _, sub := range subscriptions {
		if sub.Status == "active" && sub.ExpiryDate != nil && sub.ExpiryDate.After(now) {
			return &sub, nil
		}
	}

	return nil, errors.New("no active subscription found")
}

// IsSubscriptionActive checks if PG has active subscription
func (s *subscriptionService) IsSubscriptionActive(ctx context.Context, pgID uuid.UUID) (bool, error) {
	if pgID == uuid.Nil {
		return false, errors.New("invalid PG ID")
	}

	_, err := s.GetActiveSubscription(ctx, pgID)
	if err != nil {
		return false, nil
	}

	return true, nil
}

// GetSubscriptionStatus returns subscription status
func (s *subscriptionService) GetSubscriptionStatus(ctx context.Context, subID uuid.UUID) (string, error) {
	if subID == uuid.Nil {
		return "", errors.New("invalid subscription ID")
	}

	subscription, err := s.subscriptionRepo.GetSubscriptionByID(ctx, subID)
	if err != nil {
		return "", errors.New("subscription not found")
	}

	if subscription.Status == "active" && subscription.ExpiryDate != nil && subscription.ExpiryDate.Before(time.Now()) {
		return "expired", nil
	}

	return subscription.Status, nil
}

// RenewSubscription renews subscription for a PG
func (s *subscriptionService) RenewSubscription(ctx context.Context, pgID uuid.UUID, months int, adminName string) error {
	if pgID == uuid.Nil {
		return errors.New("invalid PG ID")
	}
	if months <= 0 || months > 36 {
		return errors.New("subscription duration must be between 1 and 36 months")
	}

	// Get current active subscription
	currentSub, err := s.GetActiveSubscription(ctx, pgID)
	if err != nil {
		return errors.New("no active subscription to renew")
	}

	if currentSub.ExpiryDate == nil {
		return errors.New("current subscription expiry date is missing")
	}

	// Create new subscription record
	newSub := &models.Subscription{
		PGID:      pgID,
		PlanName:  currentSub.PlanName,
		Amount:    currentSub.Amount,
		Status:    "active",
		StartDate: currentSub.ExpiryDate,
	}

	nextExpiry := currentSub.ExpiryDate.AddDate(0, months, 0)
	newSub.ExpiryDate = &nextExpiry
	now := time.Now()
	newSub.VerifiedAt = &now
	newSub.VerifiedBy = adminName

	if err := s.subscriptionRepo.CreateSubscription(ctx, newSub); err != nil {
		s.logger.Error("Failed to renew subscription", "error", err, "pg_id", pgID)
		return errors.New("failed to renew subscription")
	}

	s.logger.Info("Subscription renewed", "pg_id", pgID, "months", months, "new_expiry", newSub.ExpiryDate)
	return nil
}

// GetPendingSubscriptions retrieves all pending subscriptions
func (s *subscriptionService) GetPendingSubscriptions(ctx context.Context) ([]models.Subscription, error) {
	subscriptions, err := s.subscriptionRepo.GetPendingSubscriptions(ctx)
	if err != nil {
		s.logger.Error("Failed to get pending subscriptions", "error", err)
		return nil, errors.New("failed to fetch pending subscriptions")
	}

	return subscriptions, nil
}

// GetExpiredSubscriptions retrieves all expired active subscriptions
func (s *subscriptionService) GetExpiredSubscriptions(ctx context.Context) ([]models.Subscription, error) {
	subscriptions, err := s.subscriptionRepo.GetExpiredSubscriptions(ctx)
	if err != nil {
		s.logger.Error("Failed to get expired subscriptions", "error", err)
		return nil, errors.New("failed to fetch expired subscriptions")
	}

	return subscriptions, nil
}

// ProcessExpiredSubscriptions deactivates expired subscriptions
func (s *subscriptionService) ProcessExpiredSubscriptions(ctx context.Context) error {
	subscriptions, err := s.subscriptionRepo.GetExpiredSubscriptions(ctx)
	if err != nil {
		return errors.New("failed to fetch expired subscriptions")
	}

	for _, sub := range subscriptions {
		// Update PG to mark as unsubscribed
		pg, err := s.pgRepo.GetPGByID(ctx, sub.PGID)
		if err != nil {
			s.logger.Error("Failed to get PG for subscription expiry", "pg_id", sub.PGID)
			continue
		}

		pg.IsSubscribed = false
		s.pgRepo.UpdatePG(ctx, pg)

		s.logger.Info("Subscription marked as expired", "subscription_id", sub.ID, "pg_id", sub.PGID)
	}

	return nil
}

// GetSubscriptionExpiringSoon retrieves subscriptions expiring within threshold days
func (s *subscriptionService) GetSubscriptionExpiringSoon(ctx context.Context, daysThreshold int) ([]models.Subscription, error) {
	if daysThreshold <= 0 {
		daysThreshold = 7 // Default 7 days
	}

	// Get all active subscriptions
	allSubscriptions, err := s.subscriptionRepo.GetPendingSubscriptions(ctx)
	if err != nil {
		return nil, errors.New("failed to fetch subscriptions")
	}

	expiringSubscriptions := make([]models.Subscription, 0)
	thresholdDate := time.Now().AddDate(0, 0, daysThreshold)

	for _, sub := range allSubscriptions {
		if sub.Status == "active" &&
			sub.ExpiryDate.Before(thresholdDate) &&
			sub.ExpiryDate.After(time.Now()) {
			expiringSubscriptions = append(expiringSubscriptions, sub)
		}
	}

	return expiringSubscriptions, nil
}

// GetSubscriptionStats provides subscription statistics
func (s *subscriptionService) GetSubscriptionStats(ctx context.Context) (map[string]interface{}, error) {
	pending, _ := s.subscriptionRepo.GetPendingSubscriptions(ctx)
	active, _ := s.subscriptionRepo.GetExpiredSubscriptions(ctx)
	expired, _ := s.subscriptionRepo.GetExpiredSubscriptions(ctx)

	stats := map[string]interface{}{
		"total_pending": len(pending),
		"total_active":  len(active),
		"total_expired": len(expired),
		"approval_rate": calculateApprovalRate(len(active), len(pending)+len(active)),
	}

	return stats, nil
}

// Helper function to calculate approval rate
func calculateApprovalRate(active, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(active) / float64(total) * 100
}
