package vital

import (
	"context"
	"fmt"
	"strings"
)

type TableInfo struct {
	Name string `json:"name"`
}

type CheckTablesResponse struct {
	Tables  []string `json:"tables"`
	Missing []string `json:"missing"`
	Ready   bool     `json:"ready"`
}

type MigrateResponse struct {
	Message string   `json:"message"`
	Created []string `json:"created,omitempty"`
}

var requiredTables = []string{
	"users", "vital_readings", "alert_thresholds", "alerts", "devices", "provider_patients",
}

// CheckTables lists which required tables exist in the database.
//
//encore:api public method=GET path=/admin/check-tables
func (s *Service) CheckTables(ctx context.Context) (*CheckTablesResponse, error) {
	rows, err := pool.Query(ctx,
		`SELECT table_name FROM information_schema.tables
		 WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		 ORDER BY table_name`)
	if err != nil {
		return nil, fmt.Errorf("query tables: %w", err)
	}
	defer rows.Close()

	existing := map[string]bool{}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan table: %w", err)
		}
		existing[name] = true
		tables = append(tables, name)
	}

	var missing []string
	for _, t := range requiredTables {
		if !existing[t] {
			missing = append(missing, t)
		}
	}

	return &CheckTablesResponse{
		Tables:  tables,
		Missing: missing,
		Ready:   len(missing) == 0,
	}, nil
}

// RunMigration creates any missing tables using idempotent DDL.
//
//encore:api public method=POST path=/admin/migrate
func (s *Service) RunMigration(ctx context.Context) (*MigrateResponse, error) {
	check, err := s.CheckTables(ctx)
	if err != nil {
		return nil, err
	}
	if check.Ready {
		return &MigrateResponse{Message: "all tables already exist, nothing to do"}, nil
	}

	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			provider_id     VARCHAR(255) NOT NULL UNIQUE,
			email           VARCHAR(255) NOT NULL UNIQUE,
			name            VARCHAR(255) NOT NULL,
			date_of_birth   DATE,
			avatar_url      TEXT,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_users_provider_id ON users (provider_id)`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users (email)`,

		`CREATE TABLE IF NOT EXISTS vital_readings (
			id              UUID NOT NULL DEFAULT gen_random_uuid(),
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
		)`,
		`CREATE INDEX IF NOT EXISTS idx_vital_readings_user_type_time
			ON vital_readings (user_id, vital_type, measured_at DESC)`,

		`CREATE TABLE IF NOT EXISTS alert_thresholds (
			id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			vital_type      VARCHAR(50) NOT NULL,
			low_value       DOUBLE PRECISION,
			high_value      DOUBLE PRECISION,
			enabled         BOOLEAN NOT NULL DEFAULT TRUE,
			UNIQUE(user_id, vital_type)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_thresholds_user_id ON alert_thresholds (user_id)`,

		`CREATE TABLE IF NOT EXISTS alerts (
			id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			vital_type          VARCHAR(50) NOT NULL,
			value               DOUBLE PRECISION NOT NULL,
			threshold_breached  VARCHAR(10) NOT NULL,
			threshold_value     DOUBLE PRECISION NOT NULL,
			severity            VARCHAR(20) NOT NULL DEFAULT 'warning',
			acknowledged        BOOLEAN NOT NULL DEFAULT FALSE,
			acknowledged_at     TIMESTAMPTZ,
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_alerts_user_id ON alerts (user_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_alerts_unacked ON alerts (user_id, acknowledged) WHERE acknowledged = FALSE`,

		`CREATE TABLE IF NOT EXISTS devices (
			id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name            VARCHAR(100) NOT NULL,
			type            VARCHAR(50) NOT NULL,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_user_id ON devices (user_id)`,

		`CREATE TABLE IF NOT EXISTS provider_patients (
			provider_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			patient_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (provider_id, patient_id)
		)`,
	}

	var created []string
	for _, stmt := range statements {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return nil, fmt.Errorf("exec migration: %w (statement: %s)", err, truncate(stmt, 80))
		}
		if strings.HasPrefix(strings.TrimSpace(stmt), "CREATE TABLE") {
			parts := strings.Fields(stmt)
			for i, p := range parts {
				if strings.EqualFold(p, "EXISTS") && i+1 < len(parts) {
					created = append(created, parts[i+1])
					break
				}
			}
		}
	}

	return &MigrateResponse{
		Message: fmt.Sprintf("migration complete, ensured %d tables exist", len(requiredTables)),
		Created: created,
	}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
