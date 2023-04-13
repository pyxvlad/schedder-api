-- name: CreatePhoto :one
INSERT INTO photos(sha256sum) VALUES (@sha256sum) RETURNING photo_id;

-- name: AddTenantPhoto :one
WITH tmp AS (
	INSERT INTO photos(sha256sum) VALUES (@sha256sum) RETURNING photo_id
)
INSERT INTO tenant_photos(tenant_id, photo_id) SELECT @tenant_id, photo_id FROM tmp RETURNING photo_id;

-- name: ListTenantPhotos :many
SELECT photo_id FROM tenant_photos WHERE tenant_id = @tenant_id;

-- name: GetTenantPhotoHash :one
SELECT sha256sum FROM photos JOIN tenant_photos ON photos.photo_id = tenant_photos.photo_id WHERE photos.photo_id = @photo_id AND tenant_id = @tenant_id;

-- name: SetProfilePhoto :exec
WITH tmp AS (
	INSERT INTO photos(sha256sum) VALUES (@sha256sum) RETURNING photo_id
)
UPDATE accounts SET photo_id = tmp.photo_id FROM tmp WHERE account_id = @account_id;

-- name: GetProfilePhotoHash :one
SELECT sha256sum FROM photos JOIN accounts ON photos.photo_id = accounts.photo_id WHERE account_id = @account_id;


