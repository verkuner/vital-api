package vital

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/org/vital-api/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var testLogger = slog.Default()

// --- Mock Repository ---

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) Create(ctx context.Context, reading *VitalReading) error {
	args := m.Called(ctx, reading)
	if args.Error(0) == nil {
		reading.ID = uuid.New()
		reading.CreatedAt = time.Now().UTC()
	}
	return args.Error(0)
}

func (m *mockRepository) FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*VitalReading, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*VitalReading), args.Error(1)
}

func (m *mockRepository) List(ctx context.Context, params ListParams) ([]*VitalReading, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*VitalReading), args.Error(1)
}

func (m *mockRepository) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	args := m.Called(ctx, id, userID)
	return args.Error(0)
}

func (m *mockRepository) GetLatestByType(ctx context.Context, userID uuid.UUID) ([]*VitalReading, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*VitalReading), args.Error(1)
}

func (m *mockRepository) GetSummary(ctx context.Context, params SummaryParams) ([]VitalSummaryBucket, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]VitalSummaryBucket), args.Error(1)
}

// --- Tests ---

func TestService_RecordVital_ValidReading_ReturnsCreated(t *testing.T) {
	repo := new(mockRepository)
	svc := NewVitalService(repo, testLogger)

	userID := uuid.New()
	input := RecordVitalRequest{
		VitalType:  "heart_rate",
		Value:      72.0,
		Unit:       "bpm",
		MeasuredAt: time.Now().Add(-1 * time.Minute),
	}

	repo.On("Create", mock.Anything, mock.MatchedBy(func(r *VitalReading) bool {
		return r.VitalType == "heart_rate" && r.UserID == userID
	})).Return(nil)

	result, err := svc.RecordVital(context.Background(), userID, input)

	require.NoError(t, err)
	assert.Equal(t, "heart_rate", result.VitalType)
	assert.Equal(t, userID, result.UserID)
	assert.Equal(t, "normal", result.Status)
	repo.AssertExpectations(t)
}

func TestService_RecordVital_FutureTimestamp_ReturnsError(t *testing.T) {
	repo := new(mockRepository)
	svc := NewVitalService(repo, testLogger)

	input := RecordVitalRequest{
		VitalType:  "heart_rate",
		Value:      72.0,
		Unit:       "bpm",
		MeasuredAt: time.Now().Add(10 * time.Minute),
	}

	_, err := svc.RecordVital(context.Background(), uuid.New(), input)

	require.Error(t, err)
	assert.ErrorIs(t, err, apperror.ErrValidation)
	repo.AssertNotCalled(t, "Create")
}

func TestService_RecordVital_InvalidVitalType_ReturnsError(t *testing.T) {
	repo := new(mockRepository)
	svc := NewVitalService(repo, testLogger)

	input := RecordVitalRequest{
		VitalType:  "invalid_type",
		Value:      72.0,
		Unit:       "bpm",
		MeasuredAt: time.Now(),
	}

	_, err := svc.RecordVital(context.Background(), uuid.New(), input)

	require.Error(t, err)
	assert.ErrorIs(t, err, apperror.ErrValidation)
}

func TestService_RecordVital_HighValue_ReturnsNonNormalStatus(t *testing.T) {
	repo := new(mockRepository)
	svc := NewVitalService(repo, testLogger)

	userID := uuid.New()
	input := RecordVitalRequest{
		VitalType:  "heart_rate",
		Value:      105.0,
		Unit:       "bpm",
		MeasuredAt: time.Now().Add(-1 * time.Minute),
	}

	repo.On("Create", mock.Anything, mock.MatchedBy(func(r *VitalReading) bool {
		return r.Status == "high"
	})).Return(nil)

	result, err := svc.RecordVital(context.Background(), userID, input)

	require.NoError(t, err)
	assert.Equal(t, "high", result.Status)
	repo.AssertExpectations(t)
}

func TestService_GetByID_NotFound_ReturnsError(t *testing.T) {
	repo := new(mockRepository)
	svc := NewVitalService(repo, testLogger)

	id := uuid.New()
	userID := uuid.New()

	repo.On("FindByID", mock.Anything, id, userID).Return(nil, nil)

	_, err := svc.GetByID(context.Background(), id, userID)

	require.Error(t, err)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
	repo.AssertExpectations(t)
}

func TestService_RecordVital_VitalTypeValidation(t *testing.T) {
	tests := []struct {
		name      string
		vitalType string
		wantErr   bool
	}{
		{"valid heart_rate", "heart_rate", false},
		{"valid spo2", "spo2", false},
		{"valid temperature", "temperature", false},
		{"invalid empty", "", true},
		{"invalid unknown type", "blood_sugar", true},
		{"invalid uppercase", "HEART_RATE", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(mockRepository)
			svc := NewVitalService(repo, testLogger)

			if !tt.wantErr {
				repo.On("Create", mock.Anything, mock.Anything).Return(nil)
			}

			_, err := svc.RecordVital(context.Background(), uuid.New(), RecordVitalRequest{
				VitalType:  tt.vitalType,
				Value:      72,
				Unit:       "bpm",
				MeasuredAt: time.Now().Add(-1 * time.Minute),
			})
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestComputeStatus(t *testing.T) {
	tests := []struct {
		name   string
		value  float64
		low    float64
		high   float64
		expect string
	}{
		{"normal", 75.0, 60.0, 100.0, "normal"},
		{"low boundary", 59.0, 60.0, 100.0, "low"},
		{"high boundary", 101.0, 60.0, 100.0, "high"},
		{"critical low", 40.0, 60.0, 100.0, "critical"},
		{"critical high", 130.0, 60.0, 100.0, "critical"},
		{"exact low", 60.0, 60.0, 100.0, "normal"},
		{"exact high", 100.0, 60.0, 100.0, "normal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeStatus(tt.value, tt.low, tt.high)
			assert.Equal(t, tt.expect, result)
		})
	}
}
