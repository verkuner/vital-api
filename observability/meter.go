package observability

import (
	"context"
	"log/slog"
	"sync/atomic"
)

// Custom metric counters for the vital-api.
// In production these are backed by OTel SDK instruments.
// In local/dev (Encore), Encore's built-in metrics handle everything.

var (
	vitalsRecordedTotal  atomic.Int64
	alertsGeneratedTotal atomic.Int64
	wsConnectionsActive  atomic.Int64
)

// RecordVitalCreated increments the vitals.recorded.total counter.
func RecordVitalCreated(ctx context.Context, vitalType string) {
	vitalsRecordedTotal.Add(1)
	slog.DebugContext(ctx, "metric: vital recorded", slog.String("vital_type", vitalType))
}

// RecordAlertGenerated increments the alerts.generated.total counter.
func RecordAlertGenerated(ctx context.Context, severity, vitalType string) {
	alertsGeneratedTotal.Add(1)
	slog.DebugContext(ctx, "metric: alert generated",
		slog.String("severity", severity),
		slog.String("vital_type", vitalType),
	)
}

// WSConnectionOpened increments the active WebSocket connections gauge.
func WSConnectionOpened() {
	wsConnectionsActive.Add(1)
}

// WSConnectionClosed decrements the active WebSocket connections gauge.
func WSConnectionClosed() {
	wsConnectionsActive.Add(-1)
}

// ActiveWSConnections returns the current count of active WebSocket connections.
func ActiveWSConnections() int64 {
	return wsConnectionsActive.Load()
}
