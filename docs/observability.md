# Observability

Instrumentation strategy, signal types, and operational runbook for the Vital Signs API.

## Environment Strategy

The observability stack differs by environment:

| Signal | Local / Dev (Encore) | Production (Standalone) |
|--------|---------------------|------------------------|
| **Traces** | Encore auto-instruments at compile time | OTel SDK -> OTel Collector -> SigNoz |
| **Metrics** | Encore dev dashboard | OTel SDK -> OTel Collector -> SigNoz |
| **Logs** | Encore captures slog output | slog -> OTel log bridge -> SigNoz |
| **PHI scrubbing** | N/A (data stays local) | OTel Collector `attributes/sanitize` processor |
| **Dashboard** | Encore dev dashboard (localhost:9400) | SigNoz UI |

## Local/Dev Stack (Encore)

```
Go App (Encore framework)
    │
    └── Encore dev dashboard (localhost:9400)
        ├── Distributed traces (auto-instrumented)
        ├── Metrics (latency, throughput, errors)
        ├── Logs (structured slog output)
        └── Service architecture diagram
```

Encore **automatically instruments** every API call, database query, and service-to-service call at compile time. No OTel SDK, no manual span creation, no Collector setup needed for local development.

### What Encore traces automatically:
- All `//encore:api` endpoint calls with request/response data
- SQL queries with execution time and parameters
- Service-to-service calls (correlated across services)
- Pub/Sub messages

### Viewing traces locally:
```bash
encore run          # starts API + dashboard
open http://localhost:9400   # dev dashboard with traces, metrics, architecture
```

## Production Stack (OTel + SigNoz)

```
Go App (OTel SDK)
    │
    ▼
OTel Collector  (receives, processes, exports)
    │
    ├── Traces  ──► SigNoz (ClickHouse backend)
    ├── Metrics ──► SigNoz
    └── Logs    ──► SigNoz
```

| Component | Role |
|-----------|------|
| `go.opentelemetry.io/otel` | SDK — create spans, meters, log records |
| `otelhttp` | Auto-instrument HTTP handlers |
| `otelpgx` | Auto-instrument pgx database calls |
| OTel Collector | Receive OTLP, scrub PHI, batch-export |
| SigNoz | Unified trace/metric/log backend (self-hosted) |

## Production Initialization

`observability/` bootstraps all three signal pipelines when running in production mode (not Encore local/dev).

```go
// observability/otel.go
func Init(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
    // Only initialize OTel in production — Encore handles observability locally
    if cfg.Environment == "development" {
        return func(context.Context) error { return nil }, nil
    }

    // Production: full OTel pipeline
    // ... TracerProvider, MeterProvider, LoggerProvider setup
}
```

`Init` returns a single `shutdown` function that flushes all pending signals and closes exporters on graceful shutdown.

## Traces

### Local/Dev (Encore Auto-Instrumentation)

In local/dev, Encore handles all trace instrumentation at compile time. No manual code needed — every `//encore:api` call and `sqldb` query is traced automatically and visible in the dev dashboard.

### Production Auto-Instrumentation

HTTP spans are created by wrapping handlers with `otelhttp`:

```go
// observability/otel.go (production only)
handler := otelhttp.NewHandler(router, "vital-api",
    otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
    otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
        return r.Method + " " + r.URL.Path
    }),
)
```

Database spans are created automatically via `otelpgx`:

```go
// database/postgres.go (production standalone connection)
config.ConnConfig.Tracer = otelpgx.NewTracer()
```

### Manual Spans (Production Only)

Add spans for non-trivial business logic in service methods. These are only active in production — Encore traces services automatically in local/dev:

```go
// vital/service.go
func (s *Service) RecordVital(ctx context.Context, userID uuid.UUID, req RecordVitalRequest) (*VitalReading, error) {
    ctx, span := otel.Tracer("vital").Start(ctx, "vital.Service.RecordVital")
    defer span.End()

    span.SetAttributes(
        attribute.String("vital.type", req.VitalType),
        attribute.String("user.id", userID.String()),
        // NOTE: do NOT record req.Value — PHI
    )

    reading, err := s.repo.Create(ctx, reading)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return nil, fmt.Errorf("record vital: %w", err)
    }

    span.SetStatus(codes.Ok, "")
    return reading, nil
}
```

### Span Naming Convention

| Layer | Pattern | Example |
|-------|---------|---------|
| HTTP | `{METHOD} {route_pattern}` | `POST /api/v1/vitals` |
| Service | `{feature}.Service.{Method}` | `vital.Service.RecordVital` |
| Repository | `{feature}.Repository.{Method}` | `vital.Repository.Create` |
| External | `authprovider.{operation}` | `authprovider.Login` |
| WebSocket | `websocket.{operation}` | `websocket.BroadcastVital` |

### Span Attributes

Standard attributes set on all spans via OTel semantic conventions:

```go
// HTTP spans (auto via otelhttp)
semconv.HTTPMethodKey.String("POST")
semconv.HTTPRouteKey.String("/api/v1/vitals")
semconv.HTTPStatusCodeKey.Int(201)
semconv.HTTPUserAgentKey.String("VitalApp/1.0")

// DB spans (auto via otelpgx)
semconv.DBSystemPostgreSQL
semconv.DBNameKey.String("vital_signs")
semconv.DBOperationKey.String("SELECT")
// NOTE: db.statement is DELETED by OTel Collector (may contain PHI)

// Custom
attribute.String("user.id", userID.String())   // UUID only, never email
attribute.String("vital.type", "heart_rate")    // type only, never value
attribute.String("alert.severity", "critical")
```

## Metrics

### Custom Metrics (internal/observability/meter.go)

```go
var (
    VitalsRecordedTotal    metric.Int64Counter
    WSConnectionsActive    metric.Int64UpDownCounter
    APIRequestDuration     metric.Float64Histogram
    AuthProviderDuration   metric.Float64Histogram
    DBQueryDuration        metric.Float64Histogram
    AlertsGeneratedTotal   metric.Int64Counter
)

func initMetrics(meter metric.Meter) error {
    var err error
    VitalsRecordedTotal, err = meter.Int64Counter(
        "vitals.recorded.total",
        metric.WithDescription("Total vital readings recorded"),
        metric.WithUnit("{reading}"),
    )
    // ... register all metrics
    return err
}
```

### Recording Metrics

```go
// In service after successful creation:
observability.VitalsRecordedTotal.Add(ctx, 1,
    metric.WithAttributes(attribute.String("vital_type", req.VitalType)),
)

// In WebSocket hub:
observability.WSConnectionsActive.Add(ctx, 1,
    metric.WithAttributes(attribute.String("user_id", userID.String())),
)
// On disconnect:
observability.WSConnectionsActive.Add(ctx, -1, ...)
```

### Metric Inventory

| Metric Name | Type | Labels | Description |
|-------------|------|--------|-------------|
| `vitals.recorded.total` | Counter | `vital_type` | Readings recorded |
| `websocket.connections.active` | UpDownCounter | — | Open WS connections |
| `api.request.duration_ms` | Histogram | `route`, `method`, `status` | HTTP latency |
| `authprovider.duration_ms` | Histogram | `operation`, `provider` | Auth provider call latency |
| `db.query.duration_ms` | Histogram | `operation` | DB query latency |
| `alerts.generated.total` | Counter | `severity`, `vital_type` | Alerts triggered |
| `cache.hits.total` | Counter | `cache`, `operation` | Redis cache hits |
| `cache.misses.total` | Counter | `cache`, `operation` | Redis cache misses |

Go runtime metrics (goroutines, GC, memory) are exported automatically via `otel/sdk`.

## Logs

Structured logging via `slog` bridged to OTel:

```go
// internal/observability/logger.go
func NewLogger(level slog.Level) *slog.Logger {
    // In production: bridge slog -> OTel log exporter -> SigNoz
    // In development: human-readable text output to stdout
    handler := otelslog.NewHandler("vital-api",
        otelslog.WithLoggerProvider(global.GetLoggerProvider()),
    )
    return slog.New(handler)
}
```

### Log Levels

| Level | When |
|-------|------|
| `ERROR` | Unexpected errors that require investigation |
| `WARN` | Expected errors (rate limit hit, not-found) |
| `INFO` | Significant lifecycle events (server start, migration run) |
| `DEBUG` | Request/response detail, only in development |

### Log Fields

Always include in request-scoped logs:

```go
logger.InfoContext(ctx, "vital recorded",
    slog.String("user_id", userID.String()),
    slog.String("vital_type", req.VitalType),
    slog.String("request_id", middleware.RequestIDFromContext(ctx)),
    // Do NOT log: vital value, user email, JWT, passwords
)
```

Trace correlation: OTel SDK automatically injects `trace_id` and `span_id` into log records when a span is active in the context.

## Sampling

```bash
# Development: capture every request
OTEL_TRACES_SAMPLER=parentbased_traceidratio
OTEL_TRACES_SAMPLER_ARG=1.0

# Production: 10% of traces (head-based)
OTEL_TRACES_SAMPLER_ARG=0.1
```

Always-sample rules (applied via OTel Collector `sampling` processor):
- Any span with `error=true`
- Any span with HTTP status >= 500
- Any span from `POST /auth/*`

## OTel Collector Configuration

```yaml
# deployments/docker/otel-collector-config.yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

processors:
  batch:
    timeout: 5s
    send_batch_size: 512
  memory_limiter:
    check_interval: 1s
    limit_mib: 256
  attributes/sanitize:
    actions:
      - key: db.statement
        action: delete
      - key: vital.value
        action: delete
      - key: http.request.body
        action: delete
      - key: user.email
        action: hash

exporters:
  otlp/signoz:
    endpoint: signoz-otel-collector:4317
    tls:
      insecure: true  # internal network; use TLS in prod

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, attributes/sanitize, batch]
      exporters: [otlp/signoz]
    metrics:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlp/signoz]
    logs:
      receivers: [otlp]
      processors: [memory_limiter, attributes/sanitize, batch]
      exporters: [otlp/signoz]
```

## SigNoz Dashboards

Recommended dashboards to create in SigNoz:

1. **API Overview**: P50/P95/P99 latency, error rate, request rate by route
2. **Vitals Pipeline**: `vitals.recorded.total` by type over time, recording errors
3. **WebSocket**: Active connections, message throughput, disconnect rate
4. **Database**: Query latency histogram, slow query alerts (> 100ms)
5. **Auth**: Auth provider latency (Supabase/Clerk/Keycloak), token validation failures, rate limit triggers

## Alerting

Configure SigNoz alerts for:

| Alert | Condition | Severity |
|-------|-----------|----------|
| High error rate | `error_rate > 1%` over 5m | Critical |
| P99 latency | `p99 > 2000ms` over 5m | Warning |
| No vitals recorded | `vitals.recorded.total` = 0 for 10m (business hours) | Warning |
| DB query slow | `db.query.duration_ms p95 > 500ms` | Warning |
| OTel Collector down | No data received for 2m | Critical |

## Local Development (Encore)

```bash
# Start API + all infrastructure + observability
encore run

# Encore dev dashboard (traces, metrics, architecture, API explorer)
open http://localhost:9400

# No OTel Collector, no SigNoz needed for local development
```

Encore traces every request automatically — click any request in the dashboard to see the full trace including database queries, service calls, and errors.

## Production Observability Setup

```bash
# Start the full production observability stack
make docker-up   # includes SigNoz + OTel Collector

# SigNoz UI
open http://localhost:3301

# OTel Collector health check
curl http://localhost:13133/
```

Set `OTEL_TRACES_SAMPLER_ARG=0.1` for 10% sampling in production.
