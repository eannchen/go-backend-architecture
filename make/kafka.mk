# A separate Compose file/project keeps Kafka independent of the other local services.
KAFKA_COMPOSE := docker compose -f docker-compose.kafka.yml

.PHONY: dev-kafka-up dev-kafka-down dev-kafka-logs kafka-topic-init

dev-kafka-up:
	# Wait for the broker protocol healthcheck, then provision and verify the topic.
	$(KAFKA_COMPOSE) up -d --wait --wait-timeout 120 kafka
	$(MAKE) kafka-topic-init

kafka-topic-init:
	# Run again after dev-kafka-up to verify existing settings without altering them.
	$(KAFKA_COMPOSE) run --rm --no-deps kafka-init

dev-kafka-down:
	# Preserve the volume: stopping local development must not erase events or offsets.
	$(KAFKA_COMPOSE) down

dev-kafka-logs:
	$(KAFKA_COMPOSE) logs -f kafka

.PHONY: run-kafkaworker

run-kafkaworker:
	go run ./cmd/kafkaworker
