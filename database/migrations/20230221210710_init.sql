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
	password text DEFAULT NULL,
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

CREATE TYPE verification_scope AS ENUM ('register', 'passwordless_login');

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

-- CREATE TYPE weekdays AS ENUM ('monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday');
-- See https://pkg.go.dev/time#Weekday
CREATE DOMAIN weekdays AS int CHECK (VALUE IN (0, 1, 2, 3, 4, 5, 6));

CREATE TABLE schedules (
	account_id uuid REFERENCES accounts(account_id) NOT NULL,
	weekday weekdays NOT NULL,
	starting_time timetz NOT NULL,
	ending_time timetz NOT NULL,

	PRIMARY KEY(account_id, weekday),
	CHECK(starting_time < ending_time)
);

CREATE TABLE services (
	service_id uuid DEFAULT gen_random_uuid() NOT NULL,

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

CREATE TYPE appointment_status AS ENUM ('pending', 'cancelled', 'done');
CREATE TABLE appointments (
	appointment_id uuid DEFAULT gen_random_uuid() NOT NULL,
	service_id uuid REFERENCES services(service_id) NOT NULL,

	account_id uuid REFERENCES accounts(account_id) NOT NULL,

	starting timestamptz NOT NULL,

	status appointment_status DEFAULT 'pending' NOT NULL,

	PRIMARY KEY(appointment_id),

	-- check that the starting time is a multiple of 30 minutes as a Unix Timestamp
	CONSTRAINT starting_30mins_multiple CHECK((FLOOR(EXTRACT(epoch FROM starting)/60) % 30) = 0),


	-- check that there is a unique combination of account_id & starting
	CONSTRAINT unique_starting_for_account UNIQUE(account_id, starting)
);

CREATE TABLE reviews (
	review_id uuid DEFAULT gen_random_uuid() NOT NULL,
	account_id uuid REFERENCES accounts(account_id) NOT NULL,
	tenant_id uuid REFERENCES tenants(tenant_id) NOT NULL,
	message varchar(4096) NOT NULL,
	rating int NOT NULL CHECK(rating > 0 AND rating < 6),

	CONSTRAINT unique_account_and_tenant UNIQUE(account_id, tenant_id),
	PRIMARY KEY(review_id)
);

CREATE TABLE favourites (
	account_id uuid REFERENCES accounts(account_id) NOT NULL,
	tenant_id uuid REFERENCES tenants(tenant_id) NOT NULL,

	PRIMARY KEY(account_id, tenant_id)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS favourites;
DROP TABLE IF EXISTS reviews;
DROP TABLE IF EXISTS appointments;
DROP TYPE IF EXISTS appointment_status;
DROP TABLE IF EXISTS services;
DROP TABLE IF EXISTS schedules;
DROP TYPE IF EXISTS weekdays;
DROP TABLE IF EXISTS tenant_photos;
DROP TABLE IF EXISTS tenant_accounts;
DROP TABLE IF EXISTS tenants;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS verification_codes;
DROP TYPE IF EXISTS verification_scope;
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS photos;
-- +goose StatementEnd
