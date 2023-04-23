
-- name: CreateAccountWithEmail :one
INSERT INTO accounts (email, password, account_name) VALUES ($1, $2, $3) RETURNING account_id, email, phone, account_name;

-- name: CreateAccountWithPhone :one
INSERT INTO accounts (phone, password, account_name) VALUES ($1, $2, $3) RETURNING account_id, email, phone, account_name;

-- name: CreatePasswordlessAccountWithPhone :one
INSERT INTO accounts (phone, password, account_name) VALUES ($1, NULL, $2) RETURNING account_id, email, phone, account_name;

-- name: ActivateAccount :exec
UPDATE accounts SET activated = true WHERE account_id = $1;

-- name: GetPasswordByEmail :one
SELECT account_id, password FROM accounts WHERE email = $1 AND activated = true;

-- name: GetPasswordByPhone :one
SELECT account_id, password FROM accounts WHERE phone = $1 AND activated = true;

-- name: SetAdminForAccount :exec
UPDATE accounts SET is_admin = $2 WHERE account_id = $1;

-- name: SetBusinessForAccount :exec
UPDATE accounts SET is_business = $2 WHERE account_id = $1;


-- name: GetAdminForAccount :one
SELECT is_admin FROM accounts WHERE account_id = $1;


-- name: FindAccountByEmail :one
SELECT * FROM accounts WHERE email = $1;

-- name: FindAccountByPhone :one
SELECT * FROM accounts WHERE phone = $1;

