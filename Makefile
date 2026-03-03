ifneq (,$(wildcard .env))
include .env
export
endif

GOOSE_DRIVER ?= postgres
GOOSE_DBSTRING ?= $(DB_URL)
GOOSE_MIGRATION_DIR ?= internal/infra/db/postgres/migrations

.PHONY: install run test sqlc-generate migrate-up migrate-down migrate-status dev-up dev-down dev-logs check-goose-dbstring

run:
	air -c .air.toml

install:
	go install github.com/air-verse/air@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/pressly/goose/v3/cmd/goose@latest

test:
	go test ./...

sqlc-generate:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate

check-goose-dbstring:
	@if [ -z "$(GOOSE_DBSTRING)" ]; then \
		echo "GOOSE_DBSTRING is empty. Set DB_URL in .env or run:"; \
		echo "make migrate-up DB_URL='postgres://postgres:postgres@localhost:5432/vocynex?sslmode=disable'"; \
		exit 1; \
	fi

migrate-up: check-goose-dbstring
	GOOSE_DRIVER=$(GOOSE_DRIVER) GOOSE_DBSTRING='$(GOOSE_DBSTRING)' GOOSE_MIGRATION_DIR=$(GOOSE_MIGRATION_DIR) go run github.com/pressly/goose/v3/cmd/goose@latest up

migrate-down: check-goose-dbstring
	GOOSE_DRIVER=$(GOOSE_DRIVER) GOOSE_DBSTRING='$(GOOSE_DBSTRING)' GOOSE_MIGRATION_DIR=$(GOOSE_MIGRATION_DIR) go run github.com/pressly/goose/v3/cmd/goose@latest down

migrate-status: check-goose-dbstring
	GOOSE_DRIVER=$(GOOSE_DRIVER) GOOSE_DBSTRING='$(GOOSE_DBSTRING)' GOOSE_MIGRATION_DIR=$(GOOSE_MIGRATION_DIR) go run github.com/pressly/goose/v3/cmd/goose@latest status

dev-up:
	docker compose up -d postgres redis hyperdx otel-collector

dev-down:
	docker compose down

dev-logs:
	docker compose logs -f postgres redis hyperdx otel-collector
