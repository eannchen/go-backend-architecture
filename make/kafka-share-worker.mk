KAFKA_SHARE_WORKER_PROFILE := true
AIR_KAFKASHAREWORKER_CONFIG ?= $(if $(wildcard config/air/kafkashareworker.toml),config/air/kafkashareworker.toml,.air.toml)

.PHONY: run-kafkashareworker
run-kafkashareworker:
	go run ./cmd/kafkashareworker
