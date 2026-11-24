package infra

import (
	"time"
)

type UserModel struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Email        string    `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string    `gorm:"not null" json:"-"`
	Role         string    `gorm:"not null;index" json:"role"`
	CreatedAt    time.Time `json:"createdAt"`
}

type ScanModel struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	UserID          uint      `gorm:"not null;index:idx_scans_user_id" json:"userId"`
	URL             string    `gorm:"not null" json:"url"`
	NormalizedURL   string    `gorm:"index" json:"normalizedUrl"`
	Status          string    `gorm:"index;not null" json:"status"`
	ProcessingToken string    `gorm:"size:32;index" json:"-"`
	RiskLevel       string    `gorm:"index" json:"riskLevel"`
	Score           int       `json:"score"`
	Error           string    `json:"error,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`

	User       UserModel        `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Indicators []IndicatorModel `gorm:"foreignKey:ScanID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"indicators"`
}

type IndicatorModel struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	ScanID      uint   `gorm:"not null;index:idx_indicators_scan_id" json:"scanId"`
	Code        string `gorm:"not null" json:"code"`
	Description string `gorm:"not null" json:"description"`
	Weight      int    `json:"weight"`

	Scan ScanModel `gorm:"foreignKey:ScanID" json:"-"`
}

type AuditLogModel struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"userId"`
	Action    string    `gorm:"not null" json:"action"`
	Entity    string    `gorm:"not null" json:"entity"`
	EntityID  uint      `gorm:"not null" json:"entityId"`
	CreatedAt time.Time `gorm:"not null" json:"createdAt"`
}
