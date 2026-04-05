package authprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// SupabaseProvider implements AuthProvider using the Supabase Auth REST API.
type SupabaseProvider struct {
	baseURL    string
	anonKey    string
	serviceKey string
	client     *http.Client
}

func NewSupabaseProvider(baseURL, anonKey, serviceKey string) *SupabaseProvider {
	return &SupabaseProvider{
		baseURL:    baseURL,
		anonKey:    anonKey,
		serviceKey: serviceKey,
		client:     &http.Client{},
	}
}

func (p *SupabaseProvider) Register(ctx context.Context, req RegisterRequest) (*TokenPair, error) {
	body := map[string]interface{}{
		"email":    req.Email,
		"password": req.Password,
		"data":     map[string]string{"name": req.Name},
	}
	var result supabaseTokenResponse
	if err := p.post(ctx, "/auth/v1/signup", body, &result); err != nil {
		return nil, fmt.Errorf("supabase register: %w", err)
	}
	return result.toTokenPair(), nil
}

func (p *SupabaseProvider) Login(ctx context.Context, req LoginRequest) (*TokenPair, error) {
	body := map[string]string{"email": req.Email, "password": req.Password}
	var result supabaseTokenResponse
	if err := p.post(ctx, "/auth/v1/token?grant_type=password", body, &result); err != nil {
		return nil, fmt.Errorf("supabase login: %w", err)
	}
	return result.toTokenPair(), nil
}

func (p *SupabaseProvider) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	body := map[string]string{"refresh_token": refreshToken}
	var result supabaseTokenResponse
	if err := p.post(ctx, "/auth/v1/token?grant_type=refresh_token", body, &result); err != nil {
		return nil, fmt.Errorf("supabase refresh: %w", err)
	}
	return result.toTokenPair(), nil
}

func (p *SupabaseProvider) Logout(ctx context.Context, accessToken string) error {
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/auth/v1/logout", nil)
	if err != nil {
		return fmt.Errorf("create logout request: %w", err)
	}
	req.Header.Set("apikey", p.anonKey)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("supabase logout: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("supabase logout returned %d", resp.StatusCode)
	}
	return nil
}

func (p *SupabaseProvider) ForgotPassword(ctx context.Context, email string) error {
	body := map[string]interface{}{
		"email": email,
	}
	return p.post(ctx, "/auth/v1/recover", body, nil)
}

func (p *SupabaseProvider) ResetPassword(ctx context.Context, req ResetPasswordRequest) error {
	body := map[string]string{"password": req.NewPassword}
	r, err := http.NewRequestWithContext(ctx, "PUT", p.baseURL+"/auth/v1/user", nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	data, _ := json.Marshal(body)
	r.Body = io.NopCloser(bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("apikey", p.anonKey)
	r.Header.Set("Authorization", "Bearer "+req.Token)

	resp, err := p.client.Do(r)
	if err != nil {
		return fmt.Errorf("supabase reset password: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("supabase reset password returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (p *SupabaseProvider) EnrollMFA(ctx context.Context, userID string) (*MFAEnrollment, error) {
	return nil, fmt.Errorf("MFA enrollment: %w", fmt.Errorf("not yet implemented for Supabase"))
}

func (p *SupabaseProvider) VerifyMFA(ctx context.Context, userID, code string) error {
	return fmt.Errorf("MFA verify: %w", fmt.Errorf("not yet implemented for Supabase"))
}

func (p *SupabaseProvider) RemoveMFA(ctx context.Context, userID string) error {
	return fmt.Errorf("MFA remove: %w", fmt.Errorf("not yet implemented for Supabase"))
}

type supabaseTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	User         struct {
		ID string `json:"id"`
	} `json:"user"`
}

func (r *supabaseTokenResponse) toTokenPair() *TokenPair {
	return &TokenPair{
		AccessToken:  r.AccessToken,
		RefreshToken: r.RefreshToken,
		ExpiresIn:    r.ExpiresIn,
		TokenType:    r.TokenType,
		UserID:       r.User.ID,
	}
}

func (p *SupabaseProvider) post(ctx context.Context, path string, payload interface{}, result interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", p.anonKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("supabase returned %d: %s", resp.StatusCode, string(body))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
