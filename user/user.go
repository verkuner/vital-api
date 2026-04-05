package user

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
type UserServiceAPI struct {
	svc *UserService
}

func initUserServiceAPI() (*UserServiceAPI, error) {
	ctx := context.Background()
	db, err := initPool(ctx)
	if err != nil {
		return nil, err
	}
	repo := NewRepository(db)
	svc := NewUserService(repo, slog.Default())
	return &UserServiceAPI{svc: svc}, nil
}

// --- Request / Response types ---

type UpdateProfileRequest struct {
	Name        *string    `json:"name,omitempty"`
	DateOfBirth *time.Time `json:"date_of_birth,omitempty"`
	AvatarURL   *string    `json:"avatar_url,omitempty"`
}

type UserProfileResponse struct {
	Data *User `json:"data"`
}

type DeviceListResponse struct {
	Data []*Device `json:"data"`
}

type RegisterDeviceRequest struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type DeviceResponse struct {
	Data *Device `json:"data"`
}

// --- API Endpoints ---

// GetProfile returns the current user's profile.
// Auto-provisions a local profile from JWT claims on first access.
//
//encore:api auth method=GET path=/api/v1/users/me
func (s *UserServiceAPI) GetProfile(ctx context.Context) (*UserProfileResponse, error) {
	providerID, err := currentProviderID()
	if err != nil {
		return nil, err
	}
	user, err := s.svc.GetProfile(ctx, providerID)
	if err != nil && isErr(err, apperror.ErrNotFound) {
		data := auth.Data().(*authhandler.AuthData)
		name := data.Email
		user, err = s.svc.EnsureUser(ctx, providerID, data.Email, name)
		if err != nil {
			return nil, mapDomainError(err)
		}
	} else if err != nil {
		return nil, mapDomainError(err)
	}
	return &UserProfileResponse{Data: user}, nil
}

// UpdateProfile updates the current user's profile fields.
//
//encore:api auth method=PATCH path=/api/v1/users/me
func (s *UserServiceAPI) UpdateProfile(ctx context.Context, req *UpdateProfileRequest) (*UserProfileResponse, error) {
	providerID, err := currentProviderID()
	if err != nil {
		return nil, err
	}
	user, err := s.svc.GetProfile(ctx, providerID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	updated, err := s.svc.UpdateProfile(ctx, user.ID, *req)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return &UserProfileResponse{Data: updated}, nil
}

// DeleteAccount deletes the current user's account and all associated data.
//
//encore:api auth method=DELETE path=/api/v1/users/me
func (s *UserServiceAPI) DeleteAccount(ctx context.Context) error {
	providerID, err := currentProviderID()
	if err != nil {
		return err
	}
	user, err := s.svc.GetProfile(ctx, providerID)
	if err != nil {
		return mapDomainError(err)
	}
	return mapDomainError(s.svc.DeleteAccount(ctx, user.ID))
}

// ListDevices returns the current user's registered devices.
//
//encore:api auth method=GET path=/api/v1/users/me/devices
func (s *UserServiceAPI) ListDevices(ctx context.Context) (*DeviceListResponse, error) {
	providerID, err := currentProviderID()
	if err != nil {
		return nil, err
	}
	user, err := s.svc.GetProfile(ctx, providerID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	devices, err := s.svc.ListDevices(ctx, user.ID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return &DeviceListResponse{Data: devices}, nil
}

// RegisterDevice registers a new monitoring device.
//
//encore:api auth method=POST path=/api/v1/users/me/devices
func (s *UserServiceAPI) RegisterDevice(ctx context.Context, req *RegisterDeviceRequest) (*DeviceResponse, error) {
	providerID, err := currentProviderID()
	if err != nil {
		return nil, err
	}
	user, err := s.svc.GetProfile(ctx, providerID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	device, err := s.svc.RegisterDevice(ctx, user.ID, *req)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return &DeviceResponse{Data: device}, nil
}

// RemoveDevice removes a registered device.
//
//encore:api auth method=DELETE path=/api/v1/users/me/devices/:id
func (s *UserServiceAPI) RemoveDevice(ctx context.Context, id string) error {
	providerID, err := currentProviderID()
	if err != nil {
		return err
	}
	user, err := s.svc.GetProfile(ctx, providerID)
	if err != nil {
		return mapDomainError(err)
	}
	deviceID, err := uuid.Parse(id)
	if err != nil {
		return &errs.Error{Code: errs.InvalidArgument, Message: "invalid device ID"}
	}
	return mapDomainError(s.svc.RemoveDevice(ctx, deviceID, user.ID))
}

// --- Helpers ---

func currentProviderID() (string, error) {
	uid, ok := auth.UserID()
	if !ok {
		return "", &errs.Error{Code: errs.Unauthenticated, Message: "not authenticated"}
	}
	return string(uid), nil
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
