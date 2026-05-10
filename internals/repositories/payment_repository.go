package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/sidz111/pgbook/internals/models"
	"gorm.io/gorm"
)

type PaymentRepository interface {
	CreatePayment(ctx context.Context, payment *models.Payment) error
	GetPaymentByID(ctx context.Context, id uuid.UUID) (*models.Payment, error)
	GetPaymentsByTenantID(ctx context.Context, tenantID uuid.UUID) ([]models.Payment, error)
	GetPaymentsByPGID(ctx context.Context, pgID uuid.UUID) ([]models.Payment, error)

	// Verification & Status Update
	UpdateStatus(ctx context.Context, id uuid.UUID, status string, remarks string) error
	GetPendingPayments(ctx context.Context, pgID uuid.UUID) ([]models.Payment, error)
}

type paymentRepository struct {
	db *gorm.DB
}

func NewPaymentRepository(db *gorm.DB) PaymentRepository {
	return &paymentRepository{db: db}
}

func (r *paymentRepository) CreatePayment(ctx context.Context, payment *models.Payment) error {
	return r.db.WithContext(ctx).Create(payment).Error
}

func (r *paymentRepository) GetPaymentByID(ctx context.Context, id uuid.UUID) (*models.Payment, error) {
	var payment models.Payment
	err := r.db.WithContext(ctx).First(&payment, "id = ?", id).Error
	return &payment, err
}

func (r *paymentRepository) GetPaymentsByTenantID(ctx context.Context, tenantID uuid.UUID) ([]models.Payment, error) {
	var payments []models.Payment
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at desc").Find(&payments).Error
	return payments, err
}

// GetPaymentsByPGID: Owner can see all payments related to their PG, regardless of tenant, for management purposes.
func (r *paymentRepository) GetPaymentsByPGID(ctx context.Context, pgID uuid.UUID) ([]models.Payment, error) {
	var payments []models.Payment
	err := r.db.WithContext(ctx).Where("pg_id = ?", pgID).Order("created_at desc").Find(&payments).Error
	return payments, err
}

// UpdateStatus: Owner can 'Approve' or 'Reject'
func (r *paymentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, remarks string) error {
	return r.db.WithContext(ctx).Model(&models.Payment{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":  status,
			"remarks": remarks,
		}).Error
}

func (r *paymentRepository) GetPendingPayments(ctx context.Context, pgID uuid.UUID) ([]models.Payment, error) {
	var payments []models.Payment
	err := r.db.WithContext(ctx).Where("pg_id = ? AND status = ?", pgID, "pending").Find(&payments).Error
	return payments, err
}
