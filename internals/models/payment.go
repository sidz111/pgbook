package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Payment struct {
	gorm.Model
	ID          uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	PGID        uuid.UUID `gorm:"type:char(36);index;not null" json:"pg_id"`
	TenantID    uuid.UUID `gorm:"type:char(36);index;not null" json:"tenant_id"`
	Amount      float64   `gorm:"type:decimal(10,2);not null" json:"amount"`
	PaymentDate time.Time `json:"payment_date"`
	Month       string    `gorm:"type:varchar(20)" json:"month"`
	Status      string    `gorm:"type:varchar(20);default:'paid'" json:"status"`
	TxnID       string    `gorm:"type:varchar(100)" json:"txn_id"`
}

func (p *Payment) BeforeCreate(tx *gorm.DB) (err error) {
	p.ID = uuid.New()
	return
}
