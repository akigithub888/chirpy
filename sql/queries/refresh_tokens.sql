-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (
    token,
    created_at,
    updated_at,
    user_id,
    expires_at,
    revoked_at
)
VALUES ($1, NOW(), NOW(), $2, $3, NULL)
RETURNING token;

-- name: GetUserFromRefreshToken :one
SELECT
    u.id,
    u.email,
    u.created_at,
    u.updated_at,
    r.token,
    r.expires_at,
    r.revoked_at
FROM refresh_tokens r
JOIN users u ON r.user_id = u.id
WHERE r.token = $1;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET revoked_at = NOW(),
    updated_at = NOW()
WHERE token = $1;

-- name: DeleteAllRefreshTokens :exec
DELETE FROM refresh_tokens;
