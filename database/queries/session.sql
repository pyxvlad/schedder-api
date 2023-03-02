
-- name: CreateSessionToken :one
INSERT INTO sessions (account_id) VALUES ($1) RETURNING token;

-- name: GetSessionAccount :one
SELECT account_id FROM sessions WHERE token = $1 AND expiration_date > NOW() AND revoked = false LIMIT 1;

-- name: GetSessionsForAccount :many
SELECT session_id, expiration_date FROM sessions WHERE account_id = $1 AND expiration_date > NOW() AND revoked = false;

-- name: RevokeSessionForAccount :execrows
UPDATE sessions SET revoked = true WHERE session_id = $1 AND account_id = $2;

