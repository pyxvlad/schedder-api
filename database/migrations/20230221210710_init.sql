-- +goose Up
-- +goose StatementBegin

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE photos (
	photo_id uuid DEFAULT gen_random_uuid(),
	sha256sum bytea NOT NULL,

	PRIMARY KEY(photo_id)
);

CREATE TABLE accounts (
	account_id uuid DEFAULT gen_random_uuid(),
	password text NOT NULL,
	email text UNIQUE,
	phone text UNIQUE,
	account_name text NOT NULL,

	is_business boolean DEFAULT FALSE NOT NULL,
	is_admin boolean DEFAULT FALSE NOT NULL,
	activated boolean DEFAULT FALSE NOT NULL,

	photo_id UUID REFERENCES photos(photo_id) DEFAULT NULL,

	PRIMARY KEY(account_id),
	-- check that at least one of exists: email, phone
	CONSTRAINT email_or_phone CHECK(email != NULL OR phone != NULL)
);

CREATE TYPE verification_scope AS ENUM ('register');

CREATE TABLE verification_codes (
	account_id UUID REFERENCES accounts(account_id) NOT NULL,
	verification_code text NOT NULL,
	scope verification_scope NOT NULL,
	expiration_date timestamp NOT NULL DEFAULT (NOW() + interval '15m'),
	used boolean DEFAULT FALSE NOT NULL,

	PRIMARY KEY(account_id, verification_code)
);

CREATE TABLE sessions (
	session_id uuid DEFAULT gen_random_uuid(),
	token bytea UNIQUE DEFAULT gen_random_bytes(64),
	account_id uuid REFERENCES accounts(account_id) NOT NULL,

	ip inet NOT NULL,
	device text NOT NULL,

	expiration_date timestamp NOT NULL DEFAULT (NOW() + interval '7d'),
	revoked boolean DEFAULT FALSE NOT NULL,

	CHECK(length(token) >= 64),
	PRIMARY KEY(session_id)
);

CREATE TABLE tenants (
	tenant_id uuid DEFAULT gen_random_uuid(),
	tenant_name text NOT NULL,

	PRIMARY KEY(tenant_id)
);

CREATE TABLE tenant_accounts (
	tenant_id uuid REFERENCES tenants(tenant_id) NOT NULL,
	account_id uuid REFERENCES accounts(account_id) NOT NULL,

	is_manager boolean DEFAULT FALSE NOT NULL,

	PRIMARY KEY(tenant_id, account_id)
);



CREATE TABLE tenant_photos (
	tenant_id uuid REFERENCES tenants(tenant_id) NOT NULL,
	photo_id uuid REFERENCES photos(photo_id) NOT NULL,

	PRIMARY KEY(tenant_id, photo_id)
);

CREATE TABLE services (
	service_id uuid DEFAULT gen_random_uuid(),

	tenant_id uuid REFERENCES tenants(tenant_id) NOT NULL,
	account_id uuid REFERENCES accounts(account_id) NOT NULL,

	service_name text NOT NULL,
	price numeric NOT NULL,
	
	duration interval NOT NULL,

	FOREIGN KEY(tenant_id, account_id) REFERENCES tenant_accounts(tenant_id, account_id),
	-- check that duration as minutes is a multiple of 30
	CONSTRAINT duration_30mins_multiple CHECK(((EXTRACT(epoch from duration::interval)/60) % 30) = 0),
	-- check that duration is greater than 0 minutes
	CONSTRAINT duration_gt_zero CHECK(EXTRACT(epoch from duration::interval) > 0),
	-- check that there is a unique combination of tenant_id, account_id, service_name
	CONSTRAINT unique_service_name_for_tenant_user UNIQUE(tenant_id, account_id, service_name),

	PRIMARY KEY(service_id)
);


-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE services;
DROP TABLE tenant_photos;
DROP TABLE tenant_accounts;
DROP TABLE tenants;
DROP TABLE sessions;
DROP TABLE verification_codes;
DROP TYPE verification_scope;
DROP TABLE accounts;
DROP TABLE photos;
-- +goose StatementEnd
