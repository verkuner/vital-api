package authprovider

import "context"

// AuthProvider abstracts the identity provider (Vault) operations.
// Implementations: Supabase Auth (local/dev), Clerk (local/dev alt), Keycloak (production).
type AuthProvider interface {
	Register(ctx context.Context, req RegisterRequest) (*TokenPair, error)
	Login(ctx context.Context, req LoginRequest) (*TokenPair, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error)
	Logout(ctx context.Context, accessToken string) error
	ForgotPassword(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, req ResetPasswordRequest) error
	EnrollMFA(ctx context.Context, userID string) (*MFAEnrollment, error)
	VerifyMFA(ctx context.Context, userID, code string) error
	RemoveMFA(ctx context.Context, userID string) error
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	UserID       string `json:"user_id"`
	MFARequired  bool   `json:"mfa_required,omitempty"`
}

type MFAEnrollment struct {
	QRCode string `json:"qr_code"`
	Secret string `json:"secret"`
	URI    string `json:"uri"`
}
