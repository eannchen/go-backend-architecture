-- name: GetUserByEmail :one
SELECT id, email
FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT id, email
FROM users
WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (email)
VALUES ($1)
RETURNING id, email;

-- name: UpsertOAuthConnection :one
-- Finds or creates a user by OAuth provider identity.
WITH existing AS (
    SELECT u.id, u.email
    FROM oauth_connections oc
    JOIN users u ON u.id = oc.user_id
    WHERE oc.provider = $1 AND oc.provider_user_id = $2
),
new_user AS (
    INSERT INTO users (email)
    SELECT $3
    WHERE NOT EXISTS (SELECT 1 FROM existing)
    ON CONFLICT (email) DO UPDATE SET updated_at = NOW()
    RETURNING id, email
),
target_user AS (
    SELECT id, email FROM existing
    UNION ALL
    SELECT id, email FROM new_user
    LIMIT 1
),
ensure_connection AS (
    INSERT INTO oauth_connections (user_id, provider, provider_user_id)
    SELECT id, $1, $2 FROM target_user
    ON CONFLICT (provider, provider_user_id) DO NOTHING
)
SELECT id, email FROM target_user;
