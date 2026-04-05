package authprovider

import (
	"context"
	"fmt"
)

// ClerkProvider implements AuthProvider using the Clerk Backend API.
// This is a stub for local/dev as an alternative to Supabase.
type ClerkProvider struct {
	secretKey string
}

func NewClerkProvider(secretKey string) *ClerkProvider {
	return &ClerkProvider{secretKey: secretKey}
}

func (p *ClerkProvider) Register(ctx context.Context, req RegisterRequest) (*TokenPair, error) {
	return nil, fmt.Errorf("clerk register: not yet implemented")
}

func (p *ClerkProvider) Login(ctx context.Context, req LoginRequest) (*TokenPair, error) {
	return nil, fmt.Errorf("clerk login: not yet implemented")
}

func (p *ClerkProvider) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	return nil, fmt.Errorf("clerk refresh: not yet implemented")
}

func (p *ClerkProvider) Logout(ctx context.Context, accessToken string) error {
	return fmt.Errorf("clerk logout: not yet implemented")
}

func (p *ClerkProvider) ForgotPassword(ctx context.Context, email string) error {
	return fmt.Errorf("clerk forgot password: not yet implemented")
}

func (p *ClerkProvider) ResetPassword(ctx context.Context, req ResetPasswordRequest) error {
	return fmt.Errorf("clerk reset password: not yet implemented")
}

func (p *ClerkProvider) EnrollMFA(ctx context.Context, userID string) (*MFAEnrollment, error) {
	return nil, fmt.Errorf("clerk enroll MFA: not yet implemented")
}

func (p *ClerkProvider) VerifyMFA(ctx context.Context, userID, code string) error {
	return fmt.Errorf("clerk verify MFA: not yet implemented")
}

func (p *ClerkProvider) RemoveMFA(ctx context.Context, userID string) error {
	return fmt.Errorf("clerk remove MFA: not yet implemented")
}
