package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Payment struct {
	gorm.Model
	ID            uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	PGID          uuid.UUID `gorm:"type:char(36);index;not null" json:"pg_id"`
	TenantID      uuid.UUID `gorm:"type:char(36);index;not null" json:"tenant_id"`
	Amount        float64   `gorm:"type:decimal(10,2);not null" json:"amount"`
	PaymentDate   time.Time `json:"payment_date"`
	ForMonth      string    `gorm:"type:varchar(20)" json:"for_month"`
	Status        string    `gorm:"type:varchar(20);default:'pending'" json:"status"`
	Method        string    `gorm:"type:varchar(20)" json:"method"` // Cash, GPay, QR
	TransactionID string    `gorm:"type:varchar(100)" json:"transaction_id"`
	ProofURL      string    `gorm:"type:varchar(255)" json:"proof_url"` // Screenshot URL
	Remarks       string    `gorm:"type:varchar(255)" json:"remarks"`
}

func (p *Payment) BeforeCreate(tx *gorm.DB) (err error) {
	p.ID = uuid.New()
	return
}
