
-- name: CreateTenantWithAccount :one
WITH tmp AS (
	INSERT into tenants (tenant_name) VALUES ($2) RETURNING tenant_id
), is_business AS (
	SELECT is_business FROM accounts WHERE account_id = $1
)
INSERT INTO tenant_accounts (tenant_id, account_id, is_manager) SELECT tenant_id, $1, true FROM tmp, is_business WHERE is_business.is_business = true RETURNING tenant_id;

-- name: GetTenants :many
SELECT tenant_id, tenant_name FROM tenants;

-- name: IsTenantManager :one
SELECT is_manager FROM tenant_accounts WHERE tenant_id = $1 AND account_id = $2;

-- name: AddTenantMember :exec
WITH tmp AS (
	SELECT is_manager FROM tenant_accounts WHERE tenant_id = @tenant_id AND tenant_accounts.account_id = @owner_id
)
INSERT INTO tenant_accounts (tenant_id, account_id, is_manager) SELECT @tenant_id, @new_member_id, @is_manager FROM tmp WHERE tmp.is_manager = true;

-- name: GetTenantMembers :many
SELECT accounts.account_id, account_name, email, phone, is_manager FROM accounts JOIN tenant_accounts ON tenant_id = $1;

