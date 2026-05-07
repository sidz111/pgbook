package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Room struct {
	gorm.Model
	ID         uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	PGID       uuid.UUID `gorm:"type:char(36);index;not null" json:"pg_id"`
	RoomNumber string    `gorm:"type:varchar(20);not null" json:"room_number"` // e.g. A-1
	Capacity   int       `gorm:"not null" json:"capacity"`
	Occupied   int       `gorm:"default:0" json:"occupied"`
	RentAmount float64   `gorm:"type:decimal(10,2)" json:"rent_amount"`

	Tenants []Tenant `gorm:"foreignKey:RoomID" json:"tenants"`
}

func (r *Room) BeforeCreate(tx *gorm.DB) (err error) {
	r.ID = uuid.New()
	return
}
