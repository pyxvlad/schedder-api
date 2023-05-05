
-- name: CreateService :one
INSERT INTO services (
	tenant_id, account_id, service_name, price, duration
) VALUES ( @tenant_id, @account_id, @service_name, @price, @duration ) RETURNING service_id;

-- name: GetServicesForTenant :many
SELECT service_id, account_id, service_name, price, duration FROM services WHERE tenant_id = @tenant_id;

-- name: GetServices :many
SELECT service_id, service_name, price, duration FROM services WHERE tenant_id = @tenant_id AND account_id = @account_id;

-- name: GetServiceDurationAndPersonnel :one
SELECT duration, account_id FROM services WHERE service_id = @service_id;

