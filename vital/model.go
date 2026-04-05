package vital

import (
	"time"

	"github.com/google/uuid"
)

// VitalReading represents a single vital sign measurement.
type VitalReading struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	UserID     uuid.UUID  `json:"user_id" db:"user_id"`
	VitalType  string     `json:"vital_type" db:"vital_type"`
	Value      float64    `json:"value" db:"value"`
	Unit       string     `json:"unit" db:"unit"`
	Status     string     `json:"status" db:"status"`
	DeviceID   *uuid.UUID `json:"device_id,omitempty" db:"device_id"`
	Notes      *string    `json:"notes,omitempty" db:"notes"`
	MeasuredAt time.Time  `json:"measured_at" db:"measured_at"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
}

// VitalSummaryBucket is a single time-aggregated bucket.
type VitalSummaryBucket struct {
	Bucket       time.Time `json:"bucket"`
	Avg          float64   `json:"avg"`
	Min          float64   `json:"min"`
	Max          float64   `json:"max"`
	ReadingCount int64     `json:"count"`
}

// Default normal ranges per vital type.
var DefaultRanges = map[string][2]float64{
	"heart_rate":                {60, 100},
	"blood_pressure_systolic":   {90, 120},
	"blood_pressure_diastolic":  {60, 80},
	"spo2":                      {95, 100},
	"temperature":               {36.1, 37.2},
	"respiratory_rate":          {12, 20},
}

// ComputeStatus determines the status of a reading based on thresholds.
func ComputeStatus(value, low, high float64) string {
	if value < low {
		margin := low - value
		if margin > (high-low)*0.2 {
			return "critical"
		}
		return "low"
	}
	if value > high {
		margin := value - high
		if margin > (high-low)*0.2 {
			return "critical"
		}
		return "high"
	}
	return "normal"
}

// ValidVitalTypes is the set of accepted vital type strings.
var ValidVitalTypes = map[string]string{
	"heart_rate":                "bpm",
	"blood_pressure_systolic":   "mmHg",
	"blood_pressure_diastolic":  "mmHg",
	"spo2":                      "%",
	"temperature":               "degC",
	"respiratory_rate":          "breaths_per_min",
	"weight":                    "kg",
	"blood_glucose":             "mmol_L",
}
