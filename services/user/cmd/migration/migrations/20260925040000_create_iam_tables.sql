-- +goose Up
-- +goose StatementBegin
DROP TABLE IF EXISTS users; -- placeholder table from the initial scaffold

CREATE TABLE realms (
	"id" SERIAL NOT NULL PRIMARY KEY,
	"name" VARCHAR(63) NOT NULL,
	"display_name" VARCHAR(255) NOT NULL DEFAULT '',
	"enabled" BOOLEAN NOT NULL DEFAULT TRUE,
	"access_token_ttl_sec" INTEGER NOT NULL DEFAULT 900,
	"refresh_token_ttl_sec" INTEGER NOT NULL DEFAULT 2592000,
	"max_failed_attempts" INTEGER NOT NULL DEFAULT 5,
	"lockout_sec" INTEGER NOT NULL DEFAULT 900,
	"otp_login_enabled" BOOLEAN NOT NULL DEFAULT FALSE,
	"created_at" TIMESTAMPTZ(6),
	"updated_at" TIMESTAMPTZ(6),
	"deleted_at" TIMESTAMPTZ(6)
);
CREATE UNIQUE INDEX uq_realms_name ON realms ("name") WHERE deleted_at IS NULL;

CREATE TABLE realm_keys (
	"id" SERIAL NOT NULL PRIMARY KEY,
	"realm_id" INTEGER NOT NULL REFERENCES realms ("id"),
	"kid" VARCHAR(64) NOT NULL UNIQUE,
	"algorithm" VARCHAR(16) NOT NULL DEFAULT 'RS256',
	"public_key_pem" TEXT NOT NULL,
	"private_key_enc" TEXT NOT NULL,
	"active" BOOLEAN NOT NULL DEFAULT TRUE,
	"created_at" TIMESTAMPTZ(6),
	"updated_at" TIMESTAMPTZ(6)
);
CREATE INDEX idx_realm_keys_realm ON realm_keys ("realm_id", "active");

CREATE TABLE users (
	"id" SERIAL NOT NULL PRIMARY KEY,
	"realm_id" INTEGER NOT NULL REFERENCES realms ("id"),
	"username" VARCHAR(100) NOT NULL,
	"email" VARCHAR(255),
	"phone" VARCHAR(30),
	"full_name" VARCHAR(255) NOT NULL DEFAULT '',
	"password_hash" VARCHAR(255) NOT NULL DEFAULT '',
	"status" VARCHAR(16) NOT NULL DEFAULT 'active', -- active | disabled | locked
	"failed_attempts" INTEGER NOT NULL DEFAULT 0,
	"locked_until" TIMESTAMPTZ(6),
	"last_login_at" TIMESTAMPTZ(6),
	"is_service_account" BOOLEAN NOT NULL DEFAULT FALSE,
	"created_at" TIMESTAMPTZ(6),
	"updated_at" TIMESTAMPTZ(6),
	"deleted_at" TIMESTAMPTZ(6)
);
CREATE UNIQUE INDEX uq_users_realm_username ON users ("realm_id", "username") WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uq_users_realm_email ON users ("realm_id", "email") WHERE deleted_at IS NULL AND email IS NOT NULL;
CREATE UNIQUE INDEX uq_users_realm_phone ON users ("realm_id", "phone") WHERE deleted_at IS NULL AND phone IS NOT NULL;

CREATE TABLE clients (
	"id" SERIAL NOT NULL PRIMARY KEY,
	"realm_id" INTEGER NOT NULL REFERENCES realms ("id"),
	"client_id" VARCHAR(100) NOT NULL,
	"name" VARCHAR(255) NOT NULL DEFAULT '',
	"description" TEXT NOT NULL DEFAULT '',
	"type" VARCHAR(16) NOT NULL DEFAULT 'public', -- public | confidential
	"secret_hash" VARCHAR(255) NOT NULL DEFAULT '',
	"grant_types" VARCHAR(255) NOT NULL DEFAULT 'password,refresh_token', -- comma separated
	"enabled" BOOLEAN NOT NULL DEFAULT TRUE,
	"service_user_id" INTEGER REFERENCES users ("id"),
	"created_at" TIMESTAMPTZ(6),
	"updated_at" TIMESTAMPTZ(6),
	"deleted_at" TIMESTAMPTZ(6)
);
CREATE UNIQUE INDEX uq_clients_realm_client_id ON clients ("realm_id", "client_id") WHERE deleted_at IS NULL;

CREATE TABLE roles (
	"id" SERIAL NOT NULL PRIMARY KEY,
	"realm_id" INTEGER NOT NULL REFERENCES realms ("id"),
	"name" VARCHAR(100) NOT NULL,
	"description" TEXT NOT NULL DEFAULT '',
	"created_at" TIMESTAMPTZ(6),
	"updated_at" TIMESTAMPTZ(6),
	"deleted_at" TIMESTAMPTZ(6)
);
CREATE UNIQUE INDEX uq_roles_realm_name ON roles ("realm_id", "name") WHERE deleted_at IS NULL;

CREATE TABLE permissions (
	"id" SERIAL NOT NULL PRIMARY KEY,
	"realm_id" INTEGER NOT NULL REFERENCES realms ("id"),
	"service" VARCHAR(63) NOT NULL, -- '*' matches every service
	"code" VARCHAR(100) NOT NULL,   -- '*' matches every code of the service
	"type" VARCHAR(8) NOT NULL DEFAULT 'api', -- api | ui
	"description" TEXT NOT NULL DEFAULT '',
	"created_at" TIMESTAMPTZ(6),
	"updated_at" TIMESTAMPTZ(6),
	"deleted_at" TIMESTAMPTZ(6)
);
CREATE UNIQUE INDEX uq_permissions_realm_service_code ON permissions ("realm_id", "service", "code") WHERE deleted_at IS NULL;

CREATE TABLE role_permissions (
	"role_id" INTEGER NOT NULL REFERENCES roles ("id"),
	"permission_id" INTEGER NOT NULL REFERENCES permissions ("id"),
	PRIMARY KEY ("role_id", "permission_id")
);

CREATE TABLE user_roles (
	"user_id" INTEGER NOT NULL REFERENCES users ("id"),
	"role_id" INTEGER NOT NULL REFERENCES roles ("id"),
	PRIMARY KEY ("user_id", "role_id")
);

CREATE TABLE menus (
	"id" SERIAL NOT NULL PRIMARY KEY,
	"realm_id" INTEGER NOT NULL REFERENCES realms ("id"),
	"client_id" INTEGER NOT NULL REFERENCES clients ("id"),
	"parent_id" INTEGER REFERENCES menus ("id"),
	"key" VARCHAR(100) NOT NULL,
	"label" VARCHAR(255) NOT NULL,
	"path" VARCHAR(255) NOT NULL DEFAULT '',
	"icon" VARCHAR(100) NOT NULL DEFAULT '',
	"sort_order" INTEGER NOT NULL DEFAULT 0,
	"permission_id" INTEGER REFERENCES permissions ("id"),
	"created_at" TIMESTAMPTZ(6),
	"updated_at" TIMESTAMPTZ(6),
	"deleted_at" TIMESTAMPTZ(6)
);
CREATE UNIQUE INDEX uq_menus_client_key ON menus ("client_id", "key") WHERE deleted_at IS NULL;

CREATE TABLE sessions (
	"id" SERIAL NOT NULL PRIMARY KEY,
	"realm_id" INTEGER NOT NULL REFERENCES realms ("id"),
	"user_id" INTEGER NOT NULL REFERENCES users ("id"),
	"client_id" INTEGER NOT NULL REFERENCES clients ("id"),
	"family_id" INTEGER NOT NULL, -- first session id of the rotation chain; used as JWT "sid"
	"refresh_hash" VARCHAR(64) NOT NULL UNIQUE,
	"expires_at" TIMESTAMPTZ(6) NOT NULL,
	"rotated_at" TIMESTAMPTZ(6),
	"revoked_at" TIMESTAMPTZ(6),
	"ip" VARCHAR(64) NOT NULL DEFAULT '',
	"user_agent" VARCHAR(255) NOT NULL DEFAULT '',
	"created_at" TIMESTAMPTZ(6),
	"updated_at" TIMESTAMPTZ(6)
);
CREATE INDEX idx_sessions_family ON sessions ("family_id");
CREATE INDEX idx_sessions_user ON sessions ("user_id");
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS menus;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS clients;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS realm_keys;
DROP TABLE IF EXISTS realms;
-- +goose StatementEnd
