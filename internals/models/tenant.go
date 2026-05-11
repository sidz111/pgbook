package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Tenant struct {
	gorm.Model
	ID                uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	UserID            uuid.UUID  `gorm:"type:char(36);index;not null" json:"-"`
	PGID              uuid.UUID  `gorm:"type:char(36);index;not null" json:"pg_id"`
	RoomID            *uuid.UUID `gorm:"type:char(36);index" json:"room_id"`
	FirstName         string     `gorm:"type:varchar(50);not null" json:"first_name"`
	LastName          string     `gorm:"type:varchar(50)" json:"last_name"`
	Phone             string     `gorm:"type:varchar(15);not null" json:"phone"`
	ProfilePictureURL string     `gorm:"type:varchar(255)" json:"profile_picture_url"`
	IDProofURL        string     `gorm:"type:varchar(255)" json:"id_proof_url"`
	IDProofType       string     `gorm:"type:varchar(30)" json:"id_proof_type"`
	JoiningDate       time.Time  `json:"joining_date"`
	ActualJoiningDate *time.Time `json:"actual_joining_date"`
	Status            string     `gorm:"type:varchar(20);default:'pending_approval'" json:"status"` // pending_approval, active, inactive
	IsActive          bool       `gorm:"type:tinyint(1);default:1" json:"is_active"`
	IsOnNotice        bool       `gorm:"type:tinyint(1);default:0" json:"is_on_notice"`
	NoticePeriodDays  int        `gorm:"default:30" json:"notice_period_days"`
	ExitDate          *time.Time `json:"exit_date"`

	Payments []Payment `gorm:"foreignKey:TenantID" json:"payments,omitempty"`
}

func (t *Tenant) GetRemainingDays() int {
	if !t.IsActive || !t.IsOnNotice || t.ExitDate == nil {
		return 0
	}

	duration := time.Until(*t.ExitDate)
	days := int(duration.Hours() / 24)

	if days < 0 {
		return 0
	}
	return days
}

func (t *Tenant) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return
}
