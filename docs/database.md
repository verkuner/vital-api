# Database

PostgreSQL patterns, sqlc usage, and query conventions for the Vital Signs API.

## Environment Strategy

| Component | Local / Dev (Encore) | Production (Standalone) |
|-----------|---------------------|------------------------|
| PostgreSQL | Encore auto-provisioned | PostgreSQL 18 (standalone) |
| TimescaleDB | Not available | TimescaleDB 2.25 extension |
| Driver | Encore `sqldb` primitives | pgx v5 + connection pool |
| Query gen | sqlc (compatible with Encore `sqldb`) | sqlc via pgx |
| Migrations | Encore built-in migration runner | golang-migrate v4 |

## Local/Dev — Encore Database

Encore automatically provisions a PostgreSQL instance when running `encore run`. Each service declares its own database:

```go
// vital/db.go
import "encore.dev/storage/sqldb"

var db = sqldb.NewDatabase("vitals", sqldb.DatabaseConfig{
    Migrations: "./migrations",
})
```

Migrations live inside the service directory and run automatically on startup:

```
vital/
  migrations/
    1_create_vitals.up.sql
    2_add_indexes.up.sql
  db.go
  vital.go
  ...
```

### Querying with Encore sqldb

```go
// vital/repository.go
func (r *repository) FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*VitalReading, error) {
    row := db.QueryRow(ctx,
        `SELECT id, user_id, vital_type, value, unit, status, measured_at, created_at
         FROM vital_readings WHERE id = $1 AND user_id = $2`,
        id, userID,
    )
    // scan row...
}
```

Encore's `sqldb` is a thin wrapper over `database/sql` — sqlc-generated code works with it via the `database/sql` compatible interface.

### TimescaleDB Limitations in Local/Dev

Encore's auto-provisioned PostgreSQL does not include TimescaleDB. Code that uses TimescaleDB-specific features (`time_bucket`, `create_hypertable`, continuous aggregates) must handle graceful fallback:

```go
func (r *repository) GetSummary(ctx context.Context, params SummaryParams) (*VitalSummary, error) {
    // Use date_trunc in local/dev, time_bucket in production
    bucketFn := "date_trunc('hour', measured_at)"
    if r.hasTimescaleDB {
        bucketFn = "time_bucket('1 hour', measured_at)"
    }
    // ...
}
```

## Production — Standalone PostgreSQL + TimescaleDB

### Connection Pool

```go
// database/postgres.go
func NewPool(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
    config, err := pgxpool.ParseConfig(cfg.URL)
    if err != nil {
        return nil, fmt.Errorf("parse database URL: %w", err)
    }

    config.MaxConns = int32(cfg.MaxConns)       // default: 25
    config.MinConns = int32(cfg.MinConns)       // default: 5
    config.MaxConnIdleTime = 5 * time.Minute
    config.MaxConnLifetime = 30 * time.Minute
    config.HealthCheckPeriod = 1 * time.Minute

    // OTel instrumentation (production only)
    config.ConnConfig.Tracer = otelpgx.NewTracer()

    pool, err := pgxpool.NewWithConfig(ctx, config)
    if err != nil {
        return nil, fmt.Errorf("create pool: %w", err)
    }

    if err := pool.Ping(ctx); err != nil {
        return nil, fmt.Errorf("ping database: %w", err)
    }

    return pool, nil
}
```

Production environment variables:
```bash
DATABASE_URL=postgres://user:pass@db-host:5432/vital_signs?sslmode=require
DATABASE_MAX_CONNS=25
DATABASE_MIN_CONNS=5
```

## sqlc — Query Generation

Write SQL in `db/queries/`, run `make sqlc`, get type-safe Go functions. sqlc-generated code works with both Encore's `sqldb` (local/dev) and pgx (production).

```
db/
  queries/
    vitals.sql       # sqlc-annotated queries
    users.sql
    alerts.sql
    auth.sql
  sqlc.yaml          # configuration
```

### sqlc Configuration

```yaml
# db/sqlc.yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "queries/"
    schema: "vital/migrations/"
    gen:
      go:
        package: "dbgen"
        out: "database/dbgen"
        sql_package: "database/sql"   # compatible with both Encore sqldb and pgx
        emit_json_tags: true
        emit_db_tags: true
        emit_pointers_for_null_types: true
        emit_enum_valid_method: true
        emit_all_enum_values: true
```

### Writing Queries

```sql
-- db/queries/vitals.sql

-- name: CreateVitalReading :one
INSERT INTO vital_readings (
    user_id, vital_type, value, unit, status, device_id, notes, measured_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: GetVitalReadingByID :one
SELECT * FROM vital_readings
WHERE id = $1 AND user_id = $2;

-- name: ListVitalReadings :many
SELECT * FROM vital_readings
WHERE user_id = $1
  AND vital_type = $2
  AND measured_at >= $3
  AND measured_at <= $4
ORDER BY measured_at DESC
LIMIT $5;

-- name: DeleteVitalReading :exec
DELETE FROM vital_readings
WHERE id = $1 AND user_id = $2;
```

### Using Generated Code in Repositories

```go
// vital/repository.go
type repository struct {
    db *sqldb.Database   // Encore sqldb database reference
}

func NewRepository(db *sqldb.Database) Repository {
    return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, reading *VitalReading) error {
    err := r.db.QueryRow(ctx,
        `INSERT INTO vital_readings (user_id, vital_type, value, unit, status, device_id, notes, measured_at)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
         RETURNING id, created_at`,
        reading.UserID, reading.VitalType, reading.Value, reading.Unit,
        reading.Status, reading.DeviceID, reading.Notes, reading.MeasuredAt,
    ).Scan(&reading.ID, &reading.CreatedAt)
    if err != nil {
        return fmt.Errorf("create vital reading: %w", err)
    }
    return nil
}
```

### Transactions

```go
func (r *repository) RecordWithAlertCheck(ctx context.Context, reading *VitalReading, alert *Alert) error {
    tx, err := r.db.Begin(ctx)
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback()

    if err := r.createReading(ctx, tx, reading); err != nil {
        return fmt.Errorf("insert reading: %w", err)
    }

    if alert != nil {
        if err := r.createAlert(ctx, tx, alert); err != nil {
            return fmt.Errorf("insert alert: %w", err)
        }
    }

    return tx.Commit()
}
```

## TimescaleDB

### Hypertable Setup

`vital_readings` is a TimescaleDB hypertable partitioned by `measured_at`.

```sql
-- 000001_init.up.sql
CREATE TABLE vital_readings (
    id          UUID DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    vital_type  VARCHAR(50) NOT NULL,
    value       DOUBLE PRECISION NOT NULL,
    unit        VARCHAR(20) NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'normal',
    device_id   UUID,
    notes       TEXT,
    measured_at TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, measured_at)  -- partition key must be in PK
);

SELECT create_hypertable('vital_readings', 'measured_at',
    chunk_time_interval => INTERVAL '1 day'
);

-- Index for common query pattern: user + type + time range
CREATE INDEX idx_vital_readings_user_type_time
    ON vital_readings (user_id, vital_type, measured_at DESC);
```

### Query Rules for Hypertables

Always include the partition key (`measured_at`) in:
- `WHERE` clauses — enables chunk exclusion (avoids scanning all partitions)
- `PRIMARY KEY` lookups — required by TimescaleDB for point queries

```sql
-- GOOD: partition key in WHERE → chunk pruning
SELECT * FROM vital_readings
WHERE user_id = $1 AND measured_at >= NOW() - INTERVAL '7 days';

-- BAD: no time filter → full table scan across all chunks
SELECT * FROM vital_readings WHERE user_id = $1;
```

### Time-Series Aggregations

```sql
-- name: GetVitalSummary :one
-- Hourly average for dashboard charts
SELECT
    time_bucket('1 hour', measured_at) AS bucket,
    vital_type,
    AVG(value) AS avg_value,
    MIN(value) AS min_value,
    MAX(value) AS max_value,
    COUNT(*) AS reading_count
FROM vital_readings
WHERE user_id = $1
  AND vital_type = $2
  AND measured_at >= $3
  AND measured_at < $4
GROUP BY bucket, vital_type
ORDER BY bucket DESC;
```

### Continuous Aggregates

For dashboard queries that need fast pre-aggregated data:

```sql
-- 000003_continuous_aggregates.up.sql
CREATE MATERIALIZED VIEW vital_hourly_stats
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', measured_at) AS hour,
    user_id,
    vital_type,
    AVG(value)  AS avg_value,
    MIN(value)  AS min_value,
    MAX(value)  AS max_value,
    COUNT(*)    AS count
FROM vital_readings
GROUP BY hour, user_id, vital_type;

SELECT add_continuous_aggregate_policy('vital_hourly_stats',
    start_offset => INTERVAL '3 hours',
    end_offset   => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour'
);
```

### Data Retention

```sql
-- 000004_retention.up.sql
-- Automatically drop chunks older than 2 years
SELECT add_retention_policy('vital_readings', INTERVAL '2 years');
```

## Migrations

### Local/Dev (Encore)

Migrations live inside each service's `migrations/` directory and are numbered sequentially:

```
vital/
  migrations/
    1_create_vitals.up.sql
    2_add_indexes.up.sql
    3_continuous_aggregates.up.sql   # skipped locally (TimescaleDB-only)
```

Encore runs migrations automatically on `encore run`. No manual migration commands needed.

### Production (golang-migrate)

Production migrations use 6-digit zero-padded numbering with up/down pairs:

```
database/migrations/
  000001_init.up.sql
  000001_init.down.sql
  000002_add_alert_thresholds.up.sql
  000002_add_alert_thresholds.down.sql
  000003_continuous_aggregates.up.sql
  000003_continuous_aggregates.down.sql
```

### Rules

- **Local/Dev**: Encore manages migrations. Sequential numbering: `1_`, `2_`, `3_`.
- **Production**: 6-digit zero-padded. Every `up` must have a corresponding `down`.
- Never modify a migration that has been applied to any environment
- `down` migration must cleanly reverse everything the `up` does
- Test both `up` and `down` in CI before merging
- TimescaleDB-specific migrations (hypertables, continuous aggregates) should be conditional or in separate files marked for production only

### Running Migrations

```bash
# Local/Dev: automatic via `encore run`

# Production:
make migrate-up              # Apply all pending
make migrate-down            # Rollback one step
make migrate-create NAME=x   # Create new migration
make migrate-version         # Check current version
```

Production uses `scripts/migrate.sh` with `golang-migrate`:
```bash
migrate -database "$DATABASE_URL" -path database/migrations up
```

### Migration Template

```sql
-- 000005_add_devices.up.sql
CREATE TABLE devices (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        VARCHAR(100) NOT NULL,
    type        VARCHAR(50) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_devices_user_id ON devices(user_id);
```

```sql
-- 000005_add_devices.down.sql
DROP TABLE IF EXISTS devices;
```

## Redis

Used for: session cache, rate limiting state, WebSocket pub/sub, auth provider token cache.

```go
// internal/database/redis.go
func NewRedisClient(cfg Config) (*redis.Client, error) {
    opt, err := redis.ParseURL(cfg.URL)
    if err != nil {
        return nil, fmt.Errorf("parse Redis URL: %w", err)
    }

    client := redis.NewClient(opt)

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := client.Ping(ctx).Err(); err != nil {
        return nil, fmt.Errorf("ping Redis: %w", err)
    }

    return client, nil
}
```

### Key Naming Convention

```
{service}:{feature}:{identifier}
Examples:
  vital-api:session:{user_id}
  vital-api:ratelimit:auth:{ip}
  vital-api:ratelimit:api:{user_id}
  vital-api:authprovider:token:admin
  vital-api:cache:user:{user_id}
```

### TTL Policy

| Key Pattern | TTL |
|-------------|-----|
| `session:{user_id}` | Matches JWT expiry (15 min) |
| `ratelimit:*` | Window duration (1 min) |
| `authprovider:token:admin` | Token expiry - 30s |
| `cache:user:{id}` | 5 minutes |
| `websocket:hub:*` | Session lifetime |

## Query Performance Guidelines

1. **Always filter by partition key** for TimescaleDB queries
2. **Use indexes**: check `EXPLAIN ANALYZE` for queries returning > 1000 rows
3. **Avoid N+1**: prefer `JOIN` or batch queries over looping
4. **Use `LIMIT`**: always cap list queries; `sqlc` enforces via `:many` with explicit `LIMIT $n`
5. **Pagination**: use cursor (time-based) for vitals, offset for bounded sets
6. **Connection limits**: default pool max = 25; TimescaleDB background workers consume ~5 connections

### Identifying Slow Queries

```sql
-- Find queries over 100ms in pg_stat_statements
SELECT query, mean_exec_time, calls
FROM pg_stat_statements
WHERE mean_exec_time > 100
ORDER BY mean_exec_time DESC
LIMIT 20;
```

Alert configured in SigNoz: `db.query.duration_ms p95 > 500ms`.
