package alert

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/org/vital-api/apperror"
	"github.com/org/vital-api/observability"
)

// AlertService holds business logic for alerts and thresholds.
type AlertService struct {
	repo   Repository
	logger *slog.Logger
}

func NewAlertService(repo Repository, logger *slog.Logger) *AlertService {
	return &AlertService{repo: repo, logger: logger}
}

func (s *AlertService) ListAlerts(ctx context.Context, userID uuid.UUID, params ListAlertParams) ([]*Alert, error) {
	params.UserID = userID
	if params.Limit <= 0 || params.Limit > 100 {
		params.Limit = 20
	}
	if params.From.IsZero() {
		params.From = time.Now().AddDate(0, -1, 0)
	}
	if params.To.IsZero() {
		params.To = time.Now()
	}

	alerts, err := s.repo.ListAlerts(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("alert.Service.ListAlerts: %w", err)
	}
	return alerts, nil
}

func (s *AlertService) GetAlert(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*Alert, error) {
	alert, err := s.repo.FindAlertByID(ctx, id, userID)
	if err != nil {
		return nil, fmt.Errorf("alert.Service.GetAlert: %w", err)
	}
	if alert == nil {
		return nil, fmt.Errorf("alert %s: %w", id, apperror.ErrNotFound)
	}
	return alert, nil
}

func (s *AlertService) AcknowledgeAlert(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*Alert, error) {
	alert, err := s.repo.AcknowledgeAlert(ctx, id, userID)
	if err != nil {
		return nil, fmt.Errorf("alert.Service.AcknowledgeAlert: %w", err)
	}
	if alert == nil {
		return nil, fmt.Errorf("alert %s: %w", id, apperror.ErrNotFound)
	}
	return alert, nil
}

func (s *AlertService) ListThresholds(ctx context.Context, userID uuid.UUID) ([]*AlertThreshold, error) {
	thresholds, err := s.repo.ListThresholds(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("alert.Service.ListThresholds: %w", err)
	}
	return thresholds, nil
}

func (s *AlertService) SetThreshold(ctx context.Context, userID uuid.UUID, vitalType string, req SetThresholdRequest) (*AlertThreshold, error) {
	threshold := &AlertThreshold{
		UserID:    userID,
		VitalType: vitalType,
		LowValue:  req.LowValue,
		HighValue: req.HighValue,
		Enabled:   true,
	}
	if req.Enabled != nil {
		threshold.Enabled = *req.Enabled
	}

	if err := s.repo.UpsertThreshold(ctx, threshold); err != nil {
		return nil, fmt.Errorf("alert.Service.SetThreshold: %w", err)
	}
	return threshold, nil
}

func (s *AlertService) DeleteThreshold(ctx context.Context, userID uuid.UUID, vitalType string) error {
	return s.repo.DeleteThreshold(ctx, userID, vitalType)
}

// CheckAndCreateAlert evaluates a vital reading against thresholds and creates an alert if needed.
func (s *AlertService) CheckAndCreateAlert(ctx context.Context, userID uuid.UUID, vitalType string, value float64, status string) (*Alert, error) {
	if status == "normal" {
		return nil, nil
	}

	threshold, err := s.repo.GetThreshold(ctx, userID, vitalType)
	if err != nil {
		return nil, fmt.Errorf("alert.Service.CheckAndCreateAlert: %w", err)
	}

	var breached string
	var thresholdValue float64
	if threshold != nil && threshold.Enabled {
		if threshold.LowValue != nil && value < *threshold.LowValue {
			breached = "low"
			thresholdValue = *threshold.LowValue
		} else if threshold.HighValue != nil && value > *threshold.HighValue {
			breached = "high"
			thresholdValue = *threshold.HighValue
		}
	}

	if breached == "" {
		if status == "low" || status == "critical" {
			breached = "low"
		} else {
			breached = "high"
		}
	}

	severity := "warning"
	if status == "critical" {
		severity = "critical"
	}

	alert := &Alert{
		UserID:            userID,
		VitalType:         vitalType,
		Value:             value,
		ThresholdBreached: breached,
		ThresholdValue:    thresholdValue,
		Severity:          severity,
	}

	if err := s.repo.CreateAlert(ctx, alert); err != nil {
		return nil, fmt.Errorf("alert.Service.CheckAndCreateAlert: %w", err)
	}

	observability.RecordAlertGenerated(ctx, severity, vitalType)
	s.logger.InfoContext(ctx, "alert generated",
		slog.String("user_id", userID.String()),
		slog.String("vital_type", vitalType),
		slog.String("severity", severity),
		slog.String("breached", breached),
	)
	return alert, nil
}
