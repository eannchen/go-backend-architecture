GOOSE_DRIVER ?= postgres
GOOSE_DBSTRING ?= $(DB_URL)
GOOSE_MIGRATION_DIR ?= internal/infra/db/postgres/migrations

.PHONY: run test sqlc-generate migrate-up migrate-down migrate-status

run:
	go run ./cmd/api

test:
	go test ./...

sqlc-generate:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate

migrate-up:
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir $(GOOSE_MIGRATION_DIR) $(GOOSE_DRIVER) $(GOOSE_DBSTRING) up

migrate-down:
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir $(GOOSE_MIGRATION_DIR) $(GOOSE_DRIVER) $(GOOSE_DBSTRING) down

migrate-status:
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir $(GOOSE_MIGRATION_DIR) $(GOOSE_DRIVER) $(GOOSE_DBSTRING) status
