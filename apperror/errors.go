package apperror

import "errors"

// Sentinel errors used throughout the application.
// Wrap these with fmt.Errorf("context: %w", ErrXxx) to add context while
// preserving errors.Is compatibility.
var (
	ErrNotFound     = errors.New("resource not found")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrConflict     = errors.New("resource conflict")
	ErrValidation   = errors.New("validation error")
	ErrRateLimited  = errors.New("rate limit exceeded")
)

// ErrorCode returns a machine-readable string code for the given error,
// suitable for use in JSON error responses.
func ErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrNotFound):
		return "NOT_FOUND"
	case errors.Is(err, ErrUnauthorized):
		return "UNAUTHORIZED"
	case errors.Is(err, ErrForbidden):
		return "FORBIDDEN"
	case errors.Is(err, ErrConflict):
		return "CONFLICT"
	case errors.Is(err, ErrValidation):
		return "VALIDATION_ERROR"
	case errors.Is(err, ErrRateLimited):
		return "RATE_LIMITED"
	default:
		return "INTERNAL_ERROR"
	}
}
