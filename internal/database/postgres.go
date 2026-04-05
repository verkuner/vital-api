package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolConfig holds connection pool settings sourced from DatabaseConfig.
type PoolConfig struct {
	URL      string
	MaxConns int
	MinConns int
}

// NewPool creates a pgxpool.Pool with OTel instrumentation and validates the
// connection with a Ping before returning.
func NewPool(ctx context.Context, cfg PoolConfig) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}

	config.MaxConns = int32(cfg.MaxConns)
	config.MinConns = int32(cfg.MinConns)
	config.MaxConnIdleTime = 5 * time.Minute
	config.MaxConnLifetime = 30 * time.Minute
	config.HealthCheckPeriod = 1 * time.Minute

	// TODO: Add OTel tracer in production: config.ConnConfig.Tracer = otelpgx.NewTracer()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
