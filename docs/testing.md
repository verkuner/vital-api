# Testing

Testing strategy, patterns, and tooling for the Vital Signs API.

## Overview

| Layer | Tool | Scope |
|-------|------|-------|
| Unit | `testing` + `testify` | Service logic, API handler parsing, model methods |
| Integration | `encore test` + testcontainers-go | Repository + real DB (Encore-provisioned or containers) |
| End-to-End | Encore test framework | Full API → service → repository |
| Contract | Encore Service Catalog (local/dev), `swaggo` (production) | Response shape validation |

Target coverage: **80%+ for service packages**, **70%+ overall**.

## Running Tests

```bash
# Local/Dev (Encore — auto-provisions test databases)
encore test ./...           # run all tests with Encore test runner
encore test ./vital/...     # single service
encore test ./vital/... -run TestService_RecordVital_ValidReading -v

# Standard Go tests (also works)
make test                   # go test ./...
make test-cover             # with HTML coverage report
make test-integration       # integration tests (requires Docker daemon)
make test-race              # run with -race flag

# Coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

`encore test` automatically provisions isolated test databases for each test, running migrations before tests and cleaning up after. No manual Docker setup needed for local testing.

## Test Organization

Tests live alongside the code they test:

```
vital/
  vital.go
  vital_test.go        # unit tests for API handlers (mock service)
  service.go
  service_test.go      # unit tests for service (mock repo)
  repository.go
  repository_test.go   # integration tests (Encore test DB or testcontainers)
  model.go
```

Integration tests that require Docker (testcontainers) use the `integration` build tag:

```go
//go:build integration
```

```bash
go test -tags=integration ./...
```

Tests using Encore's `sqldb` automatically get a test database via `encore test` — no build tags needed.

## Naming Convention

```
Test{FunctionName}_{Scenario}_{ExpectedResult}

Examples:
  TestService_RecordVital_ValidReading_ReturnsCreatedReading
  TestService_RecordVital_FutureTimestamp_ReturnsValidationError
  TestHandler_GetVitals_Unauthenticated_Returns401
  TestRepository_FindByID_NotFound_ReturnsNil
```

## Unit Tests

### Service Tests (with mock repository)

Define the mock interface in the test file or a separate `mock_test.go` in the same package:

```go
// internal/vital/mock_test.go
type mockRepository struct {
    mock.Mock
}

func (m *mockRepository) Create(ctx context.Context, r *VitalReading) error {
    args := m.Called(ctx, r)
    return args.Error(0)
}

func (m *mockRepository) FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*VitalReading, error) {
    args := m.Called(ctx, id, userID)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*VitalReading), args.Error(1)
}
```

```go
// internal/vital/service_test.go
func TestService_RecordVital_ValidReading_ReturnsCreated(t *testing.T) {
    // Arrange
    repo := new(mockRepository)
    svc := NewService(repo, nil, slog.Default())

    input := RecordVitalRequest{
        VitalType:  "heart_rate",
        Value:      72.0,
        Unit:       "bpm",
        MeasuredAt: time.Now().Add(-1 * time.Minute),
    }
    userID := uuid.New()

    repo.On("Create", mock.Anything, mock.MatchedBy(func(r *VitalReading) bool {
        return r.VitalType == "heart_rate" && r.UserID == userID
    })).Return(nil)

    // Act
    result, err := svc.RecordVital(context.Background(), userID, input)

    // Assert
    require.NoError(t, err)
    assert.Equal(t, "heart_rate", result.VitalType)
    assert.Equal(t, userID, result.UserID)
    repo.AssertExpectations(t)
}

func TestService_RecordVital_FutureTimestamp_ReturnsError(t *testing.T) {
    repo := new(mockRepository)
    svc := NewService(repo, nil, slog.Default())

    input := RecordVitalRequest{
        VitalType:  "heart_rate",
        Value:      72.0,
        Unit:       "bpm",
        MeasuredAt: time.Now().Add(10 * time.Minute), // future
    }

    _, err := svc.RecordVital(context.Background(), uuid.New(), input)

    require.Error(t, err)
    assert.ErrorIs(t, err, apperror.ErrValidation)
    repo.AssertNotCalled(t, "Create")
}
```

### Table-Driven Tests

Use for exhaustive input validation and boundary cases:

```go
func TestService_RecordVital_VitalTypeValidation(t *testing.T) {
    tests := []struct {
        name      string
        vitalType string
        wantErr   bool
    }{
        {"valid heart_rate",     "heart_rate",           false},
        {"valid spo2",           "spo2",                 false},
        {"invalid empty",        "",                     true},
        {"invalid unknown type", "blood_sugar",          true},
        {"invalid uppercase",    "HEART_RATE",           true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            svc := NewService(new(mockRepository), nil, slog.Default())
            _, err := svc.RecordVital(ctx, uuid.New(), RecordVitalRequest{
                VitalType:  tt.vitalType,
                Value:      72,
                Unit:       "bpm",
                MeasuredAt: time.Now(),
            })
            if tt.wantErr {
                require.Error(t, err)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

### API Handler Tests

Encore API handlers are tested by calling the service methods directly (Encore handles HTTP serialization):

```go
// vital/vital_test.go
func TestVitalService_RecordVital_ValidRequest(t *testing.T) {
    svc := new(mockService)
    vs := &VitalService{svc: svc}

    req := &RecordVitalRequest{
        VitalType:  "heart_rate",
        Value:      72.0,
        Unit:       "bpm",
        MeasuredAt: time.Now().Add(-1 * time.Minute),
    }

    expectedReading := &VitalReading{VitalType: "heart_rate", Value: 72}
    svc.On("RecordVital", mock.Anything, mock.Anything, mock.Anything).
        Return(expectedReading, nil)

    resp, err := vs.RecordVital(context.Background(), req)

    require.NoError(t, err)
    assert.Equal(t, "heart_rate", resp.Data.VitalType)
    svc.AssertExpectations(t)
}
```

For raw endpoints (WebSocket), use `httptest.NewRecorder`:

```go
// websocket/websocket_test.go
func TestWebSocket_HandleUpgrade(t *testing.T) {
    // Use httptest for raw Encore endpoints
    req := httptest.NewRequest(http.MethodGet, "/ws?token=valid-jwt", nil)
    w := httptest.NewRecorder()
    // ...
}
```

## Integration Tests

### Local/Dev (Encore Test Runner)

`encore test` automatically provisions isolated test databases, runs migrations, and cleans up. Repository tests work directly with Encore's `sqldb`:

```go
// vital/repository_test.go
func TestRepository_Create_And_FindByID(t *testing.T) {
    repo := NewRepository(db)   // db is the Encore sqldb.Database declared in db.go
    ctx := context.Background()

    userID := createTestUser(t)

    reading := &VitalReading{
        UserID:     userID,
        VitalType:  "heart_rate",
        Value:      72.5,
        Unit:       "bpm",
        Status:     "normal",
        MeasuredAt: time.Now().UTC().Truncate(time.Microsecond),
    }

    err := repo.Create(ctx, reading)
    require.NoError(t, err)
    assert.NotEqual(t, uuid.Nil, reading.ID)

    found, err := repo.FindByID(ctx, reading.ID, userID)
    require.NoError(t, err)
    require.NotNil(t, found)
    assert.Equal(t, reading.VitalType, found.VitalType)
    assert.InDelta(t, reading.Value, found.Value, 0.001)
}

func TestRepository_FindByID_WrongUser_ReturnsNil(t *testing.T) {
    repo := NewRepository(db)
    ctx := context.Background()

    ownerID := createTestUser(t)
    otherID := createTestUser(t)

    reading := createTestVitalReading(t, ownerID)

    found, err := repo.FindByID(ctx, reading.ID, otherID)
    require.NoError(t, err)
    assert.Nil(t, found, "should not return another user's reading")
}
```

### CI / Production (testcontainers-go)

For CI or when testing with TimescaleDB features, use `testcontainers-go`:

```go
// testutil/containers.go
//go:build integration

package testutil

type TestDB struct {
    Pool *pgxpool.Pool
    DSN  string
}

func NewTestDB(t *testing.T) *TestDB {
    t.Helper()
    ctx := context.Background()

    container, err := postgres.Run(ctx, "timescale/timescaledb:latest-pg18",
        postgres.WithDatabase("vital_test"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database system is ready").WithStartupTimeout(30*time.Second),
        ),
    )
    require.NoError(t, err)
    t.Cleanup(func() { _ = container.Terminate(ctx) })

    dsn, err := container.ConnectionString(ctx, "sslmode=disable")
    require.NoError(t, err)

    runMigrations(t, dsn)

    pool, err := pgxpool.New(ctx, dsn)
    require.NoError(t, err)
    t.Cleanup(pool.Close)

    return &TestDB{Pool: pool, DSN: dsn}
}
```

## Test Fixtures & Helpers

```go
// internal/testutil/fixtures.go
func CreateUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
    t.Helper()
    id := uuid.New()
    _, err := pool.Exec(context.Background(),
        `INSERT INTO users (id, provider_id, email, name) VALUES ($1, $2, $3, $4)`,
        id, uuid.New(), fmt.Sprintf("user-%s@test.com", id), "Test User",
    )
    require.NoError(t, err)
    t.Cleanup(func() {
        _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id)
    })
    return id
}
```

Use `t.Cleanup` (not `defer`) for cleanup — it runs even if the test fails and works correctly in table-driven tests.

## Test Data Management

- Each test creates its own isolated data with `t.Cleanup` teardown
- Never share mutable state between tests — use subtests (`t.Run`) or separate test functions
- Use `uuid.New()` for IDs, never hardcode UUIDs in tests
- Use `time.Now().UTC()` for timestamps, avoid `time.Now()` (timezone issues in CI)

## Assertions Style

```go
// Prefer require for fatal assertions (stops test on failure)
require.NoError(t, err)
require.NotNil(t, result)

// Use assert for non-fatal checks (test continues after failure)
assert.Equal(t, http.StatusCreated, w.Code)
assert.Equal(t, "heart_rate", result.VitalType)

// Use ErrorIs for wrapped errors
assert.ErrorIs(t, err, apperror.ErrNotFound)

// Float comparisons
assert.InDelta(t, 72.5, result.Value, 0.001)
```

## CI Pipeline

```yaml
# .github/workflows/test.yml
- name: Unit tests (Encore)
  run: encore test -race -count=1 ./...

- name: Integration tests (TimescaleDB via testcontainers)
  run: go test -race -count=1 -tags=integration ./...

- name: Coverage check
  run: |
    go test -coverprofile=coverage.out ./...
    go tool cover -func=coverage.out | grep total | awk '{print $3}' | \
      awk -F'%' '{if ($1+0 < 70) {print "Coverage below 70%"; exit 1}}'

- name: Race detector
  run: go test -race ./...
```

## Mocking Guidelines

- Define mock structs in `*_test.go` files or `mock_test.go` in the same package
- Use `github.com/stretchr/testify/mock` for mock structs
- Mock only the interface boundary — do not mock concrete types
- Assert all mock expectations with `mock.AssertExpectations(t)` at the end of each test
- For simple one-off mocks, use `mock.MatchedBy` with inline functions instead of `.On("method", specificValue)`

## What NOT to Test

- Don't test the framework (Chi routing, pgx connection pooling)
- Don't test generated code (sqlc output)
- Don't test trivial getters/setters
- Don't write tests that duplicate integration test coverage in unit tests
- Don't test error messages verbatim (test error type / `errors.Is`, not string content)
