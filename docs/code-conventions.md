# Code Conventions

Go style rules, patterns, and idioms used throughout the Vital Signs API.

## Formatting

All code must be formatted with `goimports` (superset of `gofmt` that also manages imports):

```bash
goimports -w .          # format all files
make fmt                # alias

# CI enforces this:
test -z "$(goimports -l .)" || (echo "unformatted files" && exit 1)
```

**Zero tolerance** for formatting issues in PRs.

## Naming

| Kind | Convention | Example |
|------|-----------|---------|
| Packages | lowercase, single word | `vital`, `auth`, `apperror` |
| Files | `snake_case` | `handler.go`, `service_test.go` |
| Exported types | `PascalCase` | `VitalReading`, `Service` |
| Unexported | `camelCase` | `userID`, `parseRequest` |
| Interfaces | noun or adjective | `Repository`, `Cache`, `Validator` |
| Constructors | `New{Type}` | `NewService`, `NewHandler` |
| Constants | `PascalCase` | `DefaultMaxConns` |
| Error vars | `Err{Description}` | `ErrNotFound`, `ErrUnauthorized` |
| Test helpers | `must{Action}` | `mustCreateUser` |

### Interface Naming

Prefer the noun form, not `-er` for multi-method interfaces:

```go
// Good
type Repository interface { ... }
type Cache interface { ... }

// Only use -er for single-method interfaces
type Closer interface { Close() error }
type Stringer interface { String() string }
```

## Package Structure Rules

Each feature is an **Encore service** — a Go package with `//encore:api` endpoints. Each service contains:

| File | Responsibility |
|------|---------------|
| `{service}.go` | Encore API endpoints (`//encore:api` annotations). Parses requests, calls service, returns responses. No business logic. |
| `service.go` | Business logic. Depends on `Repository` interface. No HTTP. |
| `repository.go` | Database access. Returns domain models. No business logic. |
| `db.go` | Encore `sqldb.NewDatabase` declaration (if service owns a database). |
| `model.go` | Structs, DTOs, constants. No methods with side effects. |
| `migrations/` | SQL migration files (Encore-managed). |
| `*_test.go` | Tests co-located with tested code. |

Cross-feature communication uses Encore's service-to-service calls or interfaces wired at initialization — feature packages never import each other directly.

## Error Handling

### Always Handle Errors

```go
// WRONG: silently ignoring error
result, _ := svc.GetVital(ctx, id)

// CORRECT
result, err := svc.GetVital(ctx, id)
if err != nil {
    return nil, fmt.Errorf("get vital: %w", err)
}
```

### Wrapping Errors

Wrap errors with context at each layer boundary:

```go
// repository.go
func (r *repository) FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*VitalReading, error) {
    row, err := r.q.GetVitalReadingByID(ctx, dbgen.GetVitalReadingByIDParams{ID: id, UserID: userID})
    if errors.Is(err, pgx.ErrNoRows) {
        return nil, nil  // not found = nil, nil (let service decide semantics)
    }
    if err != nil {
        return nil, fmt.Errorf("query vital reading %s: %w", id, err)
    }
    return mapRow(row), nil
}

// service.go
func (s *Service) GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*VitalReading, error) {
    reading, err := s.repo.FindByID(ctx, id, userID)
    if err != nil {
        return nil, fmt.Errorf("vital.Service.GetByID: %w", err)
    }
    if reading == nil {
        return nil, fmt.Errorf("vital %s: %w", id, apperror.ErrNotFound)
    }
    return reading, nil
}
```

### Sentinel Errors

Define domain errors in `internal/apperror/errors.go`:

```go
var (
    ErrNotFound     = errors.New("resource not found")
    ErrUnauthorized = errors.New("unauthorized")
    ErrForbidden    = errors.New("forbidden")
    ErrConflict     = errors.New("resource conflict")
    ErrValidation   = errors.New("validation error")
    ErrRateLimited  = errors.New("rate limit exceeded")
)
```

Use `errors.Is` for checking, not string comparison:

```go
if errors.Is(err, apperror.ErrNotFound) {
    // ...
}
```

### HTTP Error Mapping

```go
// internal/apperror/http_errors.go
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
```

## Context

Pass `context.Context` as the **first parameter** to every function that does I/O:

```go
// CORRECT
func (s *Service) RecordVital(ctx context.Context, userID uuid.UUID, req RecordVitalRequest) (*VitalReading, error)

// WRONG: context buried in struct or missing
func (s *Service) RecordVital(req RecordVitalRequest) (*VitalReading, error)
```

Extract values from context using typed keys (never string keys):

```go
// internal/middleware/context.go
type contextKey string

const (
    claimsKey    contextKey = "claims"
    requestIDKey contextKey = "request_id"
)

func WithClaims(ctx context.Context, claims *Claims) context.Context {
    return context.WithValue(ctx, claimsKey, claims)
}

func ClaimsFromContext(ctx context.Context) *Claims {
    claims, _ := ctx.Value(claimsKey).(*Claims)
    return claims
}
```

## Dependency Injection

Constructor injection only. No globals. No `init()` side effects. Encore services use the `//encore:service` struct pattern for wiring.

```go
// service.go — depends on interface, not concrete type
type Service struct {
    repo   Repository
    cache  Cache
    logger *slog.Logger
}

func NewService(repo Repository, cache Cache, logger *slog.Logger) *Service {
    return &Service{
        repo:   repo,
        cache:  cache,
        logger: logger,
    }
}
```

Wiring happens in the Encore service initialization function:

```go
// vital/vital.go
//encore:service
type VitalService struct {
    svc *Service
}

func initVitalService() (*VitalService, error) {
    repo := NewRepository(db)
    cache := NewRedisCache(redisClient)
    svc := NewService(repo, cache, slog.Default())
    return &VitalService{svc: svc}, nil
}
```

## Interfaces

Define interfaces at the **consumer** (where they're used), not the implementor:

```go
// vital/service.go — consumer defines the interface it needs
type Repository interface {
    FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*VitalReading, error)
    Create(ctx context.Context, reading *VitalReading) error
    List(ctx context.Context, params ListParams) ([]*VitalReading, error)
    Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
}
```

This enables easy mocking in tests and keeps interfaces minimal (Interface Segregation Principle).

## Struct Tags

```go
// API request/response models — json tags
type RecordVitalRequest struct {
    VitalType  string    `json:"vital_type"  validate:"required"`
    Value      float64   `json:"value"       validate:"required,gt=0"`
    MeasuredAt time.Time `json:"measured_at" validate:"required"`
}

// Database models — db tags (sqlc generates these, do not write manually)
type VitalReading struct {
    ID         uuid.UUID `db:"id"`
    UserID     uuid.UUID `db:"user_id"`
    VitalType  string    `db:"vital_type"`
    MeasuredAt time.Time `db:"measured_at"`
}
```

Never use both `json` and `db` tags on the same struct — keep API models and DB models separate.

## Logging

Use `slog` with structured key-value pairs:

```go
// CORRECT: structured
logger.InfoContext(ctx, "vital recorded",
    slog.String("user_id", userID.String()),
    slog.String("vital_type", req.VitalType),
    slog.String("request_id", requestID),
)

// WRONG: formatted string
logger.Info(fmt.Sprintf("vital recorded for user %s", userID))

// Log errors at the boundary where you choose to absorb the error
logger.ErrorContext(ctx, "failed to record vital",
    slog.String("error", err.Error()),
    slog.String("user_id", userID.String()),
)
```

Never log:
- `vital_value` (PHI)
- JWT tokens, passwords, or secrets
- User email or name (use `user_id` UUID instead)
- Full request/response bodies

## Concurrency

- Use `sync.WaitGroup` for fan-out goroutines with cleanup
- Use `errgroup.Group` (`golang.org/x/sync/errgroup`) for goroutines that return errors
- Protect shared state with `sync.Mutex` or `sync.RWMutex`; document what the mutex protects
- Channel directions in function signatures: `chan<-` (send-only), `<-chan` (receive-only)
- Always handle goroutine lifecycle: goroutines must exit cleanly on context cancellation

```go
// errgroup for concurrent fetches
g, ctx := errgroup.WithContext(ctx)

var vitals []*VitalReading
var thresholds []*AlertThreshold

g.Go(func() error {
    var err error
    vitals, err = s.vitalRepo.ListRecent(ctx, userID)
    return err
})
g.Go(func() error {
    var err error
    thresholds, err = s.alertRepo.ListThresholds(ctx, userID)
    return err
})

if err := g.Wait(); err != nil {
    return nil, fmt.Errorf("fetch user data: %w", err)
}
```

## Response Handling

Encore handles JSON serialization and HTTP status codes automatically based on return types and errors. API functions return typed structs or `error`:

```go
//encore:api auth method=POST path=/api/v1/vitals
func (s *VitalService) RecordVital(ctx context.Context, req *RecordVitalRequest) (*VitalReadingResponse, error) {
    // Return struct → Encore serializes to JSON with 200/201
    // Return error → Encore maps to appropriate HTTP status
    // Return &errs.Error{Code: errs.NotFound, Message: "..."} → 404
}
```

For custom error codes, use Encore's `errs` package:

```go
import "encore.dev/beta/errs"

func mapDomainError(err error) error {
    switch {
    case errors.Is(err, apperror.ErrNotFound):
        return &errs.Error{Code: errs.NotFound, Message: err.Error()}
    case errors.Is(err, apperror.ErrUnauthorized):
        return &errs.Error{Code: errs.Unauthenticated, Message: err.Error()}
    case errors.Is(err, apperror.ErrForbidden):
        return &errs.Error{Code: errs.PermissionDenied, Message: err.Error()}
    case errors.Is(err, apperror.ErrValidation):
        return &errs.Error{Code: errs.InvalidArgument, Message: err.Error()}
    default:
        return err
    }
}
```

## Code Review Checklist

Before submitting a PR, verify:

- [ ] `make lint` passes (`golangci-lint run`)
- [ ] `make fmt` leaves no diffs (`goimports`)
- [ ] All errors handled (no `_` for error returns)
- [ ] Context passed to all I/O functions
- [ ] No business logic in `{service}.go` API handlers
- [ ] No HTTP concerns in `service.go`
- [ ] Repository queries always include `user_id` from auth context
- [ ] New endpoints have correct `//encore:api` annotations (auth vs public)
- [ ] Tests pass with `encore test ./...`
- [ ] Tests added for new service logic
- [ ] No PHI in logs or span attributes
- [ ] Production migration has a matching `down` file

## Linting

`golangci-lint` configured in `.golangci.yml`:

```yaml
linters:
  enable:
    - errcheck        # no unchecked errors
    - govet           # go vet checks
    - staticcheck     # SA checks
    - gosimple        # simplify code
    - ineffassign     # detect unused assignments
    - unused          # detect unused code
    - goimports       # import formatting
    - misspell        # spelling in comments
    - gosec           # security checks (G101, G304, etc.)
    - bodyclose       # ensure http.Response.Body is closed
    - noctx           # require context in http requests
    - exhaustive      # exhaustive switch on enums

linters-settings:
  gosec:
    excludes:
      - G115  # integer overflow — acceptable in our context
  errcheck:
    check-type-assertions: true
```
