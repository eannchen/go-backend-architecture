-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE users (
    id             INT          GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email          TEXT         NOT NULL UNIQUE,
    email_verified BOOLEAN      NOT NULL DEFAULT FALSE,
    name           TEXT,
    status         TEXT         NOT NULL DEFAULT 'active',
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE oauth_connections (
    id               INT          GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id          INT          NOT NULL REFERENCES users(id),
    provider         TEXT         NOT NULL,
    provider_user_id TEXT         NOT NULL,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE(provider, provider_user_id)
);

CREATE INDEX idx_oauth_connections_user_id ON oauth_connections(user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS oauth_connections;
DROP TABLE IF EXISTS users;
DROP EXTENSION IF EXISTS vector;
-- +goose StatementEnd
