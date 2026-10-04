package config

// KafkaWorkerConfig combines shared runtime settings with the consumer profile.
type KafkaWorkerConfig struct {
	RuntimeConfig
	Kafka KafkaConsumerConfig
}

func LoadKafkaWorker() (KafkaWorkerConfig, error) {
	runtimeConfig, err := LoadRuntime()
	if err != nil {
		return KafkaWorkerConfig{}, err
	}
	consumerConfig, err := LoadKafkaConsumer()
	if err != nil {
		return KafkaWorkerConfig{}, err
	}
	return KafkaWorkerConfig{RuntimeConfig: runtimeConfig, Kafka: consumerConfig}, nil
}
