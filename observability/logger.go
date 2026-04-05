package observability

import (
	"log/slog"
	"os"
)

// NewLogger returns a configured slog.Logger appropriate for the given environment.
// In development: human-readable text output to stdout.
// In production: JSON structured output (for OTel log bridge integration).
func NewLogger(env string) *slog.Logger {
	var handler slog.Handler
	switch env {
	case "production", "staging":
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	default:
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	}
	return slog.New(handler)
}
