package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Subscription struct {
	gorm.Model
	ID         uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	PGID       uuid.UUID  `gorm:"type:char(36);index;not null" json:"pg_id"`
	PlanName   string     `gorm:"type:varchar(50);default:'Monthly'" json:"plan_name"`
	Amount     float64    `gorm:"type:decimal(10,2)" json:"amount"`
	ProofURL   string     `gorm:"type:varchar(255)" json:"proof_url"`
	Status     string     `gorm:"type:varchar(20);default:'pending'" json:"status"` // pending, active, rejected
	StartDate  *time.Time `gorm:"type:datetime;null" json:"start_date"`
	ExpiryDate *time.Time `gorm:"type:datetime;null" json:"expiry_date"`
	VerifiedAt *time.Time `gorm:"type:datetime;null" json:"verified_at"`
	VerifiedBy string     `json:"verified_by"` // Admin name/ID
}

func (s *Subscription) BeforeCreate(tx *gorm.DB) (err error) {
	s.ID = uuid.New()
	return
}
