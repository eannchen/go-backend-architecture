package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// KafkaSecurityConfig is shared by publishers and both consumer styles.
// TLS encrypts broker traffic; SASL authenticates this client to the broker.
type KafkaSecurityConfig struct {
	TLS  KafkaTLSConfig
	SASL KafkaSASLConfig
}

type KafkaTLSConfig struct {
	Enabled bool
	// Empty uses system trust roots. A file selects the supplied private CA roots.
	ServerCAFile string
	// Optional client identity for brokers requiring mutual TLS. Set both files.
	ClientCertFile string
	ClientKeyFile  string
	// Empty verifies each broker's hostname. Override only when its certificate
	// deliberately uses a different identity shared by the contacted brokers.
	ServerName string
}

type KafkaSASLMechanism string

const (
	KafkaSASLNone        KafkaSASLMechanism = "none"
	KafkaSASLPlain       KafkaSASLMechanism = "plain"
	KafkaSASLScramSHA256 KafkaSASLMechanism = "scram-sha-256"
	KafkaSASLScramSHA512 KafkaSASLMechanism = "scram-sha-512"
)

type KafkaSASLConfig struct {
	Mechanism KafkaSASLMechanism
	Username  string
	Password  string
}

func LoadKafkaSecurity() (KafkaSecurityConfig, error) {
	enabled, err := strconv.ParseBool(strings.TrimSpace(getEnv("KAFKA_TLS_ENABLED", "false")))
	if err != nil {
		return KafkaSecurityConfig{}, fmt.Errorf("KAFKA_TLS_ENABLED must be a boolean")
	}
	cfg := KafkaSecurityConfig{
		TLS: KafkaTLSConfig{
			Enabled:        enabled,
			ServerCAFile:   strings.TrimSpace(getEnv("KAFKA_TLS_SERVER_CA_FILE", "")),
			ClientCertFile: strings.TrimSpace(getEnv("KAFKA_TLS_CLIENT_CERT_FILE", "")),
			ClientKeyFile:  strings.TrimSpace(getEnv("KAFKA_TLS_CLIENT_KEY_FILE", "")),
			ServerName:     strings.TrimSpace(getEnv("KAFKA_TLS_SERVER_NAME", "")),
		},
		SASL: KafkaSASLConfig{
			Mechanism: KafkaSASLMechanism(strings.ToLower(strings.TrimSpace(getEnv("KAFKA_SASL_MECHANISM", "none")))),
			// Credentials are opaque: whitespace may be part of a username or password.
			Username: getEnv("KAFKA_SASL_USERNAME", ""),
			Password: getEnv("KAFKA_SASL_PASSWORD", ""),
		},
	}
	return cfg, cfg.Validate()
}

func (cfg KafkaSecurityConfig) Validate() error {
	if !cfg.TLS.Enabled && (cfg.TLS.ServerCAFile != "" || cfg.TLS.ClientCertFile != "" || cfg.TLS.ClientKeyFile != "" || cfg.TLS.ServerName != "") {
		return fmt.Errorf("KAFKA_TLS_ENABLED must be true when TLS files or server name are configured")
	}
	if (cfg.TLS.ClientCertFile == "") != (cfg.TLS.ClientKeyFile == "") {
		return fmt.Errorf("KAFKA_TLS_CLIENT_CERT_FILE and KAFKA_TLS_CLIENT_KEY_FILE must be configured together")
	}
	switch cfg.SASL.Mechanism {
	case "", KafkaSASLNone:
		if cfg.SASL.Username != "" || cfg.SASL.Password != "" {
			return fmt.Errorf("KAFKA_SASL_MECHANISM must be enabled when credentials are configured")
		}
	case KafkaSASLPlain, KafkaSASLScramSHA256, KafkaSASLScramSHA512:
		if cfg.SASL.Username == "" || cfg.SASL.Password == "" {
			return fmt.Errorf("KAFKA_SASL_USERNAME and KAFKA_SASL_PASSWORD are required for SASL")
		}
	default:
		return fmt.Errorf("KAFKA_SASL_MECHANISM must be none, plain, scram-sha-256, or scram-sha-512")
	}
	return nil
}

// KafkaWorkerRoutesConfig binds example workflows to configurable topic names.
// Each nonempty binding becomes a subscription. Empty disables that workflow.
type KafkaWorkerRoutesConfig struct {
	NotificationTopic string
	ActivityTopic     string
}

func loadKafkaWorkerRoutes() (KafkaWorkerRoutesConfig, error) {
	cfg := KafkaWorkerRoutesConfig{
		NotificationTopic: strings.TrimSpace(getEnv("KAFKA_NOTIFICATION_TOPIC", "notifications.requested.v1")),
		ActivityTopic:     strings.TrimSpace(getEnv("KAFKA_ACTIVITY_TOPIC", "activities.recorded.v1")),
	}
	topics := cfg.Topics()
	if err := validateKafkaTopics(topics); err != nil {
		return cfg, fmt.Errorf("Kafka worker topic bindings (KAFKA_NOTIFICATION_TOPIC and KAFKA_ACTIVITY_TOPIC): %w", err)
	}

	return cfg, nil
}

// Topics derives subscriptions from enabled workflows.
func (cfg KafkaWorkerRoutesConfig) Topics() []string {
	topics := make([]string, 0, 2)
	for _, topic := range []string{cfg.NotificationTopic, cfg.ActivityTopic} {
		if topic != "" {
			topics = append(topics, topic)
		}
	}
	return topics
}

func validateKafkaSubscription(brokers, topics []string) error {
	if err := validateKafkaTopics(topics); err != nil {
		return err
	}
	return validateKafkaConnection(brokers, topics[0])
}

func validateKafkaTopics(topics []string) error {
	if len(topics) == 0 {
		return fmt.Errorf("Kafka subscription must contain at least one topic")
	}
	seen := make(map[string]bool, len(topics))
	for _, topic := range topics {
		if topic == "" || strings.TrimSpace(topic) != topic || len(topic) > 249 || topic == "." || topic == ".." {
			return fmt.Errorf("invalid Kafka topic %q", topic)
		}
		for _, c := range topic {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-') {
				return fmt.Errorf("invalid Kafka topic %q", topic)
			}
		}
		if seen[topic] {
			return fmt.Errorf("duplicate Kafka topic %q", topic)
		}
		seen[topic] = true
	}
	return nil
}

// Both publishers and consumers need cluster discovery addresses and a topic.
func validateKafkaConnection(brokers []string, topic string) error {
	if len(brokers) == 0 {
		return fmt.Errorf("KAFKA_BROKERS must contain at least one broker")
	}
	for i, broker := range brokers {
		if strings.TrimSpace(broker) == "" {
			return fmt.Errorf("KAFKA_BROKERS entry at index %d must not be empty", i)
		}
	}
	if strings.TrimSpace(topic) == "" {
		return fmt.Errorf("KAFKA_PUBLISH_TOPIC must not be empty")
	}
	return nil
}

// Fail visibly on removed settings instead of accepting tuning that has no effect.
func rejectKafkaSetting(key, explanation string) error {
	if _, exists := os.LookupEnv(key); exists {
		return fmt.Errorf("%s is no longer supported: %s", key, explanation)
	}
	return nil
}

// Bound network overhead before SDK construction so startup errors identify the
// setting. This is separate from broker-side waits and overall caller deadlines.
func validateKafkaRequestTimeout(timeout time.Duration) error {
	if timeout < 100*time.Millisecond || timeout > 15*time.Minute {
		return fmt.Errorf("KAFKA_REQUEST_TIMEOUT must be between 100ms and 15m")
	}
	return nil
}
