package vital

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the data access interface for vital readings.
type Repository interface {
	Create(ctx context.Context, reading *VitalReading) error
	FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*VitalReading, error)
	List(ctx context.Context, params ListParams) ([]*VitalReading, error)
	Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	GetLatestByType(ctx context.Context, userID uuid.UUID) ([]*VitalReading, error)
	GetSummary(ctx context.Context, params SummaryParams) ([]VitalSummaryBucket, error)
}

type ListParams struct {
	UserID    uuid.UUID
	VitalType *string
	From      time.Time
	To        time.Time
	Before    *time.Time
	Limit     int
}

type SummaryParams struct {
	UserID    uuid.UUID
	VitalType string
	From      time.Time
	To        time.Time
	Bucket    string
}

type repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{pool: pool}
}

func (r *repository) Create(ctx context.Context, reading *VitalReading) error {
	err := r.pool.QueryRow(ctx,
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

func (r *repository) FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*VitalReading, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, user_id, vital_type, value, unit, status, device_id, notes, measured_at, created_at
		 FROM vital_readings WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	reading, err := scanReading(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query vital reading %s: %w", id, err)
	}
	return reading, nil
}

func (r *repository) List(ctx context.Context, params ListParams) ([]*VitalReading, error) {
	to := params.To
	if params.Before != nil {
		to = *params.Before
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, vital_type, value, unit, status, device_id, notes, measured_at, created_at
		 FROM vital_readings
		 WHERE user_id = $1
		   AND ($2::varchar IS NULL OR vital_type = $2)
		   AND measured_at >= $3
		   AND measured_at < $4
		 ORDER BY measured_at DESC
		 LIMIT $5`,
		params.UserID, params.VitalType, params.From, to, params.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list vital readings: %w", err)
	}
	defer rows.Close()

	var readings []*VitalReading
	for rows.Next() {
		reading, err := scanReadingRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scan vital reading: %w", err)
		}
		readings = append(readings, reading)
	}
	return readings, rows.Err()
}

func (r *repository) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM vital_readings WHERE id = $1 AND user_id = $2`, id, userID,
	)
	if err != nil {
		return fmt.Errorf("delete vital reading: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	return nil
}

func (r *repository) GetLatestByType(ctx context.Context, userID uuid.UUID) ([]*VitalReading, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT ON (vital_type) id, user_id, vital_type, value, unit, status, device_id, notes, measured_at, created_at
		 FROM vital_readings
		 WHERE user_id = $1
		 ORDER BY vital_type, measured_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("get latest vitals: %w", err)
	}
	defer rows.Close()

	var readings []*VitalReading
	for rows.Next() {
		reading, err := scanReadingRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scan latest vital: %w", err)
		}
		readings = append(readings, reading)
	}
	return readings, rows.Err()
}

func (r *repository) GetSummary(ctx context.Context, params SummaryParams) ([]VitalSummaryBucket, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT
			date_trunc($5, measured_at) AS bucket,
			AVG(value) AS avg_value,
			MIN(value) AS min_value,
			MAX(value) AS max_value,
			COUNT(*) AS reading_count
		 FROM vital_readings
		 WHERE user_id = $1
		   AND vital_type = $2
		   AND measured_at >= $3
		   AND measured_at < $4
		 GROUP BY bucket
		 ORDER BY bucket DESC`,
		params.UserID, params.VitalType, params.From, params.To, params.Bucket,
	)
	if err != nil {
		return nil, fmt.Errorf("get vital summary: %w", err)
	}
	defer rows.Close()

	var buckets []VitalSummaryBucket
	for rows.Next() {
		var b VitalSummaryBucket
		if err := rows.Scan(&b.Bucket, &b.Avg, &b.Min, &b.Max, &b.ReadingCount); err != nil {
			return nil, fmt.Errorf("scan summary bucket: %w", err)
		}
		buckets = append(buckets, b)
	}
	return buckets, rows.Err()
}

func scanReading(row pgx.Row) (*VitalReading, error) {
	var r VitalReading
	err := row.Scan(&r.ID, &r.UserID, &r.VitalType, &r.Value, &r.Unit,
		&r.Status, &r.DeviceID, &r.Notes, &r.MeasuredAt, &r.CreatedAt)
	return &r, err
}

func scanReadingRows(rows pgx.Rows) (*VitalReading, error) {
	var r VitalReading
	err := rows.Scan(&r.ID, &r.UserID, &r.VitalType, &r.Value, &r.Unit,
		&r.Status, &r.DeviceID, &r.Notes, &r.MeasuredAt, &r.CreatedAt)
	return &r, err
}
