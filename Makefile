ifneq (,$(wildcard .env))
include .env
export
endif

GOOSE_DRIVER ?= postgres
GOOSE_DBSTRING ?= $(DB_URL)
GOOSE_MIGRATION_DIR ?= $(CURDIR)/internal/infra/db/postgres/migrations
GO_TEST ?= go test
AIR_HTTPAPI_CMD ?= air -c .air.toml
AIR_GRPCAPI_CMD ?= air -c .air.grpcapi.toml
OAPI_CODEGEN_CMD ?= github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
BUF_CMD ?= github.com/bufbuild/buf/cmd/buf@v1.72.0
# `?=` sets a default that a developer or CI can override on the command line.
# BASE_REF is a local Git name such as main, a tag, or a commit hash. AGAINST is
# Buf's description of the baseline: `.git` means this repository, `ref=` selects
# that Git version, and `subdir=` selects its contracts/grpc directory. Make treats
# `#` as a comment marker, so `\#` preserves the literal `#` that Buf requires.
PROTO_BREAKING_BASE_REF ?= main
PROTO_BREAKING_AGAINST ?= .git\#ref=$(PROTO_BREAKING_BASE_REF),subdir=contracts/grpc
PROTO_GENERATED_DIR ?= internal/delivery/grpc/gen
SQLC_CMD ?= github.com/sqlc-dev/sqlc/cmd/sqlc@latest
GOOSE_CMD ?= github.com/pressly/goose/v3/cmd/goose@latest
GOOSE_RUN = GOOSE_DRIVER=$(GOOSE_DRIVER) GOOSE_DBSTRING='$(GOOSE_DBSTRING)' GOOSE_MIGRATION_DIR=$(GOOSE_MIGRATION_DIR) go run $(GOOSE_CMD)
INTEGRATION_PACKAGES := \
	./internal/infra/db/postgres/store \
	./internal/infra/cache/redis/store \
	./internal/infra/kvstore/redis/store \
	./internal/delivery/http/integration

.PHONY: install run run-httpapi run-httpapi-stop run-grpcapi run-grpcapi-stop fmt-check vet build check test test-cover test-race test-grpc test-integration test-all ci openapi-generate proto-generate proto-lint proto-breaking proto-generated-check sqlc-generate migrate-up migrate-down migrate-status dev-up dev-down dev-logs check-goose-dbstring openapi proto proto-check sqlc mup mdown mstatus

run: run-httpapi

run-httpapi:
	$(AIR_HTTPAPI_CMD)

run-httpapi-stop:
	@pids=$$(lsof -tiTCP:8080 -sTCP:LISTEN); \
	if [ -n "$$pids" ]; then \
		echo "Stopping process(es) on :8080 -> $$pids"; \
		kill $$pids; \
	else \
		echo "No process is listening on :8080"; \
	fi

run-grpcapi:
	$(AIR_GRPCAPI_CMD)

run-grpcapi-stop:
	@pids=$$(lsof -tiTCP:9090 -sTCP:LISTEN); \
	if [ -n "$$pids" ]; then \
		echo "Stopping process(es) on :9090 -> $$pids"; \
		kill $$pids; \
	else \
		echo "No process is listening on :9090"; \
	fi

install:
	go install github.com/air-verse/air@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/pressly/goose/v3/cmd/goose@latest
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
	go install github.com/bufbuild/buf/cmd/buf@v1.72.0

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

test-grpc:
	$(GO_TEST) ./internal/delivery/grpc/...

test-integration:
	# Disabling the cache proves container startup, migrations, and cleanup on every run.
	$(GO_TEST) -count=1 -tags=integration $(INTEGRATION_PACKAGES)

test-all: test test-integration

ci: proto-check check test-race test-integration

openapi-generate:
	go run $(OAPI_CODEGEN_CMD) -config oapi-codegen.yaml contracts/http/openapi.yaml

proto-generate:
	go run $(BUF_CMD) generate

proto-lint:
	go run $(BUF_CMD) lint

# Compare the working-tree protobuf contract with a historical contract without
# modifying either version. The FILE breaking policy in buf.yaml permits compatible
# additions, but rejects changes that can break generated clients or wire data, such
# as removing or renaming definitions, reusing field numbers, changing field types,
# or changing an RPC signature. CI uses the exact PR base or pre-push commit so the
# comparison does not depend on a moving branch name.
#
# Syntax used below:
# - `git rev-parse ... '<ref>^{commit}'` validates the ref and converts it to one
#   immutable commit hash. `--quiet` suppresses Git's technical error when the ref
#   is missing.
# - `[ -z "$$base_commit" ]` is true when the shell variable is an empty string,
#   which means Git could not resolve the requested baseline.
# - `git ls-tree` reads file names from that committed snapshot without switching
#   branches. `-r` walks subdirectories, `--name-only` omits Git object metadata,
#   and `-- contracts/grpc` limits the listing to that path.
# - `grep -q '\.proto$$'` silently checks whether the baseline contains any path
#   ending in `.proto`. Make consumes one `$`, so `$$` sends one regex `$` to grep.
# - `$$base_commit` likewise sends `$base_commit` to the shell instead of asking
#   Make to expand it. The leading `@` stops Make from printing the full command
#   before running it, leaving only useful results and error messages in the output.
# - If the baseline has no protobuf file, this is the first contract and there is
#   nothing older to protect. Otherwise Buf compares the working tree with that
#   snapshot using the compatibility rules configured in buf.yaml.
proto-breaking:
	@base_commit="$$(git rev-parse --verify --quiet '$(PROTO_BREAKING_BASE_REF)^{commit}')"; \
	if [ -z "$$base_commit" ]; then \
		echo "Protobuf breaking-check baseline '$(PROTO_BREAKING_BASE_REF)' is not available locally."; \
		exit 1; \
	fi; \
	if git ls-tree -r --name-only "$$base_commit" -- contracts/grpc | grep -q '\.proto$$'; then \
		go run $(BUF_CMD) breaking --against '$(PROTO_BREAKING_AGAINST)'; \
	else \
		echo "Skipping protobuf breaking check: baseline '$(PROTO_BREAKING_BASE_REF)' predates the first gRPC contract."; \
	fi

proto-generated-check: proto-generate
	@changes="$$(git status --short -- $(PROTO_GENERATED_DIR))"; \
	if [ -n "$$changes" ]; then \
		echo "Generated protobuf files are out of date. Run 'make proto-generate' and commit the result:"; \
		echo "$$changes"; \
		exit 1; \
	fi

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
proto: proto-generate
proto-check: proto-lint proto-breaking proto-generated-check
sqlc: sqlc-generate
mup: migrate-up
mdown: migrate-down
mstatus: migrate-status
