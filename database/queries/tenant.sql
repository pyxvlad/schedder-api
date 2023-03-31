
-- name: CreateTenantWithAccount :one
WITH tmp AS (
	INSERT into tenants (tenant_name) VALUES ($2) RETURNING tenant_id
), is_business AS (
	SELECT is_business FROM accounts WHERE account_id = $1
)
INSERT INTO tenant_accounts (tenant_id, account_id) SELECT tenant_id, $1 FROM tmp, is_business WHERE is_business.is_business = true RETURNING tenant_id;

-- name: GetTenants :many
SELECT tenant_id, tenant_name FROM tenants;

-- name: GetTenantMembers :many
SELECT accounts.account_id, account_name, email, phone, is_admin FROM accounts JOIN tenant_accounts ON tenant_id = $1;

-- name: AddTenantMember :exec
WITH tmp AS (
	SELECT is_admin FROM tenant_accounts WHERE tenant_id = $1 AND tenant_accounts.account_id = $2
)
INSERT INTO tenant_accounts (tenant_id, account_id, is_admin) SELECT $1, $3, $4 FROM tmp WHERE tmp.is_admin = true;

