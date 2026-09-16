.DEFAULT_GOAL := run

include make/common.mk
-include make/public-http.mk
-include make/service-grpc.mk

ifeq ($(PUBLIC_HTTP_PROFILE),true)
run: run-httpapi
else ifeq ($(SERVICE_GRPC_PROFILE),true)
run: run-grpcapi
else
run:
	@echo "No application profile is installed."
	@exit 1
endif

ci: $(CONTRACT_CHECK_TARGETS) check test-race test-integration

install:
	@set -e; for tool in $(INSTALL_TOOLS); do go install "$$tool"; done

.PHONY: run ci install
