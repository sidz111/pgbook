package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Tenant struct {
	gorm.Model
	ID          uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	PGID        uuid.UUID `gorm:"type:char(36);index;not null" json:"pg_id"`
	RoomID      uuid.UUID `gorm:"type:char(36);index;not null" json:"room_id"`
	FirstName   string    `gorm:"type:varchar(50);not null" json:"first_name"`
	LastName    string    `gorm:"type:varchar(50)" json:"last_name"`
	Email       string    `gorm:"type:varchar(100)" json:"email"`
	Phone       string    `gorm:"type:varchar(15);not null" json:"phone"`
	JoiningDate time.Time `json:"joining_date"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`

	Payments []Payment `gorm:"foreignKey:TenantID" json:"payments"`
}

func (t *Tenant) BeforeCreate(tx *gorm.DB) (err error) {
	t.ID = uuid.New()
	return
}
