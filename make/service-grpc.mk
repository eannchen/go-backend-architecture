SERVICE_GRPC_PROFILE := true
SQLC_CONFIG ?= $(if $(wildcard sqlc.service-grpc.yaml),sqlc.service-grpc.yaml,sqlc.yaml)
AIR_GRPCAPI_CONFIG ?= $(if $(wildcard .air.grpcapi.toml),.air.grpcapi.toml,.air.toml)
AIR_GRPCAPI_CMD ?= air -c $(AIR_GRPCAPI_CONFIG)
BUF_CMD ?= github.com/bufbuild/buf/cmd/buf@v1.72.0
# `?=` permits CI to select an immutable baseline commit. Buf reads that Git
# snapshot's contracts/grpc directory without changing the working tree.
PROTO_BREAKING_BASE_REF ?= main
PROTO_BREAKING_AGAINST ?= .git\#ref=$(PROTO_BREAKING_BASE_REF),subdir=contracts/grpc
PROTO_GENERATED_DIR ?= internal/delivery/grpc/gen
CONTRACT_CHECK_TARGETS += proto-check
INSTALL_TOOLS += $(BUF_CMD)

.PHONY: run-grpcapi run-grpcapi-stop test-grpc proto-generate proto-lint proto-breaking proto-generated-check proto-check proto

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

test-grpc:
	$(GO_TEST) ./internal/delivery/grpc/...

proto-generate:
	go run $(BUF_CMD) generate

proto-lint:
	go run $(BUF_CMD) lint

# Compare the working-tree protobuf contract with a historical contract without
# modifying either version. Buf's FILE policy permits compatible additions but
# rejects removed/renamed definitions, reused field numbers, changed field types,
# and changed RPC signatures.
#
# `git rev-parse ... '<ref>^{commit}'` validates the ref and resolves it to one
# immutable commit. `--quiet` suppresses Git's low-level missing-ref message so
# this target can print a clearer one. `[ -z "$$base_commit" ]` detects that Git
# returned no commit hash. Make consumes one `$`, so `$$` passes `$` to the shell.
#
# `git ls-tree -r --name-only` reads paths recursively from that commit without
# changing branches; `-- contracts/grpc` limits the lookup. `grep -q '\.proto$$'`
# checks silently for a protobuf file. The leading `@` hides Make's full recipe
# command and leaves the useful results and errors visible.
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

proto-check: proto-lint proto-breaking proto-generated-check
proto: proto-generate
