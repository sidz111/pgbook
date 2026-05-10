package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/sidz111/pgbook/internals/models"
	"gorm.io/gorm"
)

type TenantRepository interface {
	CreateTenant(ctx context.Context, tenant *models.Tenant) error
	GetTenantByID(ctx context.Context, id uuid.UUID) (*models.Tenant, error)
	GetTenantByUserID(ctx context.Context, userID uuid.UUID) (*models.Tenant, error)
	GetTenantsByPGID(ctx context.Context, pgID uuid.UUID) ([]models.Tenant, error)
	UpdateTenant(ctx context.Context, tenant *models.Tenant) error
	UpdateProfilePhoto(ctx context.Context, id uuid.UUID, photoURL string) error

	// Notice & Exit Logic
	SetNoticePeriod(ctx context.Context, id uuid.UUID, exitDate time.Time) error
	DeactivateTenant(ctx context.Context, id uuid.UUID) error
	GetTenantsOnNotice(ctx context.Context, pgID uuid.UUID) ([]models.Tenant, error)
	ProcessExpiries(ctx context.Context) error
	CancelNotice(ctx context.Context, id uuid.UUID) error
}

type tenantRepository struct {
	db *gorm.DB
}

func NewTenantRepository(db *gorm.DB) TenantRepository {
	return &tenantRepository{db: db}
}

func (r *tenantRepository) CreateTenant(ctx context.Context, tenant *models.Tenant) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Tenant Create
		if err := tx.Create(tenant).Error; err != nil {
			return err
		}

		// 2. Room table occupied count +1
		return tx.Model(&models.Room{}).Where("id = ?", tenant.RoomID).
			UpdateColumn("occupied", gorm.Expr("occupied + ?", 1)).Error
	})
}

func (r *tenantRepository) GetTenantByID(ctx context.Context, id uuid.UUID) (*models.Tenant, error) {
	var tenant models.Tenant
	err := r.db.WithContext(ctx).Preload("Payments").First(&tenant, "id = ?", id).Error
	return &tenant, err
}

// after login to see profile
func (r *tenantRepository) GetTenantByUserID(ctx context.Context, userID uuid.UUID) (*models.Tenant, error) {
	var tenant models.Tenant
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&tenant).Error
	return &tenant, err
}

func (r *tenantRepository) GetTenantsByPGID(ctx context.Context, pgID uuid.UUID) ([]models.Tenant, error) {
	var tenants []models.Tenant
	err := r.db.WithContext(ctx).Where("pg_id = ? AND is_active = ?", pgID, true).Find(&tenants).Error
	return tenants, err
}

func (r *tenantRepository) UpdateTenant(ctx context.Context, tenant *models.Tenant) error {
	return r.db.WithContext(ctx).Model(&models.Tenant{}).
		Where("id = ?", tenant.ID).
		Select("*").
		Omit("ID", "UserID", "PGID"). // dont change these critical fields
		Updates(tenant).Error
}

func (r *tenantRepository) UpdateProfilePhoto(ctx context.Context, id uuid.UUID, photoURL string) error {
	return r.db.WithContext(ctx).Model(&models.Tenant{}).Where("id = ?", id).Update("profile_picture_url", photoURL).Error
}

func (r *tenantRepository) SetNoticePeriod(ctx context.Context, id uuid.UUID, exitDate time.Time) error {
	return r.db.WithContext(ctx).Model(&models.Tenant{}).
		Where("id = ? AND is_active = ?", id, true).
		Updates(map[string]interface{}{
			"is_on_notice": true,
			"exit_date":    exitDate,
		}).Error
}

func (r *tenantRepository) DeactivateTenant(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var tenant models.Tenant
		if err := tx.First(&tenant, "id = ?", id).Error; err != nil {
			return err
		}

		// 1. Do Status false
		if err := tx.Model(&tenant).Update("is_active", false).Error; err != nil {
			return err
		}

		// 2. Room occupancy -1
		return tx.Model(&models.Room{}).Where("id = ?", tenant.RoomID).
			UpdateColumn("occupied", gorm.Expr("occupied - ?", 1)).Error
	})
}

func (r *tenantRepository) GetTenantsOnNotice(ctx context.Context, pgID uuid.UUID) ([]models.Tenant, error) {
	var tenants []models.Tenant
	err := r.db.WithContext(ctx).Where("pg_id = ? AND is_on_notice = ?", pgID, true).Find(&tenants).Error
	return tenants, err
}

func (r *tenantRepository) ProcessExpiries(ctx context.Context) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var expiredTenants []models.Tenant

		err := tx.Where("is_on_notice = ? AND exit_date <= ? AND is_active = ?", true, time.Now(), true).
			Find(&expiredTenants).Error
		if err != nil {
			return err
		}

		for _, tenant := range expiredTenants {
			// 2. Status inactive
			tx.Model(&tenant).Update("is_active", false)

			// 3. Room occupancy -1 (Automatic)
			tx.Model(&models.Room{}).Where("id = ?", tenant.RoomID).
				UpdateColumn("occupied", gorm.Expr("occupied - ?", 1))
		}
		return nil
	})
}

func (r *tenantRepository) CancelNotice(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&models.Tenant{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_on_notice": false,
			"exit_date":    nil,
		}).Error
}
