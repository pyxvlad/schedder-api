
-- name: GetVerificationCodeScope :one
SELECT scope FROM verification_codes WHERE account_id = $1
	AND verification_code = $2 AND expiration_date > NOW() AND used = False;

-- name: CreateVerificationCode :exec
INSERT INTO verification_codes (
	account_id, verification_code, scope
) VALUES ( $1, $2, $3 );
