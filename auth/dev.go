package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type CreateTestUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type CreateTestUserResponse struct {
	Message string `json:"message"`
	UserID  string `json:"user_id"`
	Email   string `json:"email"`
}

// CreateTestUser creates a pre-confirmed user via Supabase Admin API.
// Dev/local only — bypasses email confirmation for testing.
//
//encore:api public method=POST path=/dev/create-test-user
func (s *AuthService) CreateTestUser(ctx context.Context, req *CreateTestUserRequest) (*CreateTestUserResponse, error) {
	body := map[string]interface{}{
		"email":         req.Email,
		"password":      req.Password,
		"email_confirm": true,
		"user_metadata":  map[string]string{"name": req.Name},
	}
	data, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		secrets.SupabaseURL+"/auth/v1/admin/users", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("apikey", secrets.SupabaseServiceRoleKey)
	httpReq.Header.Set("Authorization", "Bearer "+secrets.SupabaseServiceRoleKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("supabase admin: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("supabase admin returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &CreateTestUserResponse{
		Message: "test user created (email pre-confirmed)",
		UserID:  result.ID,
		Email:   result.Email,
	}, nil
}
