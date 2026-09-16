PUBLIC_HTTP_PROFILE := true
SQLC_CONFIG ?= $(if $(wildcard sqlc.public-http.yaml),sqlc.public-http.yaml,sqlc.yaml)
AIR_HTTPAPI_CONFIG ?= $(if $(wildcard .air.httpapi.toml),.air.httpapi.toml,.air.toml)
AIR_HTTPAPI_CMD ?= air -c $(AIR_HTTPAPI_CONFIG)
OAPI_CODEGEN_CMD ?= github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0
VACUUM_CMD ?= github.com/daveshanley/vacuum@v0.30.1
OASDIFF_CMD ?= github.com/oasdiff/oasdiff@v1.29.1
OPENAPI_CONTRACT ?= contracts/http/openapi.yaml
OPENAPI_GENERATED_DIR ?= internal/delivery/http/openapi/gen
OPENAPI_BREAKING_BASE_REF ?= main
INTEGRATION_PACKAGES += ./internal/delivery/http/integration
CONTRACT_CHECK_TARGETS += openapi-check
INSTALL_TOOLS += $(OAPI_CODEGEN_CMD) $(VACUUM_CMD) $(OASDIFF_CMD)

.PHONY: run-httpapi run-httpapi-stop openapi-generate openapi-lint openapi-breaking openapi-generated-check openapi-check openapi

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

openapi-generate:
	go run $(OAPI_CODEGEN_CMD) -config oapi-codegen.yaml $(OPENAPI_CONTRACT)

# Vacuum normally checks online for a newer release. This command-scoped setting
# disables only that check, keeping local and CI lint runs quiet and network-independent.
openapi-lint:
	VACUUM_NO_UPDATE_CHECK=true go run $(VACUUM_CMD) lint -d $(OPENAPI_CONTRACT)

# Compare the working-tree OpenAPI contract with a historical contract without
# checking out or modifying either version. OASDiff permits compatible additions
# but rejects removals, newly required inputs, narrowed inputs, and removed required
# responses because existing clients may rely on them.
#
# `git rev-parse ... '<ref>^{commit}'` validates the ref and resolves it to one
# immutable commit. `--quiet` suppresses Git's low-level missing-ref message so
# this target can print a clearer one. `[ -z "$$base_commit" ]` detects that Git
# returned no commit hash. Make consumes one `$`, so `$$` passes `$` to the shell.
#
# `git cat-file -e '<commit>:<path>'` checks whether that commit contains the
# contract. `mktemp` creates a temporary baseline file, and `trap` removes it when
# the command ends. `git show` copies the historical contract there without changing
# branches. OASDiff then compares baseline to working tree; `--fail-on ERR` makes
# detected breaking changes fail the target. The leading `@` hides Make's full
# recipe command and leaves the useful results and errors visible.
openapi-breaking:
	@base_commit="$$(git rev-parse --verify --quiet '$(OPENAPI_BREAKING_BASE_REF)^{commit}')"; \
	if [ -z "$$base_commit" ]; then \
		echo "OpenAPI breaking-check baseline '$(OPENAPI_BREAKING_BASE_REF)' is not available locally."; \
		exit 1; \
	fi; \
	if git cat-file -e "$$base_commit:$(OPENAPI_CONTRACT)" 2>/dev/null; then \
		base_contract="$$(mktemp)"; \
		trap 'rm -f "$$base_contract"' EXIT; \
		git show "$$base_commit:$(OPENAPI_CONTRACT)" > "$$base_contract"; \
		go run $(OASDIFF_CMD) breaking --fail-on ERR "$$base_contract" "$(OPENAPI_CONTRACT)"; \
	else \
		echo "Skipping OpenAPI breaking check: baseline '$(OPENAPI_BREAKING_BASE_REF)' predates the HTTP contract."; \
	fi

openapi-generated-check: openapi-generate
	@changes="$$(git status --short -- $(OPENAPI_GENERATED_DIR))"; \
	if [ -n "$$changes" ]; then \
		echo "Generated OpenAPI files are out of date. Run 'make openapi-generate' and commit the result:"; \
		echo "$$changes"; \
		exit 1; \
	fi

openapi-check: openapi-lint openapi-breaking openapi-generated-check
openapi: openapi-generate
