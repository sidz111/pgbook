package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/sidz111/pgbook/internals/models"
	"gorm.io/gorm"
)

type SubscriptionRepository interface {
	CreateSubscription(ctx context.Context, sub *models.Subscription) error
	GetSubscriptionByID(ctx context.Context, id uuid.UUID) (*models.Subscription, error)
	GetSubscriptionsByPGID(ctx context.Context, pgID uuid.UUID) ([]models.Subscription, error)

	// Admin Approval Logic
	ApproveSubscription(ctx context.Context, subID uuid.UUID, months int, adminName string) error
	RejectSubscription(ctx context.Context, subID uuid.UUID) error

	// Cron Jobs / Utility
	GetPendingSubscriptions(ctx context.Context) ([]models.Subscription, error)
	GetExpiredSubscriptions(ctx context.Context) ([]models.Subscription, error)
}

type subscriptionRepository struct {
	db *gorm.DB
}

func NewSubscriptionRepository(db *gorm.DB) SubscriptionRepository {
	return &subscriptionRepository{db: db}
}

func (r *subscriptionRepository) CreateSubscription(ctx context.Context, sub *models.Subscription) error {
	return r.db.WithContext(ctx).Create(sub).Error
}

func (r *subscriptionRepository) GetSubscriptionByID(ctx context.Context, id uuid.UUID) (*models.Subscription, error) {
	var sub models.Subscription
	err := r.db.WithContext(ctx).First(&sub, "id = ?", id).Error
	return &sub, err
}

func (r *subscriptionRepository) GetSubscriptionsByPGID(ctx context.Context, pgID uuid.UUID) ([]models.Subscription, error) {
	var subs []models.Subscription
	err := r.db.WithContext(ctx).Where("pg_id = ?", pgID).Order("created_at desc").Find(&subs).Error
	return subs, err
}

func (r *subscriptionRepository) ApproveSubscription(ctx context.Context, subID uuid.UUID, months int, adminName string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sub models.Subscription
		if err := tx.First(&sub, "id = ?", subID).Error; err != nil {
			return err
		}

		// Determine base date for extension from any active subscription for the PG
		now := time.Now()
		expiryBase := now

		var activeSub models.Subscription
		if err := tx.Where("pg_id = ? AND status = ?", sub.PGID, "active").Order("expiry_date desc").First(&activeSub).Error; err == nil && activeSub.ExpiryDate != nil && activeSub.ExpiryDate.After(now) {
			expiryBase = *activeSub.ExpiryDate
		}

		expiry := expiryBase.AddDate(0, months, 0)
		verifiedAt := now

		// 1. Subscription status update
		if err := tx.Model(&sub).Updates(map[string]interface{}{
			"status":      "active",
			"start_date":  now,
			"expiry_date": expiry,
			"verified_at": &verifiedAt,
			"verified_by": adminName,
		}).Error; err != nil {
			return err
		}

		// 2. PG table subscription flag active
		return tx.Model(&models.PG{}).Where("id = ?", sub.PGID).
			Update("is_subscribed", true).Error
	})
}

// RejectSubscription: If payment screenshot fake
func (r *subscriptionRepository) RejectSubscription(ctx context.Context, subID uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&models.Subscription{}).
		Where("id = ?", subID).
		Update("status", "rejected").Error
}

func (r *subscriptionRepository) GetPendingSubscriptions(ctx context.Context) ([]models.Subscription, error) {
	var subs []models.Subscription
	err := r.db.WithContext(ctx).Where("status = ?", "pending").Find(&subs).Error
	return subs, err
}

func (r *subscriptionRepository) GetExpiredSubscriptions(ctx context.Context) ([]models.Subscription, error) {
	var subs []models.Subscription
	err := r.db.WithContext(ctx).Where("expiry_date < ? AND status = ?", time.Now(), "active").Find(&subs).Error
	return subs, err
}
