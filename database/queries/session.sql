
-- name: CreateSessionToken :one
INSERT INTO sessions (account_id) VALUES ($1) RETURNING token;

-- name: GetSessionAccount :one
SELECT account_id FROM sessions WHERE token = $1 AND expiration_date > NOW() AND valid = true LIMIT 1;


