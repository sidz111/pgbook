package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	RoleAdmin  = "admin"
	RoleOwner  = "pg_owner"
	RoleTenant = "tenant"
)

type User struct {
	gorm.Model
	ID           uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	Email        string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"email"`
	Password     string    `gorm:"type:varchar(255);not null" json:"-"`
	Role         string    `gorm:"type:varchar(20);default:'tenant';not null" json:"role"`
	IsActive     bool      `gorm:"type:tinyint(1);default:1" json:"is_active"`
	RefreshToken string    `json:"-"`
}

func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return
}
