
-- name: AddFavourite :exec
INSERT INTO favourites (tenant_id, account_id) VALUES (@tenant_id, @account_id);

-- name: GetFavourites :many
SELECT tenant_id FROM favourites WHERE account_id = @account_id;

-- name: RemoveFavourite :exec
DELETE FROM favourites WHERE tenant_id = @tenant_id AND account_id = @account_id;

