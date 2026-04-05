package auth

import (
	"context"
	"fmt"

	"encore.dev/beta/auth"
	"encore.dev/beta/errs"
	"github.com/org/vital-api/authprovider"
)

//encore:service
type AuthService struct {
	provider authprovider.AuthProvider
}

func initAuthService() (*AuthService, error) {
	return &AuthService{
		provider: newAuthProvider(),
	}, nil
}

// Register creates a new user account via the configured auth provider.
//
//encore:api public method=POST path=/api/v1/auth/register
func (s *AuthService) Register(ctx context.Context, req *RegisterRequest) (*TokenResponse, error) {
	result, err := s.provider.Register(ctx, authprovider.RegisterRequest{
		Email:    req.Email,
		Password: req.Password,
		Name:     req.Name,
	})
	if err != nil {
		return nil, &errs.Error{Code: errs.Internal, Message: fmt.Sprintf("register: %v", err)}
	}
	return tokenPairToResponse(result), nil
}

// Login authenticates an existing user via the configured auth provider.
//
//encore:api public method=POST path=/api/v1/auth/login
func (s *AuthService) Login(ctx context.Context, req *LoginRequest) (*TokenResponse, error) {
	result, err := s.provider.Login(ctx, authprovider.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return nil, &errs.Error{Code: errs.Unauthenticated, Message: fmt.Sprintf("login: %v", err)}
	}
	return tokenPairToResponse(result), nil
}

// Refresh exchanges a refresh token for a new access token.
//
//encore:api public method=POST path=/api/v1/auth/refresh
func (s *AuthService) Refresh(ctx context.Context, req *RefreshRequest) (*TokenResponse, error) {
	result, err := s.provider.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, &errs.Error{Code: errs.Unauthenticated, Message: fmt.Sprintf("refresh: %v", err)}
	}
	return tokenPairToResponse(result), nil
}

// Logout revokes the current session.
//
//encore:api auth method=POST path=/api/v1/auth/logout
func (s *AuthService) Logout(ctx context.Context, req *LogoutRequest) (*MessageResponse, error) {
	if err := s.provider.Logout(ctx, req.RefreshToken); err != nil {
		return nil, &errs.Error{Code: errs.Internal, Message: fmt.Sprintf("logout: %v", err)}
	}
	return &MessageResponse{Message: "logged out"}, nil
}

// ForgotPassword triggers a password reset email.
//
//encore:api public method=POST path=/api/v1/auth/forgot-password
func (s *AuthService) ForgotPassword(ctx context.Context, req *ForgotPasswordRequest) (*MessageResponse, error) {
	if err := s.provider.ForgotPassword(ctx, req.Email); err != nil {
		return nil, &errs.Error{Code: errs.Internal, Message: fmt.Sprintf("forgot password: %v", err)}
	}
	return &MessageResponse{Message: "reset email sent"}, nil
}

// ResetPassword completes a password reset.
//
//encore:api public method=POST path=/api/v1/auth/reset-password
func (s *AuthService) ResetPassword(ctx context.Context, req *ResetPasswordRequest) (*MessageResponse, error) {
	if err := s.provider.ResetPassword(ctx, authprovider.ResetPasswordRequest{
		Token:       req.Token,
		NewPassword: req.NewPassword,
	}); err != nil {
		return nil, &errs.Error{Code: errs.Internal, Message: fmt.Sprintf("reset password: %v", err)}
	}
	return &MessageResponse{Message: "password reset"}, nil
}

// EnrollMFA begins TOTP MFA enrollment.
//
//encore:api auth method=POST path=/api/v1/auth/mfa/enroll
func (s *AuthService) EnrollMFA(ctx context.Context) (*MFAEnrollmentResponse, error) {
	uid, ok := auth.UserID()
	if !ok {
		return nil, &errs.Error{Code: errs.Unauthenticated, Message: "not authenticated"}
	}
	enrollment, err := s.provider.EnrollMFA(ctx, string(uid))
	if err != nil {
		return nil, &errs.Error{Code: errs.Internal, Message: fmt.Sprintf("enroll MFA: %v", err)}
	}
	return &MFAEnrollmentResponse{Data: enrollment}, nil
}

// VerifyMFA confirms MFA enrollment with a TOTP code.
//
//encore:api auth method=POST path=/api/v1/auth/mfa/verify
func (s *AuthService) VerifyMFA(ctx context.Context, req *MFACodeRequest) (*MessageResponse, error) {
	uid, ok := auth.UserID()
	if !ok {
		return nil, &errs.Error{Code: errs.Unauthenticated, Message: "not authenticated"}
	}
	if err := s.provider.VerifyMFA(ctx, string(uid), req.Code); err != nil {
		return nil, &errs.Error{Code: errs.Internal, Message: fmt.Sprintf("verify MFA: %v", err)}
	}
	return &MessageResponse{Message: "MFA enrolled"}, nil
}

// RemoveMFA removes MFA from the account.
//
//encore:api auth method=DELETE path=/api/v1/auth/mfa
func (s *AuthService) RemoveMFA(ctx context.Context) (*MessageResponse, error) {
	uid, ok := auth.UserID()
	if !ok {
		return nil, &errs.Error{Code: errs.Unauthenticated, Message: "not authenticated"}
	}
	if err := s.provider.RemoveMFA(ctx, string(uid)); err != nil {
		return nil, &errs.Error{Code: errs.Internal, Message: fmt.Sprintf("remove MFA: %v", err)}
	}
	return &MessageResponse{Message: "MFA removed"}, nil
}
