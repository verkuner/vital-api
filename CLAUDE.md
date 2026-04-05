# Vital Signs API

Go backend REST API for the Vital Signs monitoring mobile application.

## Project Overview

- **Service Name**: vital-api
- **Language**: Go 1.24+
- **Framework**: Encore (local/dev), self-hosted Docker (production)
- **Database**: Encore-managed PostgreSQL (local/dev), standalone PostgreSQL 18 + TimescaleDB 2.25 (production)
- **Cache**: Redis (session cache, rate limiting, pub/sub for real-time)
- **Auth**: Supabase Auth or Clerk as Vault (local/dev), Keycloak as Vault (production); Encore auth handler as Gatekeeper (all environments)
- **Real-time**: WebSocket (vital signs streaming via gorilla/websocket)
- **API Style**: RESTful JSON, versioned (/api/v1)
- **Observability**: Encore built-in tracing + dev dashboard (local/dev), OpenTelemetry -> OTel Collector -> SigNoz (production)

## Environment Strategy

The project uses a **dual-environment architecture**:

| Concern | Local / Dev | Production |
|---------|-------------|------------|
| **Platform** | Encore (`encore run`) | Self-hosted Docker / K8s |
| **Database** | Encore auto-provisioned PostgreSQL | Standalone PostgreSQL 18 + TimescaleDB 2.25 |
| **Auth (Vault)** | Supabase Auth or Clerk (managed, issues JWTs) | Standalone Keycloak (issues JWTs) |
| **Auth (Gatekeeper)** | Encore `//encore:authhandler` (validates JWTs) | Encore `//encore:authhandler` (same code) |
| **Tracing** | Encore built-in distributed tracing | OTel SDK -> OTel Collector -> SigNoz |
| **Metrics** | Encore dev dashboard (localhost:9400) | OTel SDK -> OTel Collector -> SigNoz |
| **Logs** | Encore structured logging to stdout | slog -> OTel log bridge -> SigNoz |
| **Hot reload** | `encore run` (automatic) | N/A |
| **API docs** | Encore Service Catalog + API Explorer | Swagger/OpenAPI |

Encore handles infrastructure provisioning, API routing, tracing, and database management for local and dev environments. Production uses standalone services with full control over infrastructure.

## Tech Stack

- **Platform**: Encore (local/dev), Docker + K8s (production)
- **API Framework**: Encore `//encore:api` annotations (replaces manual router setup)
- **Database**: Encore `sqldb` primitives (local/dev) + pgx/sqlc (production queries)
- **Migrations**: Encore built-in migrations (local/dev), golang-migrate (production)
- **Validation**: go-playground/validator
- **Logging**: slog (stdlib structured logging)
- **Config**: Encore secrets + config (local/dev), envconfig (production)
- **Testing**: stdlib testing + testify + `encore test` (local/dev), testcontainers-go (integration)
- **WebSocket**: gorilla/websocket
- **JWT**: golang-jwt/jwt/v5 (provider-agnostic token validation in Encore auth handler)
- **Observability**: Encore built-in (local/dev), go.opentelemetry.io/otel (production -> SigNoz)
- **API Docs**: Encore Service Catalog (local/dev), swaggo/swag (production)
- **Containerization**: `encore build docker` (production images)

## Project Structure

Encore service-based layout. Each feature (auth, vital, alert, user) is an Encore service with `//encore:api` endpoints, plus a service layer and repository layer. Shared infrastructure lives in top-level packages.

```
vital-api/
  encore.app                # Encore app configuration
  encore.gen.go             # Encore generated code (do not edit)

  # ---- Feature services (Encore services) ----
  auth/
    auth.go                 # Encore API endpoints (//encore:api annotations)
    service.go              # Auth business logic
    repository.go           # Auth DB operations (user lookup, session)
    model.go                # LoginRequest, TokenResponse, MfaChallenge
    auth_test.go
    service_test.go
  vital/
    vital.go                # Encore API endpoints (//encore:api annotations)
    service.go              # Vitals business logic, threshold checking
    repository.go           # Vital readings DB operations
    db.go                   # Encore sqldb.NewDatabase declaration
    migrations/             # SQL migration files (Encore-managed)
      1_create_vitals.up.sql
    model.go                # VitalReading, VitalType, VitalSummary
    vital_test.go
    service_test.go
    repository_test.go
  user/
    user.go                 # Encore API endpoints
    service.go              # User business logic
    repository.go           # User DB operations
    model.go                # User, UpdateProfileRequest
    user_test.go
    service_test.go
  alert/
    alert.go                # Encore API endpoints
    service.go              # Alert generation and management
    repository.go           # Alert DB operations
    model.go                # Alert, AlertThreshold
    alert_test.go
    service_test.go
  websocket/
    websocket.go            # WebSocket upgrade + connection handler
    hub.go                  # Connection hub (broadcast, per-user routing)
    client.go               # Single WebSocket client connection
    message.go              # Message types (subscribe, vital_reading)

  # ---- Shared infrastructure ----
  authhandler/
    authhandler.go          # Encore //encore:authhandler (validates JWTs from any OIDC provider)
  authprovider/
    provider.go             # AuthProvider interface (Register, Login, ResetPassword, MFA)
    supabase.go             # Supabase Auth implementation (local/dev)
    clerk.go                # Clerk implementation (local/dev alternative)
    keycloak.go             # Keycloak Admin REST API implementation (production)
  middleware/
    ratelimit.go            # Rate limiting middleware (Encore middleware)
    cors.go                 # CORS configuration
  observability/
    otel.go                 # OTel setup (production only — Encore handles local/dev)
    meter.go                # Custom metrics (production OTel export)
    logger.go               # slog config
  database/
    postgres.go             # pgx connection pool (production standalone DB)
    redis.go                # Redis client setup
  apperror/
    errors.go               # Domain error types
    http_errors.go          # Error-to-HTTP-status mapping

  db/
    queries/                # sqlc query files (for production standalone DB)
      auth.sql
      users.sql
      vitals.sql
      alerts.sql
    sqlc.yaml               # sqlc configuration
  deployments/
    docker/
      Dockerfile            # Built via `encore build docker`
      docker-compose.yml    # Production: Postgres + TimescaleDB + Redis + Keycloak + SigNoz + OTel Collector
      otel-collector-config.yaml  # OTel Collector pipeline (production only)
    k8s/                    # Kubernetes manifests (production)
  scripts/
    migrate.sh              # Migration helper (production standalone DB)
    seed.sh                 # Dev data seeding
  go.mod
  go.sum
  Makefile
  CLAUDE.md
```

## Code Conventions

### Go Style

- Follow [Effective Go](https://go.dev/doc/effective_go) and Go Code Review Comments.
- Use `gofmt` / `goimports` for formatting. Zero formatting issues.
- **Naming**: `PascalCase` for exported, `camelCase` for unexported, `snake_case` for files.
- **Error Handling**: Always handle errors. Never use `_` for error returns. Wrap errors with `fmt.Errorf("context: %w", err)`.
- **Context**: Pass `context.Context` as the first parameter to all functions that do I/O.
- **Interfaces**: Define interfaces where they are used (consumer side), not where they are implemented.
- **Struct Tags**: Use `json:"field_name"` for API models, `db:"column_name"` for DB models.

### Package Rules

- Each feature is an **Encore service** (`auth/`, `vital/`, `user/`, `alert/`) — a Go package with `//encore:api` endpoints.
- `{service}.go` - Encore API endpoints. Parses requests, calls service, returns responses. No business logic.
- `service.go` - Business logic. Depends on repository interface (defined in same package). No HTTP concerns.
- `repository.go` - Database access only. Returns domain models. No business logic.
- `db.go` - Encore `sqldb.NewDatabase` declaration (if service owns a database).
- `model.go` - Domain models and DTOs for this feature. No methods with side effects.
- Shared infra packages (`authhandler/`, `authprovider/`, `database/`, `middleware/`, `observability/`, `apperror/`) are used across features.
- Features should NOT import other feature packages directly. Cross-feature communication goes through Encore service-to-service calls or interfaces.

### Error Handling Pattern

```go
// Domain errors in apperror/
var (
    ErrNotFound       = errors.New("resource not found")
    ErrUnauthorized   = errors.New("unauthorized")
    ErrForbidden      = errors.New("forbidden")
    ErrConflict       = errors.New("resource conflict")
    ErrValidation     = errors.New("validation error")
    ErrRateLimited    = errors.New("rate limit exceeded")
)

// Wrap with context
func (s *VitalService) GetByID(ctx context.Context, id uuid.UUID) (*model.Vital, error) {
    vital, err := s.repo.FindByID(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("get vital %s: %w", id, err)
    }
    if vital == nil {
        return nil, fmt.Errorf("vital %s: %w", id, apperror.ErrNotFound)
    }
    return vital, nil
}

// Handler maps errors to HTTP status
func mapError(err error) int {
    switch {
    case errors.Is(err, apperror.ErrNotFound):
        return http.StatusNotFound
    case errors.Is(err, apperror.ErrUnauthorized):
        return http.StatusUnauthorized
    // ...
    }
}
```

### Dependency Injection

Constructor injection, no globals, no init() side effects. Encore services wire dependencies in package-level `init` or via Encore's service struct pattern.

```go
// vital/service.go
type Repository interface {
    FindByID(ctx context.Context, id uuid.UUID) (*VitalReading, error)
    Create(ctx context.Context, reading *VitalReading) error
}

type Service struct {
    repo   Repository
    cache  database.Cache
    logger *slog.Logger
}

func NewService(repo Repository, cache database.Cache, logger *slog.Logger) *Service {
    return &Service{repo: repo, cache: cache, logger: logger}
}
```

```go
// vital/vital.go — Encore service initialization
//encore:service
type VitalService struct {
    svc *Service
}

func initVitalService() (*VitalService, error) {
    repo := NewRepository(db)
    svc := NewService(repo, redisCache, slog.Default())
    return &VitalService{svc: svc}, nil
}
```

### Middleware Chain

Encore handles recovery, request ID, and logging automatically. Custom middleware is registered via Encore's middleware API:

```
Request
  --> Encore built-in (recovery, request ID, logging, tracing)
  --> CORS (Encore middleware)
  --> Rate Limiting (Encore middleware, Redis-backed)
  --> //encore:authhandler (for auth-required endpoints)
  --> //encore:api handler
```

## Database

### Environment-Specific Database Strategy

| Concern | Local / Dev (Encore) | Production (Standalone) |
|---------|---------------------|------------------------|
| **Provisioning** | `encore run` auto-provisions PostgreSQL | Manual / IaC (Terraform, etc.) |
| **Extension** | Standard PostgreSQL | PostgreSQL 18 + TimescaleDB 2.25 |
| **Migrations** | Encore built-in migration runner | golang-migrate |
| **Connection** | Encore `sqldb` primitives | pgx connection pool |
| **Queries** | Encore `sqldb.Query` / sqlc | sqlc generated code via pgx |

### Local/Dev (Encore-Managed)

Encore automatically provisions a PostgreSQL instance when running `encore run`. Declare databases in each service:

```go
// vital/db.go
var db = sqldb.NewDatabase("vitals", sqldb.DatabaseConfig{
    Migrations: "./migrations",
})
```

Migrations live inside each service directory (e.g. `vital/migrations/`). Encore runs them automatically on startup.

**Note**: Encore's local PostgreSQL does not include TimescaleDB. Time-series aggregations (`time_bucket`, continuous aggregates) are only available in production. Local/dev uses standard PostgreSQL `date_trunc` or application-level aggregation as a fallback.

### Production (Standalone PostgreSQL + TimescaleDB)

- TimescaleDB hypertable for `vital_readings` (optimized time-series queries).
- Standard tables for users, alerts, thresholds.
- Use `pgx` connection pool with sensible limits (max 25 connections).
- All queries via `sqlc` for compile-time type safety.
- Connect Encore's built Docker image to standalone Postgres via infrastructure config.

### Migration Rules

- **Local/Dev**: Encore manages migrations. Files are numbered sequentially: `1_description.up.sql`.
- **Production**: Migrations are sequential, numbered: `000001_description.up.sql`, `000001_description.down.sql`.
- Every production `up` migration MUST have a corresponding `down` migration.
- Never modify existing migrations. Create new ones for changes.
- Test both up and down migrations.

### Schema Overview

```sql
-- Users (identity managed by auth provider, cached locally)
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id     VARCHAR(255) NOT NULL UNIQUE,  -- Supabase/Clerk user ID (local/dev) or Keycloak sub (prod)
    email           VARCHAR(255) NOT NULL UNIQUE,
    name            VARCHAR(255) NOT NULL,
    date_of_birth   DATE,
    avatar_url      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Vital Readings (TimescaleDB hypertable)
CREATE TABLE vital_readings (
    id              UUID DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    vital_type      VARCHAR(50) NOT NULL,
    value           DOUBLE PRECISION NOT NULL,
    unit            VARCHAR(20) NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'normal',
    device_id       UUID,
    notes           TEXT,
    measured_at     TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, measured_at)
);

SELECT create_hypertable('vital_readings', 'measured_at');

-- Alert Thresholds
CREATE TABLE alert_thresholds (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    vital_type      VARCHAR(50) NOT NULL,
    low_value       DOUBLE PRECISION,
    high_value      DOUBLE PRECISION,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    UNIQUE(user_id, vital_type)
);

-- Alerts
CREATE TABLE alerts (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    vital_type          VARCHAR(50) NOT NULL,
    value               DOUBLE PRECISION NOT NULL,
    threshold_breached  VARCHAR(10) NOT NULL,
    threshold_value     DOUBLE PRECISION NOT NULL,
    severity            VARCHAR(20) NOT NULL DEFAULT 'warning',
    acknowledged        BOOLEAN NOT NULL DEFAULT false,
    acknowledged_at     TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

## Auth Architecture

The auth system separates two concerns:

- **Vault** (identity provider): Stores user credentials, issues JWTs, handles registration/login/MFA.
- **Gatekeeper** (token validator): Encore `//encore:authhandler` validates JWTs from whichever Vault is configured.

### Environment-Specific Auth Strategy

| Concern | Local / Dev | Production |
|---------|-------------|------------|
| **Vault** | Supabase Auth or Clerk (managed SaaS) | Standalone Keycloak |
| **Gatekeeper** | Encore `//encore:authhandler` | Encore `//encore:authhandler` (same code) |
| **JWT issuer** | Supabase / Clerk | Keycloak |
| **JWKS endpoint** | Supabase / Clerk `.well-known/jwks.json` | Keycloak JWKS endpoint |
| **User management API** | Supabase Auth REST API / Clerk Backend API | Keycloak Admin REST API |
| **Self-hosted?** | No (managed SaaS — zero infra) | Yes (standalone Keycloak) |

### Gatekeeper — Encore Auth Handler

The auth handler is **provider-agnostic**. It validates JWTs from any OIDC-compliant issuer by fetching the JWKS from a configurable endpoint:

```go
// authhandler/authhandler.go
//encore:authhandler
func Authenticate(ctx context.Context, token string) (auth.UID, *AuthData, error) {
    // 1. Fetch JWKS from configured provider's well-known endpoint (cache with TTL)
    //    - Local/dev: Supabase/Clerk JWKS URL
    //    - Production: Keycloak JWKS URL
    // 2. Parse and validate JWT signature (RS256)
    // 3. Check claims: iss, aud, exp, iat
    // 4. Extract user info: sub, email, roles (claim paths vary by provider)
    // 5. Return auth.UID (provider's user ID) and AuthData
}
```

Endpoints annotated with `//encore:api auth` automatically require authentication through this handler.

### Vault — AuthProvider Interface

User management operations (register, login, password reset, MFA) are abstracted behind an `AuthProvider` interface, with implementations for each provider:

```go
// authprovider/provider.go
type AuthProvider interface {
    Register(ctx context.Context, req RegisterRequest) (*RegisterResult, error)
    Login(ctx context.Context, req LoginRequest) (*TokenPair, error)
    RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error)
    Logout(ctx context.Context, refreshToken string) error
    ForgotPassword(ctx context.Context, email string) error
    ResetPassword(ctx context.Context, req ResetPasswordRequest) error
    EnrollMFA(ctx context.Context, userID string) (*MFAEnrollment, error)
    VerifyMFA(ctx context.Context, userID, code string) error
    RemoveMFA(ctx context.Context, userID string) error
}
```

| Implementation | Package | Used In |
|---------------|---------|---------|
| Supabase Auth | `authprovider/supabase.go` | Local/dev (default) |
| Clerk | `authprovider/clerk.go` | Local/dev (alternative) |
| Keycloak | `authprovider/keycloak.go` | Production |

The active provider is selected at startup based on the `AUTH_PROVIDER` config value.

## Observability

See `docs/observability.md` for full details.

### Environment-Specific Observability

| Signal | Local / Dev (Encore) | Production |
|--------|---------------------|------------|
| **Traces** | Encore auto-instruments all API calls, DB queries, pub/sub — viewable in Encore dev dashboard (localhost:9400) | OTel SDK -> OTel Collector -> SigNoz |
| **Metrics** | Encore dev dashboard (request rates, latencies, error rates) | OTel custom counters/histograms -> SigNoz |
| **Logs** | Encore structured logging to stdout + dev dashboard | slog -> OTel log bridge -> SigNoz |
| **PHI Scrubbing** | Encore does not export data externally (local only) | OTel Collector `attributes/sanitize` processor |
| **Sampling** | 100% (Encore traces everything locally) | `parentbased_traceidratio` at 10% |

### Local/Dev (Encore Built-in)

Encore automatically instruments every API call, database query, and service-to-service call at compile time — **no OTel SDK, no manual span creation needed**. The Encore dev dashboard at `localhost:9400` provides:

- Distributed traces for all requests
- Database query timing and parameters
- API request/response inspection
- Service architecture diagram
- Real-time metrics (latency, error rate, throughput)

### Production (OTel + SigNoz)

Production uses the full OpenTelemetry pipeline:

```
Go App (OTel SDK) -> OTel Collector -> SigNoz (ClickHouse)
```

Instrumentation packages (production only):

```go
go.opentelemetry.io/otel
go.opentelemetry.io/otel/sdk
go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc
go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp
go.opentelemetry.io/contrib/instrumentation/github.com/jackc/pgx/v5/otelpgx
```

### Span Naming Convention (Production)

- HTTP: `{METHOD} {route_pattern}` (auto from otelhttp)
- Service: `{feature}.Service.{MethodName}` (e.g., `vital.Service.RecordVital`)
- Repository: `{feature}.Repository.{MethodName}` (e.g., `vital.Repository.Create`)
- External: `authprovider.{operation}` (e.g., `authprovider.Login`, `authprovider.ValidateToken`)

### Custom Metrics (Production)

| Metric | Type | Description |
|--------|------|-------------|
| `vitals.recorded.total` | Counter | Total vital readings recorded (by type) |
| `websocket.connections.active` | UpDownCounter | Active WebSocket connections |
| `api.request.duration_ms` | Histogram | API request latency |
| `authprovider.duration_ms` | Histogram | Auth provider call latency |
| `db.query.duration_ms` | Histogram | Database query latency |

## Environment Variables

### Local/Dev (Encore)

Encore manages most infrastructure config automatically. Only app-specific config is needed:

```bash
# Auth provider selection
AUTH_PROVIDER=supabase             # supabase | clerk | keycloak

# Supabase Auth (default for local/dev)
SUPABASE_URL=https://<project>.supabase.co
SUPABASE_ANON_KEY=eyJ...
SUPABASE_SERVICE_ROLE_KEY=eyJ...   # Encore secret: `encore secret set SupabaseServiceRoleKey`
JWKS_URL=https://<project>.supabase.co/auth/v1/.well-known/jwks.json

# Or Clerk (alternative for local/dev)
# CLERK_SECRET_KEY=sk_test_...     # Encore secret
# JWKS_URL=https://<clerk-domain>/.well-known/jwks.json

CORS_ALLOWED_ORIGINS=http://localhost:3000
```

Encore auto-provisions: PostgreSQL (DATABASE_URL), port assignment, service discovery, tracing endpoint.

### Production (Standalone)

```bash
# Server
PORT=5080
ENV=production

# Auth provider
AUTH_PROVIDER=keycloak

# Keycloak (standalone — production)
KEYCLOAK_URL=https://keycloak.example.com
KEYCLOAK_REALM=vital-signs
KEYCLOAK_CLIENT_ID=vital-mobile-app
KEYCLOAK_ADMIN_CLIENT_ID=vital-api-admin
KEYCLOAK_ADMIN_CLIENT_SECRET=<from-vault>
JWKS_URL=https://keycloak.example.com/realms/vital-signs/protocol/openid-connect/certs

# Database (standalone PostgreSQL + TimescaleDB)
DATABASE_URL=postgres://user:pass@db-host:5432/vital_signs?sslmode=require
DATABASE_MAX_CONNS=25

# Redis
REDIS_URL=redis://redis-host:6379/0

# CORS
CORS_ALLOWED_ORIGINS=https://app.example.com

# Observability (OpenTelemetry — production only)
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317
OTEL_SERVICE_NAME=vital-api
OTEL_TRACES_SAMPLER=parentbased_traceidratio
OTEL_TRACES_SAMPLER_ARG=0.1

# Rate Limiting
RATE_LIMIT_AUTH=10     # per minute per IP
RATE_LIMIT_GENERAL=100 # per minute per user
```

## Build & Run

### Local/Dev (Encore)

```bash
# Start API + all infrastructure (Postgres, tracing, dashboard)
encore run                  # http://localhost:4000, dashboard at localhost:9400

# Testing
encore test ./...           # run all tests with Encore test runner

# Database
# Migrations run automatically on `encore run`
# Create new migration in the service's migrations/ directory

# Generate sqlc code (for production query compatibility)
make sqlc

# Seed dev data
make seed
```

### Production

```bash
# Build Docker image via Encore
encore build docker vital-api:latest

# Or build standalone
make build                  # go build -o bin/api cmd/api/main.go

# Docker Compose (production infra)
make docker-up              # Production infra: Postgres+TimescaleDB + Redis + Keycloak + SigNoz + OTel Collector
make docker-down            # Stop all services

# Database (production standalone)
make migrate-up             # Run migrations via golang-migrate
make migrate-down           # Rollback last migration
make migrate-create NAME=x  # Create new migration
```

### Common

```bash
# Linting
make lint                   # golangci-lint run
make fmt                    # goimports -w .

# Testing
make test                   # go test ./...
make test-cover             # With coverage report
make test-integration       # Integration tests (requires Docker)
```

## Testing

- **Unit Tests**: Test services with mocked repositories. Test handlers with mocked services.
- **Integration Tests**: Use `testcontainers-go` for PostgreSQL/Redis. Test full request lifecycle.
- **Table-Driven Tests**: Use table-driven tests for input validation and edge cases.
- **Naming**: `TestFunctionName_Scenario_ExpectedResult`
- **Coverage**: Minimum 80% for services, 70% overall.

```go
// vital/service_test.go
func TestService_RecordVital_ValidReading(t *testing.T) {
    // Arrange
    repo := NewMockRepository(t)  // mock defined in same package
    svc := NewService(repo, nil, slog.Default())

    // Act
    result, err := svc.RecordVital(ctx, input)

    // Assert
    require.NoError(t, err)
    assert.Equal(t, "heart_rate", result.VitalType)
}
```

## API Versioning

- All routes under `/api/v1/`.
- When breaking changes needed, create `/api/v2/` alongside v1.
- Deprecate v1 with `Sunset` header before removal.

## Git Conventions

- **Branch Naming**: `feature/<ticket>-description`, `bugfix/<ticket>-description`
- **Commit Messages**: Conventional Commits: `feat(vitals): add real-time WebSocket streaming`
- **PR Requirements**: All tests pass, lint clean, at least 1 approval.

## Key Decisions

- **Encore for local/dev**: Zero-config infrastructure, built-in tracing/metrics, rapid iteration. Avoids Docker Compose sprawl for development.
- **Standalone infra for production**: Full control over PostgreSQL + TimescaleDB, Keycloak, and observability stack. No vendor lock-in for production deployment.
- **Supabase Auth / Clerk for local/dev, Keycloak for production**: Managed SaaS auth in dev avoids self-hosting Keycloak locally. Keycloak in production gives full control. The `AuthProvider` interface abstracts provider differences.
- **Encore auth handler as provider-agnostic Gatekeeper**: Single auth handler validates JWTs from any OIDC-compliant issuer via configurable JWKS URL. Same code in all environments.
- **Encore tracing for local/dev, OTel + SigNoz for production**: Encore's zero-config tracing eliminates instrumentation boilerplate during development. Production uses industry-standard OTel pipeline for full observability.
- **sqlc over GORM**: Compile-time type safety, no ORM magic, raw SQL performance. Works with both Encore's `sqldb` and standalone pgx.
- **TimescaleDB (production only)**: Purpose-built for time-series vital readings queries and aggregations. Local/dev uses standard PostgreSQL with application-level aggregation fallbacks.
- **slog over zerolog/zap**: Stdlib structured logging, zero external dependencies for logging. Works with both Encore's log capture and OTel log bridge.
- **Separate from mobile repo**: Independent deployment, CI/CD, versioning.
