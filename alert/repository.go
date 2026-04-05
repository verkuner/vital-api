package alert

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the data access interface for alerts and thresholds.
type Repository interface {
	CreateAlert(ctx context.Context, alert *Alert) error
	FindAlertByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*Alert, error)
	ListAlerts(ctx context.Context, params ListAlertParams) ([]*Alert, error)
	AcknowledgeAlert(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*Alert, error)
	GetThreshold(ctx context.Context, userID uuid.UUID, vitalType string) (*AlertThreshold, error)
	ListThresholds(ctx context.Context, userID uuid.UUID) ([]*AlertThreshold, error)
	UpsertThreshold(ctx context.Context, threshold *AlertThreshold) error
	DeleteThreshold(ctx context.Context, userID uuid.UUID, vitalType string) error
}

type ListAlertParams struct {
	UserID       uuid.UUID
	Severity     *string
	Acknowledged *bool
	VitalType    *string
	From         time.Time
	To           time.Time
	Limit        int
	Offset       int
}

type repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{pool: pool}
}

func (r *repository) CreateAlert(ctx context.Context, alert *Alert) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO alerts (user_id, vital_type, value, threshold_breached, threshold_value, severity)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, acknowledged, created_at`,
		alert.UserID, alert.VitalType, alert.Value,
		alert.ThresholdBreached, alert.ThresholdValue, alert.Severity,
	).Scan(&alert.ID, &alert.Acknowledged, &alert.CreatedAt)
	if err != nil {
		return fmt.Errorf("create alert: %w", err)
	}
	return nil
}

func (r *repository) FindAlertByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*Alert, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, user_id, vital_type, value, threshold_breached, threshold_value,
		        severity, acknowledged, acknowledged_at, created_at
		 FROM alerts WHERE id = $1 AND user_id = $2`, id, userID,
	)
	a, err := scanAlert(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find alert %s: %w", id, err)
	}
	return a, nil
}

func (r *repository) ListAlerts(ctx context.Context, params ListAlertParams) ([]*Alert, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, vital_type, value, threshold_breached, threshold_value,
		        severity, acknowledged, acknowledged_at, created_at
		 FROM alerts
		 WHERE user_id = $1
		   AND ($2::varchar IS NULL OR severity = $2)
		   AND ($3::bool IS NULL OR acknowledged = $3)
		   AND ($4::varchar IS NULL OR vital_type = $4)
		   AND created_at >= $5
		   AND created_at <= $6
		 ORDER BY created_at DESC
		 LIMIT $7 OFFSET $8`,
		params.UserID, params.Severity, params.Acknowledged, params.VitalType,
		params.From, params.To, params.Limit, params.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list alerts: %w", err)
	}
	defer rows.Close()

	var alerts []*Alert
	for rows.Next() {
		a, err := scanAlertRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scan alert: %w", err)
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

func (r *repository) AcknowledgeAlert(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*Alert, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE alerts
		 SET acknowledged = TRUE, acknowledged_at = NOW()
		 WHERE id = $1 AND user_id = $2
		 RETURNING id, user_id, vital_type, value, threshold_breached, threshold_value,
		           severity, acknowledged, acknowledged_at, created_at`,
		id, userID,
	)
	a, err := scanAlert(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("acknowledge alert %s: %w", id, err)
	}
	return a, nil
}

func (r *repository) GetThreshold(ctx context.Context, userID uuid.UUID, vitalType string) (*AlertThreshold, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, user_id, vital_type, low_value, high_value, enabled
		 FROM alert_thresholds WHERE user_id = $1 AND vital_type = $2`,
		userID, vitalType,
	)
	var t AlertThreshold
	err := row.Scan(&t.ID, &t.UserID, &t.VitalType, &t.LowValue, &t.HighValue, &t.Enabled)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get threshold: %w", err)
	}
	return &t, nil
}

func (r *repository) ListThresholds(ctx context.Context, userID uuid.UUID) ([]*AlertThreshold, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, vital_type, low_value, high_value, enabled
		 FROM alert_thresholds WHERE user_id = $1 ORDER BY vital_type`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list thresholds: %w", err)
	}
	defer rows.Close()

	var thresholds []*AlertThreshold
	for rows.Next() {
		var t AlertThreshold
		if err := rows.Scan(&t.ID, &t.UserID, &t.VitalType, &t.LowValue, &t.HighValue, &t.Enabled); err != nil {
			return nil, fmt.Errorf("scan threshold: %w", err)
		}
		thresholds = append(thresholds, &t)
	}
	return thresholds, rows.Err()
}

func (r *repository) UpsertThreshold(ctx context.Context, threshold *AlertThreshold) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO alert_thresholds (user_id, vital_type, low_value, high_value, enabled)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (user_id, vital_type) DO UPDATE
		 SET low_value = EXCLUDED.low_value,
		     high_value = EXCLUDED.high_value,
		     enabled = EXCLUDED.enabled
		 RETURNING id`,
		threshold.UserID, threshold.VitalType, threshold.LowValue, threshold.HighValue, threshold.Enabled,
	).Scan(&threshold.ID)
	if err != nil {
		return fmt.Errorf("upsert threshold: %w", err)
	}
	return nil
}

func (r *repository) DeleteThreshold(ctx context.Context, userID uuid.UUID, vitalType string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM alert_thresholds WHERE user_id = $1 AND vital_type = $2`,
		userID, vitalType,
	)
	if err != nil {
		return fmt.Errorf("delete threshold: %w", err)
	}
	return nil
}

func scanAlert(row pgx.Row) (*Alert, error) {
	var a Alert
	err := row.Scan(&a.ID, &a.UserID, &a.VitalType, &a.Value,
		&a.ThresholdBreached, &a.ThresholdValue, &a.Severity,
		&a.Acknowledged, &a.AcknowledgedAt, &a.CreatedAt)
	return &a, err
}

func scanAlertRows(rows pgx.Rows) (*Alert, error) {
	var a Alert
	err := rows.Scan(&a.ID, &a.UserID, &a.VitalType, &a.Value,
		&a.ThresholdBreached, &a.ThresholdValue, &a.Severity,
		&a.Acknowledged, &a.AcknowledgedAt, &a.CreatedAt)
	return &a, err
}
