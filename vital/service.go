package vital

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/org/vital-api/apperror"
	"github.com/org/vital-api/observability"
)

// VitalService holds business logic for vital readings.
type VitalService struct {
	repo   Repository
	logger *slog.Logger
}

func NewVitalService(repo Repository, logger *slog.Logger) *VitalService {
	return &VitalService{repo: repo, logger: logger}
}

func (s *VitalService) RecordVital(ctx context.Context, userID uuid.UUID, req RecordVitalRequest) (*VitalReading, error) {
	if _, ok := ValidVitalTypes[req.VitalType]; !ok {
		return nil, fmt.Errorf("unknown vital type %q: %w", req.VitalType, apperror.ErrValidation)
	}
	if req.Value <= 0 {
		return nil, fmt.Errorf("value must be positive: %w", apperror.ErrValidation)
	}
	if req.MeasuredAt.After(time.Now().Add(5 * time.Minute)) {
		return nil, fmt.Errorf("measured_at cannot be in the future: %w", apperror.ErrValidation)
	}

	status := "normal"
	if rng, ok := DefaultRanges[req.VitalType]; ok {
		status = ComputeStatus(req.Value, rng[0], rng[1])
	}

	reading := &VitalReading{
		UserID:     userID,
		VitalType:  req.VitalType,
		Value:      req.Value,
		Unit:       req.Unit,
		Status:     status,
		DeviceID:   req.DeviceID,
		Notes:      req.Notes,
		MeasuredAt: req.MeasuredAt,
	}

	if err := s.repo.Create(ctx, reading); err != nil {
		return nil, fmt.Errorf("vital.Service.RecordVital: %w", err)
	}

	observability.RecordVitalCreated(ctx, req.VitalType)
	s.logger.InfoContext(ctx, "vital recorded",
		slog.String("user_id", userID.String()),
		slog.String("vital_type", req.VitalType),
		slog.String("status", status),
	)
	return reading, nil
}

func (s *VitalService) GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*VitalReading, error) {
	reading, err := s.repo.FindByID(ctx, id, userID)
	if err != nil {
		return nil, fmt.Errorf("vital.Service.GetByID: %w", err)
	}
	if reading == nil {
		return nil, fmt.Errorf("vital %s: %w", id, apperror.ErrNotFound)
	}
	return reading, nil
}

func (s *VitalService) ListVitals(ctx context.Context, userID uuid.UUID, params ListParams) ([]*VitalReading, error) {
	params.UserID = userID
	if params.Limit <= 0 || params.Limit > 100 {
		params.Limit = 20
	}
	if params.From.IsZero() {
		params.From = time.Now().AddDate(0, 0, -7)
	}
	if params.To.IsZero() {
		params.To = time.Now()
	}

	readings, err := s.repo.List(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("vital.Service.ListVitals: %w", err)
	}
	return readings, nil
}

func (s *VitalService) DeleteVital(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	existing, err := s.repo.FindByID(ctx, id, userID)
	if err != nil {
		return fmt.Errorf("vital.Service.DeleteVital: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("vital %s: %w", id, apperror.ErrNotFound)
	}
	if err := s.repo.Delete(ctx, id, userID); err != nil {
		return fmt.Errorf("vital.Service.DeleteVital: %w", err)
	}
	return nil
}

func (s *VitalService) GetLatest(ctx context.Context, userID uuid.UUID) ([]*VitalReading, error) {
	readings, err := s.repo.GetLatestByType(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("vital.Service.GetLatest: %w", err)
	}
	return readings, nil
}

func (s *VitalService) GetSummary(ctx context.Context, userID uuid.UUID, params SummaryParams) (*SummaryResponse, error) {
	params.UserID = userID
	if params.Bucket == "" {
		params.Bucket = "hour"
	}
	if params.From.IsZero() {
		params.From = time.Now().AddDate(0, 0, -7)
	}
	if params.To.IsZero() {
		params.To = time.Now()
	}

	buckets, err := s.repo.GetSummary(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("vital.Service.GetSummary: %w", err)
	}

	var overall struct {
		avg   float64
		min   float64
		max   float64
		count int64
	}
	if len(buckets) > 0 {
		overall.min = buckets[0].Min
		overall.max = buckets[0].Max
	}
	for _, b := range buckets {
		overall.avg += b.Avg * float64(b.ReadingCount)
		overall.count += b.ReadingCount
		if b.Min < overall.min {
			overall.min = b.Min
		}
		if b.Max > overall.max {
			overall.max = b.Max
		}
	}
	avgValue := 0.0
	if overall.count > 0 {
		avgValue = overall.avg / float64(overall.count)
	}

	return &SummaryResponse{
		Data: SummaryData{
			VitalType: params.VitalType,
			From:      params.From,
			To:        params.To,
			Overall: OverallStats{
				Avg:   avgValue,
				Min:   overall.min,
				Max:   overall.max,
				Count: overall.count,
			},
			Buckets: buckets,
		},
	}, nil
}
