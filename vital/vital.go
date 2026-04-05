package vital

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
	authhandler "github.com/org/vital-api/authhandler"
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
type Service struct {
	svc *VitalService
}

func initService() (*Service, error) {
	ctx := context.Background()
	db, err := initPool(ctx)
	if err != nil {
		return nil, err
	}
	repo := NewRepository(db)
	svc := NewVitalService(repo, slog.Default())
	return &Service{svc: svc}, nil
}

// --- Request / Response types ---

type RecordVitalRequest struct {
	VitalType  string     `json:"vital_type"`
	Value      float64    `json:"value"`
	Unit       string     `json:"unit"`
	DeviceID   *uuid.UUID `json:"device_id,omitempty"`
	Notes      *string    `json:"notes,omitempty"`
	MeasuredAt time.Time  `json:"measured_at"`
}

type VitalReadingResponse struct {
	Data *VitalReading `json:"data"`
}

type ListVitalsRequest struct {
	VitalType string `query:"vital_type"`
	From      string `query:"from"`
	To        string `query:"to"`
	Before    string `query:"before"`
	Limit     int    `query:"limit"`
}

type ListVitalsResponse struct {
	Data []*VitalReading `json:"data"`
	Meta ListMeta        `json:"meta"`
}

type ListMeta struct {
	Limit      int    `json:"limit"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type SummaryRequest struct {
	VitalType string `query:"vital_type"`
	From      string `query:"from"`
	To        string `query:"to"`
	Bucket    string `query:"bucket"`
}

type SummaryResponse struct {
	Data SummaryData `json:"data"`
}

type SummaryData struct {
	VitalType string               `json:"vital_type"`
	From      time.Time            `json:"from"`
	To        time.Time            `json:"to"`
	Overall   OverallStats         `json:"overall"`
	Buckets   []VitalSummaryBucket `json:"buckets"`
}

type OverallStats struct {
	Avg   float64 `json:"avg"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Count int64   `json:"count"`
}

type LatestVitalsResponse struct {
	Data []*VitalReading `json:"data"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type MeResponse struct {
	UserID string   `json:"user_id"`
	Email  string   `json:"email"`
	Roles  []string `json:"roles"`
}

// --- API Endpoints ---

// Health is a public health check endpoint.
//
//encore:api public method=GET path=/vital/health
func (s *Service) Health(ctx context.Context) (*HealthResponse, error) {
	return &HealthResponse{Status: "ok", Service: "vital"}, nil
}

// Me returns the current authenticated user's information.
//
//encore:api auth method=GET path=/vital/me
func (s *Service) Me(ctx context.Context) (*MeResponse, error) {
	uid, _ := auth.UserID()
	data := auth.Data().(*authhandler.AuthData)
	return &MeResponse{
		UserID: string(uid),
		Email:  data.Email,
		Roles:  data.Roles,
	}, nil
}

// RecordVital records a new vital sign reading.
//
//encore:api auth method=POST path=/api/v1/vitals
func (s *Service) RecordVital(ctx context.Context, req *RecordVitalRequest) (*VitalReadingResponse, error) {
	userID, err := currentUserID()
	if err != nil {
		return nil, err
	}
	reading, err := s.svc.RecordVital(ctx, userID, *req)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return &VitalReadingResponse{Data: reading}, nil
}

// GetVital returns a specific vital reading.
//
//encore:api auth method=GET path=/api/v1/vital/:id
func (s *Service) GetVital(ctx context.Context, id string) (*VitalReadingResponse, error) {
	userID, err := currentUserID()
	if err != nil {
		return nil, err
	}
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, &errs.Error{Code: errs.InvalidArgument, Message: "invalid vital ID"}
	}
	reading, err := s.svc.GetByID(ctx, uid, userID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return &VitalReadingResponse{Data: reading}, nil
}

// ListVitals lists vital readings with filters and pagination.
//
//encore:api auth method=GET path=/api/v1/vitals
func (s *Service) ListVitals(ctx context.Context, req *ListVitalsRequest) (*ListVitalsResponse, error) {
	userID, err := currentUserID()
	if err != nil {
		return nil, err
	}
	limit := 20
	if req.Limit > 0 {
		limit = req.Limit
	}
	var vitalType *string
	if req.VitalType != "" {
		vitalType = &req.VitalType
	}
	params := ListParams{
		VitalType: vitalType,
		Limit:     limit + 1,
	}
	if req.From != "" {
		if t, err := time.Parse(time.RFC3339, req.From); err == nil {
			params.From = t
		}
	}
	if req.To != "" {
		if t, err := time.Parse(time.RFC3339, req.To); err == nil {
			params.To = t
		}
	}
	if req.Before != "" {
		if t, err := time.Parse(time.RFC3339, req.Before); err == nil {
			params.Before = &t
		}
	}

	readings, err := s.svc.ListVitals(ctx, userID, params)
	if err != nil {
		return nil, mapDomainError(err)
	}

	hasMore := len(readings) > limit
	if hasMore {
		readings = readings[:limit]
	}

	nextCursor := ""
	if hasMore && len(readings) > 0 {
		nextCursor = readings[len(readings)-1].MeasuredAt.Format(time.RFC3339Nano)
	}

	return &ListVitalsResponse{
		Data: readings,
		Meta: ListMeta{Limit: limit, HasMore: hasMore, NextCursor: nextCursor},
	}, nil
}

// DeleteVital deletes a vital reading.
//
//encore:api auth method=DELETE path=/api/v1/vital/:id
func (s *Service) DeleteVital(ctx context.Context, id string) error {
	userID, err := currentUserID()
	if err != nil {
		return err
	}
	uid, err := uuid.Parse(id)
	if err != nil {
		return &errs.Error{Code: errs.InvalidArgument, Message: "invalid vital ID"}
	}
	if err := s.svc.DeleteVital(ctx, uid, userID); err != nil {
		return mapDomainError(err)
	}
	return nil
}

// GetSummary returns aggregated vital stats for a time range.
//
//encore:api auth method=GET path=/api/v1/vitals/summary
func (s *Service) GetSummary(ctx context.Context, req *SummaryRequest) (*SummaryResponse, error) {
	userID, err := currentUserID()
	if err != nil {
		return nil, err
	}
	params := SummaryParams{VitalType: req.VitalType, Bucket: req.Bucket}
	if req.From != "" {
		if t, err := time.Parse(time.RFC3339, req.From); err == nil {
			params.From = t
		}
	}
	if req.To != "" {
		if t, err := time.Parse(time.RFC3339, req.To); err == nil {
			params.To = t
		}
	}

	resp, err := s.svc.GetSummary(ctx, userID, params)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return resp, nil
}

// GetLatestVitals returns the most recent reading per vital type.
//
//encore:api auth method=GET path=/api/v1/vitals/latest
func (s *Service) GetLatestVitals(ctx context.Context) (*LatestVitalsResponse, error) {
	userID, err := currentUserID()
	if err != nil {
		return nil, err
	}
	readings, err := s.svc.GetLatest(ctx, userID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return &LatestVitalsResponse{Data: readings}, nil
}

// --- Helpers ---

func currentUserID() (uuid.UUID, error) {
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
	case isErr(err, apperror.ErrRateLimited):
		code = errs.ResourceExhausted
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
