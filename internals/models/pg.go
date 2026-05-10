package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PG struct {
	gorm.Model
	ID            uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	UserID        uuid.UUID `gorm:"type:char(36);index;not null" json:"user_id"` // User Table Link
	Name          string    `gorm:"type:varchar(100);not null" json:"name"`
	OwnerName     string    `gorm:"type:varchar(100)" json:"owner_name"`
	OwnerPhotoURL string    `gorm:"type:varchar(255)" json:"owner_photo_url"`
	Phone         string    `gorm:"type:varchar(15)" json:"phone"`
	Address       string    `gorm:"type:text" json:"address"`
	ScannerURL    string    `gorm:"type:varchar(255)" json:"scanner_url"`

	// Subscription Status
	IsSubscribed bool       `gorm:"type:tinyint(1);default:0" json:"is_subscribed"`
	TrialEndsAt  *time.Time `json:"trial_ends_at"`

	// Relations
	Rooms         []Room         `gorm:"foreignKey:PGID" json:"rooms"`
	Tenants       []Tenant       `gorm:"foreignKey:PGID" json:"tenants"`
	Subscriptions []Subscription `gorm:"foreignKey:PGID" json:"subscriptions"`
}

func (p *PG) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return
}
