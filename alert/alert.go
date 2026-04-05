package alert

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"encore.dev/beta/auth"
	"encore.dev/beta/errs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/org/vital-api/apperror"
)

var secrets struct {
	DatabaseURL string
}

var pool *pgxpool.Pool

func initPool(ctx context.Context) (*pgxpool.Pool, error) {
	if pool != nil {
		return pool, nil
	}
	p, err := pgxpool.New(ctx, secrets.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	if err := p.Ping(ctx); err != nil {
		p.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	pool = p
	return pool, nil
}

//encore:service
type AlertServiceAPI struct {
	svc *AlertService
}

func initAlertServiceAPI() (*AlertServiceAPI, error) {
	ctx := context.Background()
	db, err := initPool(ctx)
	if err != nil {
		return nil, err
	}
	repo := NewRepository(db)
	svc := NewAlertService(repo, slog.Default())
	return &AlertServiceAPI{svc: svc}, nil
}

// --- Request / Response types ---

type ListAlertsRequest struct {
	Severity     string `query:"severity"`
	Acknowledged string `query:"acknowledged"`
	VitalType    string `query:"vital_type"`
	From         string `query:"from"`
	To           string `query:"to"`
	Page         int    `query:"page"`
	PerPage      int    `query:"per_page"`
}

type ListAlertsResponse struct {
	Data []*Alert  `json:"data"`
	Meta AlertMeta `json:"meta"`
}

type AlertMeta struct {
	Total   int `json:"total"`
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
}

type AlertResponse struct {
	Data *Alert `json:"data"`
}

type SetThresholdRequest struct {
	LowValue  *float64 `json:"low_value,omitempty"`
	HighValue *float64 `json:"high_value,omitempty"`
	Enabled   *bool    `json:"enabled,omitempty"`
}

type ThresholdResponse struct {
	Data *AlertThreshold `json:"data"`
}

type ThresholdListResponse struct {
	Data []*AlertThreshold `json:"data"`
}

// --- API Endpoints ---

// ListAlerts returns a filtered, paginated list of alerts.
//
//encore:api auth method=GET path=/api/v1/alerts
func (s *AlertServiceAPI) ListAlerts(ctx context.Context, req *ListAlertsRequest) (*ListAlertsResponse, error) {
	userID, err := currentUserUUID()
	if err != nil {
		return nil, err
	}

	page := 1
	perPage := 20
	if req.Page > 0 {
		page = req.Page
	}
	if req.PerPage > 0 && req.PerPage <= 100 {
		perPage = req.PerPage
	}

	var severity, vitalType *string
	var acknowledged *bool
	if req.Severity != "" {
		severity = &req.Severity
	}
	if req.VitalType != "" {
		vitalType = &req.VitalType
	}
	if req.Acknowledged == "true" {
		v := true
		acknowledged = &v
	} else if req.Acknowledged == "false" {
		v := false
		acknowledged = &v
	}

	params := ListAlertParams{
		Severity:     severity,
		Acknowledged: acknowledged,
		VitalType:    vitalType,
		Limit:        perPage,
		Offset:       (page - 1) * perPage,
	}
	if req.From != "" {
		if t, parseErr := time.Parse(time.RFC3339, req.From); parseErr == nil {
			params.From = t
		}
	}
	if req.To != "" {
		if t, parseErr := time.Parse(time.RFC3339, req.To); parseErr == nil {
			params.To = t
		}
	}

	alerts, err := s.svc.ListAlerts(ctx, userID, params)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return &ListAlertsResponse{
		Data: alerts,
		Meta: AlertMeta{Total: len(alerts), Page: page, PerPage: perPage},
	}, nil
}

// GetAlert returns a specific alert.
//
//encore:api auth method=GET path=/api/v1/alert/:id
func (s *AlertServiceAPI) GetAlert(ctx context.Context, id string) (*AlertResponse, error) {
	userID, err := currentUserUUID()
	if err != nil {
		return nil, err
	}
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, &errs.Error{Code: errs.InvalidArgument, Message: "invalid alert ID"}
	}
	alert, err := s.svc.GetAlert(ctx, uid, userID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return &AlertResponse{Data: alert}, nil
}

// AcknowledgeAlert marks an alert as acknowledged.
//
//encore:api auth method=POST path=/api/v1/alert/:id/acknowledge
func (s *AlertServiceAPI) AcknowledgeAlert(ctx context.Context, id string) (*AlertResponse, error) {
	userID, err := currentUserUUID()
	if err != nil {
		return nil, err
	}
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, &errs.Error{Code: errs.InvalidArgument, Message: "invalid alert ID"}
	}
	alert, err := s.svc.AcknowledgeAlert(ctx, uid, userID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return &AlertResponse{Data: alert}, nil
}

// ListThresholds returns the current user's alert thresholds.
//
//encore:api auth method=GET path=/api/v1/thresholds
func (s *AlertServiceAPI) ListThresholds(ctx context.Context) (*ThresholdListResponse, error) {
	userID, err := currentUserUUID()
	if err != nil {
		return nil, err
	}
	thresholds, err := s.svc.ListThresholds(ctx, userID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return &ThresholdListResponse{Data: thresholds}, nil
}

// SetThreshold sets or updates a threshold for a vital type.
//
//encore:api auth method=PUT path=/api/v1/thresholds/:vitalType
func (s *AlertServiceAPI) SetThreshold(ctx context.Context, vitalType string, req *SetThresholdRequest) (*ThresholdResponse, error) {
	userID, err := currentUserUUID()
	if err != nil {
		return nil, err
	}
	threshold, err := s.svc.SetThreshold(ctx, userID, vitalType, *req)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return &ThresholdResponse{Data: threshold}, nil
}

// DeleteThreshold removes a threshold (reverts to defaults).
//
//encore:api auth method=DELETE path=/api/v1/thresholds/:vitalType
func (s *AlertServiceAPI) DeleteThreshold(ctx context.Context, vitalType string) error {
	userID, err := currentUserUUID()
	if err != nil {
		return err
	}
	return mapDomainError(s.svc.DeleteThreshold(ctx, userID, vitalType))
}

// --- Helpers ---

func currentUserUUID() (uuid.UUID, error) {
	uid, ok := auth.UserID()
	if !ok {
		return uuid.Nil, &errs.Error{Code: errs.Unauthenticated, Message: "not authenticated"}
	}
	providerID := string(uid)
	var userID uuid.UUID
	err := pool.QueryRow(context.Background(),
		`SELECT id FROM users WHERE provider_id = $1`, providerID).Scan(&userID)
	if err != nil {
		return uuid.Nil, &errs.Error{Code: errs.Unauthenticated, Message: "user profile not found — call GET /api/v1/users/me first"}
	}
	return userID, nil
}

func mapDomainError(err error) error {
	if err == nil {
		return nil
	}
	code := errs.Internal
	switch {
	case isErr(err, apperror.ErrNotFound):
		code = errs.NotFound
	case isErr(err, apperror.ErrUnauthorized):
		code = errs.Unauthenticated
	case isErr(err, apperror.ErrForbidden):
		code = errs.PermissionDenied
	case isErr(err, apperror.ErrValidation):
		code = errs.InvalidArgument
	case isErr(err, apperror.ErrConflict):
		code = errs.AlreadyExists
	}
	return &errs.Error{Code: code, Message: err.Error()}
}

func isErr(err, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		unwrapped, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapped.Unwrap()
	}
	return false
}
