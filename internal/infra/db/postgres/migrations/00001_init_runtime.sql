-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS system_runtime_kv (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS system_runtime_kv;
-- +goose StatementEnd
