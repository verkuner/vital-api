-- name: CreateVitalReading :one
INSERT INTO vital_readings (user_id, vital_type, value, unit, status, device_id, notes, measured_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetVitalReadingByID :one
SELECT * FROM vital_readings
WHERE id = $1 AND user_id = $2;

-- name: ListVitalReadings :many
SELECT * FROM vital_readings
WHERE user_id = $1
  AND ($2::varchar IS NULL OR vital_type = $2)
  AND measured_at >= $3
  AND measured_at <= $4
ORDER BY measured_at DESC
LIMIT $5;

-- name: ListVitalReadingsBefore :many
SELECT * FROM vital_readings
WHERE user_id = $1
  AND ($2::varchar IS NULL OR vital_type = $2)
  AND measured_at >= $3
  AND measured_at < $4
ORDER BY measured_at DESC
LIMIT $5;

-- name: DeleteVitalReading :exec
DELETE FROM vital_readings
WHERE id = $1 AND user_id = $2;

-- name: GetLatestVitalByType :many
SELECT DISTINCT ON (vital_type) *
FROM vital_readings
WHERE user_id = $1
ORDER BY vital_type, measured_at DESC;

-- name: GetVitalSummary :many
SELECT
    date_trunc($5::text, measured_at) AS bucket,
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
