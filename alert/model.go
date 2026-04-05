package alert

import (
	"time"

	"github.com/google/uuid"
)

// Alert represents an alert triggered when a vital reading breaches a threshold.
type Alert struct {
	ID                uuid.UUID  `json:"id" db:"id"`
	UserID            uuid.UUID  `json:"user_id" db:"user_id"`
	VitalType         string     `json:"vital_type" db:"vital_type"`
	Value             float64    `json:"value" db:"value"`
	ThresholdBreached string     `json:"threshold_breached" db:"threshold_breached"`
	ThresholdValue    float64    `json:"threshold_value" db:"threshold_value"`
	Severity          string     `json:"severity" db:"severity"`
	Acknowledged      bool       `json:"acknowledged" db:"acknowledged"`
	AcknowledgedAt    *time.Time `json:"acknowledged_at,omitempty" db:"acknowledged_at"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
}

// AlertThreshold defines the monitoring bounds for a vital type.
type AlertThreshold struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	VitalType string    `json:"vital_type" db:"vital_type"`
	LowValue  *float64  `json:"low_value,omitempty" db:"low_value"`
	HighValue *float64  `json:"high_value,omitempty" db:"high_value"`
	Enabled   bool      `json:"enabled" db:"enabled"`
}
