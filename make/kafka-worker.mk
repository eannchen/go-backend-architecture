KAFKA_WORKER_PROFILE := true
AIR_KAFKAWORKER_CONFIG ?= $(if $(wildcard .air.kafkaworker.toml),.air.kafkaworker.toml,.air.toml)

.PHONY: run-kafkaworker
run-kafkaworker:
	go run ./cmd/kafkaworker
