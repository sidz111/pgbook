package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PG struct {
	gorm.Model
	ID        uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	Name      string    `gorm:"type:varchar(100);not null" json:"name"`
	OwnerName string    `gorm:"type:varchar(100)" json:"owner_name"`
	Email     string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"not null" json:"-"`
	Address   string    `gorm:"type:text" json:"address"`

	Rooms   []Room   `gorm:"foreignKey:PGID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"rooms"`
	Tenants []Tenant `gorm:"foreignKey:PGID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"tenants"`
}

func (p *PG) BeforeCreate(tx *gorm.DB) (err error) {
	p.ID = uuid.New()
	return
}
