package user

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/org/vital-api/apperror"
)

// UserService holds business logic for user profiles and devices.
type UserService struct {
	repo   Repository
	logger *slog.Logger
}

func NewUserService(repo Repository, logger *slog.Logger) *UserService {
	return &UserService{repo: repo, logger: logger}
}

func (s *UserService) GetProfile(ctx context.Context, providerID string) (*User, error) {
	user, err := s.repo.FindByProviderID(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("user.Service.GetProfile: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user profile for provider %s: %w", providerID, apperror.ErrNotFound)
	}
	return user, nil
}

func (s *UserService) UpdateProfile(ctx context.Context, userID uuid.UUID, req UpdateProfileRequest) (*User, error) {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user.Service.UpdateProfile: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user %s: %w", userID, apperror.ErrNotFound)
	}

	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.DateOfBirth != nil {
		user.DateOfBirth = req.DateOfBirth
	}
	if req.AvatarURL != nil {
		user.AvatarURL = req.AvatarURL
	}

	if err := s.repo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("user.Service.UpdateProfile: %w", err)
	}
	return user, nil
}

func (s *UserService) DeleteAccount(ctx context.Context, userID uuid.UUID) error {
	if err := s.repo.Delete(ctx, userID); err != nil {
		return fmt.Errorf("user.Service.DeleteAccount: %w", err)
	}
	return nil
}

func (s *UserService) ListDevices(ctx context.Context, userID uuid.UUID) ([]*Device, error) {
	devices, err := s.repo.ListDevices(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user.Service.ListDevices: %w", err)
	}
	return devices, nil
}

func (s *UserService) RegisterDevice(ctx context.Context, userID uuid.UUID, req RegisterDeviceRequest) (*Device, error) {
	device := &Device{
		UserID: userID,
		Name:   req.Name,
		Type:   req.Type,
	}
	if err := s.repo.CreateDevice(ctx, device); err != nil {
		return nil, fmt.Errorf("user.Service.RegisterDevice: %w", err)
	}
	return device, nil
}

func (s *UserService) RemoveDevice(ctx context.Context, deviceID uuid.UUID, userID uuid.UUID) error {
	if err := s.repo.DeleteDevice(ctx, deviceID, userID); err != nil {
		return fmt.Errorf("user.Service.RemoveDevice: %w", err)
	}
	return nil
}

// EnsureUser creates or retrieves a user record by provider ID.
// Called after authentication to ensure a local profile exists.
func (s *UserService) EnsureUser(ctx context.Context, providerID, email, name string) (*User, error) {
	existing, err := s.repo.FindByProviderID(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("user.Service.EnsureUser: %w", err)
	}
	if existing != nil {
		return existing, nil
	}

	user := &User{
		ProviderID: providerID,
		Email:      email,
		Name:       name,
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("user.Service.EnsureUser create: %w", err)
	}
	return user, nil
}
