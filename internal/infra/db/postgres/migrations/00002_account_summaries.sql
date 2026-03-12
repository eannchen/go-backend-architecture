-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS account_summaries (
    id BIGINT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    plan TEXT NOT NULL,
    status TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS account_summaries;
-- +goose StatementEnd
