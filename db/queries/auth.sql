-- name: GetUserByProviderIDForAuth :one
SELECT id, provider_id, email, name FROM users
WHERE provider_id = $1;

-- name: UpsertUserFromProvider :one
INSERT INTO users (provider_id, email, name)
VALUES ($1, $2, $3)
ON CONFLICT (provider_id) DO UPDATE
SET email = EXCLUDED.email,
    name = EXCLUDED.name,
    updated_at = NOW()
RETURNING *;
