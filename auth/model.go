package auth

import "github.com/org/vital-api/authprovider"

// LoginRequest is the request body for the login endpoint.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RegisterRequest is the request body for the register endpoint.
type RegisterRequest struct {
	Email       string  `json:"email"`
	Password    string  `json:"password"`
	Name        string  `json:"name"`
	DateOfBirth *string `json:"date_of_birth,omitempty"`
}

// TokenResponse is returned after successful authentication.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	UserID       string `json:"user_id"`
	MFARequired  bool   `json:"mfa_required,omitempty"`
}

// RefreshRequest is the request body for the token refresh endpoint.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// LogoutRequest is the request body for the logout endpoint.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// ForgotPasswordRequest is the request body for forgot-password.
type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

// ResetPasswordRequest is the request body for password reset.
type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

// MFACodeRequest is the request body for MFA verification.
type MFACodeRequest struct {
	Code string `json:"code"`
}

// MFAEnrollmentResponse wraps the MFA enrollment data.
type MFAEnrollmentResponse struct {
	Data *authprovider.MFAEnrollment `json:"data"`
}

// MessageResponse is a generic message response.
type MessageResponse struct {
	Message string `json:"message"`
}

func tokenPairToResponse(tp *authprovider.TokenPair) *TokenResponse {
	return &TokenResponse{
		AccessToken:  tp.AccessToken,
		RefreshToken: tp.RefreshToken,
		ExpiresIn:    tp.ExpiresIn,
		TokenType:    tp.TokenType,
		UserID:       tp.UserID,
		MFARequired:  tp.MFARequired,
	}
}
