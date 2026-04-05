-- name: CreateAlert :one
INSERT INTO alerts (user_id, vital_type, value, threshold_breached, threshold_value, severity)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetAlertByID :one
SELECT * FROM alerts
WHERE id = $1 AND user_id = $2;

-- name: ListAlerts :many
SELECT * FROM alerts
WHERE user_id = $1
  AND ($2::varchar IS NULL OR severity = $2)
  AND ($3::bool IS NULL OR acknowledged = $3)
  AND ($4::varchar IS NULL OR vital_type = $4)
  AND created_at >= $5
  AND created_at <= $6
ORDER BY created_at DESC
LIMIT $7 OFFSET $8;

-- name: AcknowledgeAlert :one
UPDATE alerts
SET acknowledged = TRUE, acknowledged_at = NOW()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: CountUnacknowledgedAlerts :one
SELECT COUNT(*) FROM alerts
WHERE user_id = $1 AND acknowledged = FALSE;

-- name: GetThresholdByUserAndType :one
SELECT * FROM alert_thresholds
WHERE user_id = $1 AND vital_type = $2;

-- name: ListThresholdsByUser :many
SELECT * FROM alert_thresholds
WHERE user_id = $1
ORDER BY vital_type;

-- name: UpsertThreshold :one
INSERT INTO alert_thresholds (user_id, vital_type, low_value, high_value, enabled)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id, vital_type) DO UPDATE
SET low_value = EXCLUDED.low_value,
    high_value = EXCLUDED.high_value,
    enabled = EXCLUDED.enabled
RETURNING *;

-- name: DeleteThreshold :exec
DELETE FROM alert_thresholds
WHERE user_id = $1 AND vital_type = $2;
