package observability

const (
	// keyMessagingSystem messaging system name, set to kafka.
	keyMessagingSystem = "messaging.system"
	// keyMessagingDestinationName topic name. Keep record coordinates in logs and spans.
	keyMessagingDestinationName = "messaging.destination.name"
	// keyMessagingPartitionID partition containing the record.
	keyMessagingPartitionID = "messaging.destination.partition.id"
	// keyKafkaOffset record position within its partition.
	keyKafkaOffset = "messaging.kafka.offset"
	// keyApplicationKafkaOperation fixed operation name selected by app wiring, also used in metrics.
	keyApplicationKafkaOperation = "app.kafka.operation"
	// keyApplicationKafkaOutcome processing outcome, such as success or timeout, also used in metrics.
	keyApplicationKafkaOutcome = "app.kafka.outcome"
	// keyLogDurationMS processing duration in milliseconds for completion logs.
	keyLogDurationMS = "duration.ms"
	// keyErrorType failure classification derived from the processing outcome.
	keyErrorType = "error.type"
	// instrumentationScope instrumentation scope for Kafka processing spans.
	instrumentationScope = "delivery.kafka"
)
