KAFKA_WORKER_PROFILE := true
AIR_KAFKAWORKER_CONFIG ?= $(if $(wildcard config/air/kafkaworker.toml),config/air/kafkaworker.toml,.air.toml)

.PHONY: run-kafkaworker
run-kafkaworker:
	go run ./cmd/kafkaworker
