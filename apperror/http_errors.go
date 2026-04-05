package apperror

import (
	"errors"
	"net/http"
)

// StatusCode maps a domain error to the appropriate HTTP status code.
// Unknown errors map to 500 Internal Server Error.
func StatusCode(err error) int {
	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrConflict):
		return http.StatusConflict
	case errors.Is(err, ErrValidation):
		return http.StatusUnprocessableEntity
	case errors.Is(err, ErrRateLimited):
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}
