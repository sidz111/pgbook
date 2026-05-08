package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/sidz111/pgbook/internals/models"
	"gorm.io/gorm"
)

type PGRepository interface {
	CreatePG(ctx context.Context, pg *models.PG) error
	GetPGByEmail(ctx context.Context, email string) (*models.PG, error)
	GetPGByID(ctx context.Context, id uuid.UUID) (*models.PG, error)
	UpdatePG(ctx context.Context, pg *models.PG) error
	UpdatePassword(ctx context.Context, id uuid.UUID, newPassword string) error
	DeletePG(ctx context.Context, id uuid.UUID) error
	GetPGStatistics(ctx context.Context, id uuid.UUID) (map[string]int64, error)
	GetAllPGs(ctx context.Context, limit int, offset int) ([]models.PG, error)
	EmailExists(ctx context.Context, email string) bool
}

type pgRepository struct {
	db *gorm.DB
}

func NewPGRepository(db *gorm.DB) PGRepository {
	return &pgRepository{db: db}
}

func (r *pgRepository) CreatePG(ctx context.Context, pg *models.PG) error {
	return r.db.WithContext(ctx).Create(pg).Error
}

func (r *pgRepository) GetPGByEmail(ctx context.Context, email string) (*models.PG, error) {
	var pg models.PG
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&pg).Error
	if err != nil {
		return nil, err
	}
	return &pg, nil
}

func (r *pgRepository) GetPGByID(ctx context.Context, id uuid.UUID) (*models.PG, error) {
	var pg models.PG
	err := r.db.WithContext(ctx).Preload("Rooms").Preload("Tenants").First(&pg, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &pg, nil
}

func (r *pgRepository) UpdatePG(ctx context.Context, pg *models.PG) error {
	return r.db.WithContext(ctx).Model(&models.PG{}).Where("id = ?", pg.ID).Updates(pg).Error
}

func (r *pgRepository) UpdatePassword(ctx context.Context, id uuid.UUID, newPassword string) error {
	return r.db.WithContext(ctx).Model(&models.PG{}).Where("id = ?", id).Update("password", newPassword).Error
}

func (r *pgRepository) DeletePG(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.PG{}, "id = ?", id).Error
}

func (r *pgRepository) GetPGStatistics(ctx context.Context, id uuid.UUID) (map[string]int64, error) {
	var roomCount int64
	var tenantCount int64

	r.db.WithContext(ctx).Model(&models.Room{}).Where("pg_id = ?", id).Count(&roomCount)
	r.db.WithContext(ctx).Model(&models.Tenant{}).Where("pg_id = ? AND is_active = ?", id, true).Count(&tenantCount)

	stats := map[string]int64{
		"total_rooms":    roomCount,
		"active_tenants": tenantCount,
	}
	return stats, nil
}

func (r *pgRepository) GetAllPGs(ctx context.Context, limit int, offset int) ([]models.PG, error) {
	var pgs []models.PG
	err := r.db.WithContext(ctx).Limit(limit).Offset(offset).Find(&pgs).Error
	return pgs, err
}

func (r *pgRepository) EmailExists(ctx context.Context, email string) bool {
	var count int64
	r.db.WithContext(ctx).Model(&models.PG{}).Where("email = ?", email).Count(&count)
	return count > 0
}
