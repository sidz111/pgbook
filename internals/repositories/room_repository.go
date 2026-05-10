package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/sidz111/pgbook/internals/models"
	"gorm.io/gorm"
)

type RoomRepository interface {
	CreateRoom(ctx context.Context, room *models.Room) error
	GetRoomByID(ctx context.Context, id uuid.UUID) (*models.Room, error)
	GetRoomsByPGID(ctx context.Context, pgID uuid.UUID) ([]models.Room, error)
	UpdateRoom(ctx context.Context, room *models.Room) error
	DeleteRoom(ctx context.Context, id uuid.UUID) error
	UpdateOccupancy(ctx context.Context, roomID uuid.UUID, increment bool) error
}

type roomRepository struct {
	db *gorm.DB
}

func NewRoomRepository(db *gorm.DB) RoomRepository {
	return &roomRepository{db: db}
}

func (r *roomRepository) CreateRoom(ctx context.Context, room *models.Room) error {
	return r.db.WithContext(ctx).Create(room).Error
}

func (r *roomRepository) GetRoomByID(ctx context.Context, id uuid.UUID) (*models.Room, error) {
	var room models.Room
	err := r.db.WithContext(ctx).Preload("Tenants").First(&room, "id = ?", id).Error
	return &room, err
}

func (r *roomRepository) GetRoomsByPGID(ctx context.Context, pgID uuid.UUID) ([]models.Room, error) {
	var rooms []models.Room
	err := r.db.WithContext(ctx).Where("pg_id = ?", pgID).Find(&rooms).Error
	return rooms, err
}

func (r *roomRepository) UpdateRoom(ctx context.Context, room *models.Room) error {
	return r.db.WithContext(ctx).Model(&models.Room{}).
		Where("id = ?", room.ID).
		Updates(room).Error
}

func (r *roomRepository) DeleteRoom(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.Room{}, "id = ?", id).Error
}

func (r *roomRepository) UpdateOccupancy(ctx context.Context, roomID uuid.UUID, increment bool) error {
	val := 1
	if !increment {
		val = -1
	}
	return r.db.WithContext(ctx).Model(&models.Room{}).
		Where("id = ?", roomID).
		UpdateColumn("occupied", gorm.Expr("occupied + ?", val)).Error
}
