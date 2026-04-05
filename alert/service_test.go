package alert

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var testLogger = slog.Default()

// --- Mock Repository ---

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) CreateAlert(ctx context.Context, alert *Alert) error {
	args := m.Called(ctx, alert)
	if args.Error(0) == nil {
		alert.ID = uuid.New()
		alert.CreatedAt = time.Now().UTC()
	}
	return args.Error(0)
}

func (m *mockRepository) FindAlertByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*Alert, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Alert), args.Error(1)
}

func (m *mockRepository) ListAlerts(ctx context.Context, params ListAlertParams) ([]*Alert, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Alert), args.Error(1)
}

func (m *mockRepository) AcknowledgeAlert(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*Alert, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Alert), args.Error(1)
}

func (m *mockRepository) GetThreshold(ctx context.Context, userID uuid.UUID, vitalType string) (*AlertThreshold, error) {
	args := m.Called(ctx, userID, vitalType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*AlertThreshold), args.Error(1)
}

func (m *mockRepository) ListThresholds(ctx context.Context, userID uuid.UUID) ([]*AlertThreshold, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*AlertThreshold), args.Error(1)
}

func (m *mockRepository) UpsertThreshold(ctx context.Context, threshold *AlertThreshold) error {
	args := m.Called(ctx, threshold)
	if args.Error(0) == nil {
		threshold.ID = uuid.New()
	}
	return args.Error(0)
}

func (m *mockRepository) DeleteThreshold(ctx context.Context, userID uuid.UUID, vitalType string) error {
	args := m.Called(ctx, userID, vitalType)
	return args.Error(0)
}

// --- Tests ---

func TestAlertService_CheckAndCreateAlert_NormalStatus_NoAlert(t *testing.T) {
	repo := new(mockRepository)
	svc := NewAlertService(repo, testLogger)

	alert, err := svc.CheckAndCreateAlert(context.Background(), uuid.New(), "heart_rate", 75.0, "normal")

	require.NoError(t, err)
	assert.Nil(t, alert)
	repo.AssertNotCalled(t, "CreateAlert")
}

func TestAlertService_CheckAndCreateAlert_HighStatus_CreatesWarning(t *testing.T) {
	repo := new(mockRepository)
	svc := NewAlertService(repo, testLogger)

	userID := uuid.New()
	high := 100.0
	threshold := &AlertThreshold{
		UserID:    userID,
		VitalType: "heart_rate",
		HighValue: &high,
		Enabled:   true,
	}

	repo.On("GetThreshold", mock.Anything, userID, "heart_rate").Return(threshold, nil)
	repo.On("CreateAlert", mock.Anything, mock.MatchedBy(func(a *Alert) bool {
		return a.Severity == "warning" && a.ThresholdBreached == "high"
	})).Return(nil)

	alert, err := svc.CheckAndCreateAlert(context.Background(), userID, "heart_rate", 105.0, "high")

	require.NoError(t, err)
	assert.NotNil(t, alert)
	assert.Equal(t, "warning", alert.Severity)
	repo.AssertExpectations(t)
}

func TestAlertService_CheckAndCreateAlert_CriticalStatus_CreatesCritical(t *testing.T) {
	repo := new(mockRepository)
	svc := NewAlertService(repo, testLogger)

	userID := uuid.New()

	repo.On("GetThreshold", mock.Anything, userID, "heart_rate").Return(nil, nil)
	repo.On("CreateAlert", mock.Anything, mock.MatchedBy(func(a *Alert) bool {
		return a.Severity == "critical"
	})).Return(nil)

	alert, err := svc.CheckAndCreateAlert(context.Background(), userID, "heart_rate", 150.0, "critical")

	require.NoError(t, err)
	assert.NotNil(t, alert)
	assert.Equal(t, "critical", alert.Severity)
	repo.AssertExpectations(t)
}

func TestAlertService_SetThreshold_Success(t *testing.T) {
	repo := new(mockRepository)
	svc := NewAlertService(repo, testLogger)

	userID := uuid.New()
	low := 50.0
	high := 110.0
	enabled := true

	repo.On("UpsertThreshold", mock.Anything, mock.MatchedBy(func(t *AlertThreshold) bool {
		return t.UserID == userID && t.VitalType == "heart_rate"
	})).Return(nil)

	result, err := svc.SetThreshold(context.Background(), userID, "heart_rate", SetThresholdRequest{
		LowValue:  &low,
		HighValue: &high,
		Enabled:   &enabled,
	})

	require.NoError(t, err)
	assert.Equal(t, "heart_rate", result.VitalType)
	assert.Equal(t, &low, result.LowValue)
	repo.AssertExpectations(t)
}
