package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/sidz111/pgbook/internals/models"
	"gorm.io/gorm"
)

type UserRepository interface {
	CreateUser(ctx context.Context, user *models.User) error
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	UpdatePassword(ctx context.Context, id uuid.UUID, hashedPwd string) error
	EmailExists(ctx context.Context, email string) bool
	UpdateUser(ctx context.Context, user *models.User) error
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) CreateUser(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	return &user, err
}

func (r *userRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error
	return &user, err
}

func (r *userRepository) UpdatePassword(ctx context.Context, id uuid.UUID, hashedPwd string) error {
	return r.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).Update("password", hashedPwd).Error
}

func (r *userRepository) EmailExists(ctx context.Context, email string) bool {
	var count int64
	r.db.WithContext(ctx).Model(&models.User{}).Where("email = ?", email).Count(&count)
	return count > 0
}

func (r *userRepository) UpdateUser(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}
