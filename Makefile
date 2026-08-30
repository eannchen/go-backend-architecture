ifneq (,$(wildcard .env))
include .env
export
endif

GOOSE_DRIVER ?= postgres
GOOSE_DBSTRING ?= $(DB_URL)
GOOSE_MIGRATION_DIR ?= $(CURDIR)/internal/infra/db/postgres/migrations
GO_TEST ?= go test
AIR_CMD ?= air -c .air.toml
OAPI_CODEGEN_CMD ?= github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
SQLC_CMD ?= github.com/sqlc-dev/sqlc/cmd/sqlc@latest
GOOSE_CMD ?= github.com/pressly/goose/v3/cmd/goose@latest
GOOSE_RUN = GOOSE_DRIVER=$(GOOSE_DRIVER) GOOSE_DBSTRING='$(GOOSE_DBSTRING)' GOOSE_MIGRATION_DIR=$(GOOSE_MIGRATION_DIR) go run $(GOOSE_CMD)
INTEGRATION_PACKAGES := \
	./internal/infra/db/postgres/store \
	./internal/infra/cache/redis/store \
	./internal/infra/kvstore/redis/store \
	./internal/delivery/http/integration

.PHONY: install run run-stop fmt-check vet build check test test-cover test-race test-integration test-all ci openapi-generate sqlc-generate migrate-up migrate-down migrate-status dev-up dev-down dev-logs check-goose-dbstring openapi sqlc mup mdown mstatus

run:
	$(AIR_CMD)

run-stop:
	@pids=$$(lsof -tiTCP:8080 -sTCP:LISTEN); \
	if [ -n "$$pids" ]; then \
		echo "Stopping process(es) on :8080 -> $$pids"; \
		kill $$pids; \
	else \
		echo "No process is listening on :8080"; \
	fi

install:
	go install github.com/air-verse/air@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/pressly/goose/v3/cmd/goose@latest
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

fmt-check:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "The following Go files need gofmt:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

vet:
	go vet ./...

build:
	go build ./...

check: fmt-check vet build

test:
	$(GO_TEST) ./...

test-cover:
	$(GO_TEST) -coverprofile=coverage.out ./...

test-race:
	$(GO_TEST) -race ./...

test-integration:
	# Disabling the cache proves container startup, migrations, and cleanup on every run.
	$(GO_TEST) -count=1 -tags=integration $(INTEGRATION_PACKAGES)

test-all: test test-integration

ci: check test-race test-integration

openapi-generate:
	go run $(OAPI_CODEGEN_CMD) -config oapi-codegen.yaml docs/openapi.yaml

sqlc-generate:
	go run $(SQLC_CMD) generate

check-goose-dbstring:
	@if [ -z "$(GOOSE_DBSTRING)" ]; then \
		echo "GOOSE_DBSTRING is empty. Set DB_URL in .env or run:"; \
		echo "make migrate-up DB_URL='postgres://postgres:postgres@localhost:5432/go-backend-architecture?sslmode=disable'"; \
		exit 1; \
	fi

migrate-up: check-goose-dbstring
	$(GOOSE_RUN) up

migrate-down: check-goose-dbstring
	$(GOOSE_RUN) down

migrate-status: check-goose-dbstring
	$(GOOSE_RUN) status

dev-up:
	docker compose up -d postgres redis hyperdx otel-collector

dev-down:
	docker compose down

dev-logs:
	docker compose logs -f postgres redis hyperdx otel-collector

openapi: openapi-generate
sqlc: sqlc-generate
mup: migrate-up
mdown: migrate-down
mstatus: migrate-status
