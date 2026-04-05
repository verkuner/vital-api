-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByProviderID :one
SELECT * FROM users WHERE provider_id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: CreateUser :one
INSERT INTO users (provider_id, email, name, date_of_birth)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET name = COALESCE(sqlc.narg('name'), name),
    date_of_birth = COALESCE(sqlc.narg('date_of_birth'), date_of_birth),
    avatar_url = COALESCE(sqlc.narg('avatar_url'), avatar_url),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: ListDevicesByUser :many
SELECT * FROM devices WHERE user_id = $1 ORDER BY created_at DESC;

-- name: CreateDevice :one
INSERT INTO devices (user_id, name, type)
VALUES ($1, $2, $3)
RETURNING *;

-- name: DeleteDevice :exec
DELETE FROM devices WHERE id = $1 AND user_id = $2;
