package authprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// KeycloakProvider implements AuthProvider using the Keycloak Admin REST API.
// Used in production deployments.
type KeycloakProvider struct {
	baseURL           string
	realm             string
	clientID          string
	adminClientID     string
	adminClientSecret string
	client            *http.Client
}

func NewKeycloakProvider(baseURL, realm, clientID, adminClientID, adminClientSecret string) *KeycloakProvider {
	return &KeycloakProvider{
		baseURL:           strings.TrimRight(baseURL, "/"),
		realm:             realm,
		clientID:          clientID,
		adminClientID:     adminClientID,
		adminClientSecret: adminClientSecret,
		client:            &http.Client{},
	}
}

func (p *KeycloakProvider) tokenEndpoint() string {
	return fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", p.baseURL, p.realm)
}

func (p *KeycloakProvider) adminURL(path string) string {
	return fmt.Sprintf("%s/admin/realms/%s%s", p.baseURL, p.realm, path)
}

func (p *KeycloakProvider) Register(ctx context.Context, req RegisterRequest) (*TokenPair, error) {
	adminToken, err := p.getAdminToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("keycloak admin token: %w", err)
	}

	userPayload := map[string]interface{}{
		"username":  req.Email,
		"email":     req.Email,
		"enabled":   true,
		"firstName": req.Name,
		"credentials": []map[string]interface{}{
			{"type": "password", "value": req.Password, "temporary": false},
		},
	}

	data, _ := json.Marshal(userPayload)
	r, err := http.NewRequestWithContext(ctx, "POST", p.adminURL("/users"), bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create user request: %w", err)
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := p.client.Do(r)
	if err != nil {
		return nil, fmt.Errorf("keycloak create user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("keycloak create user returned %d: %s", resp.StatusCode, string(body))
	}

	return p.Login(ctx, LoginRequest{Email: req.Email, Password: req.Password})
}

func (p *KeycloakProvider) Login(ctx context.Context, req LoginRequest) (*TokenPair, error) {
	form := url.Values{
		"grant_type": {"password"},
		"client_id":  {p.clientID},
		"username":   {req.Email},
		"password":   {req.Password},
	}

	r, err := http.NewRequestWithContext(ctx, "POST", p.tokenEndpoint(), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create login request: %w", err)
	}
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(r)
	if err != nil {
		return nil, fmt.Errorf("keycloak login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("keycloak login returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode login response: %w", err)
	}

	return &TokenPair{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresIn,
		TokenType:    result.TokenType,
	}, nil
}

func (p *KeycloakProvider) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {p.clientID},
		"refresh_token": {refreshToken},
	}

	r, err := http.NewRequestWithContext(ctx, "POST", p.tokenEndpoint(), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create refresh request: %w", err)
	}
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(r)
	if err != nil {
		return nil, fmt.Errorf("keycloak refresh: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("keycloak refresh returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode refresh response: %w", err)
	}

	return &TokenPair{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresIn,
		TokenType:    result.TokenType,
	}, nil
}

func (p *KeycloakProvider) Logout(ctx context.Context, accessToken string) error {
	endpoint := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/logout", p.baseURL, p.realm)
	r, err := http.NewRequestWithContext(ctx, "POST", endpoint, nil)
	if err != nil {
		return fmt.Errorf("create logout request: %w", err)
	}
	r.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := p.client.Do(r)
	if err != nil {
		return fmt.Errorf("keycloak logout: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

func (p *KeycloakProvider) ForgotPassword(_ context.Context, _ string) error {
	return fmt.Errorf("keycloak forgot password: not yet implemented")
}

func (p *KeycloakProvider) ResetPassword(_ context.Context, _ ResetPasswordRequest) error {
	return fmt.Errorf("keycloak reset password: not yet implemented")
}

func (p *KeycloakProvider) EnrollMFA(_ context.Context, _ string) (*MFAEnrollment, error) {
	return nil, fmt.Errorf("keycloak enroll MFA: not yet implemented")
}

func (p *KeycloakProvider) VerifyMFA(_ context.Context, _, _ string) error {
	return fmt.Errorf("keycloak verify MFA: not yet implemented")
}

func (p *KeycloakProvider) RemoveMFA(_ context.Context, _ string) error {
	return fmt.Errorf("keycloak remove MFA: not yet implemented")
}

func (p *KeycloakProvider) getAdminToken(ctx context.Context) (string, error) {
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {p.adminClientID},
		"client_secret": {p.adminClientSecret},
	}

	r, err := http.NewRequestWithContext(ctx, "POST", p.tokenEndpoint(), strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("create admin token request: %w", err)
	}
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(r)
	if err != nil {
		return "", fmt.Errorf("keycloak admin token: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode admin token: %w", err)
	}
	return result.AccessToken, nil
}
