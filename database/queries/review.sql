
-- name: CreateReview :exec
INSERT INTO reviews(account_id, tenant_id, message, rating) VALUES (@account_id, @tenant_id, @message, @rating);

-- name: Reviews :many
SELECT review_id, account_id, message, rating FROM reviews WHERE tenant_id = @tenant_id;

