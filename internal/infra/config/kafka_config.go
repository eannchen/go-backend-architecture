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

// KafkaWorkerFailurePolicy chooses how one worker's exit affects its siblings.
type KafkaWorkerFailurePolicy string

const (
	KafkaWorkersStopAll     KafkaWorkerFailurePolicy = "stop-all"
	KafkaWorkersIndependent KafkaWorkerFailurePolicy = "independent"
)

// KafkaWorkerRoutesConfig binds example workflows to configurable topic names.
// Each nonempty binding becomes a subscription. Empty disables that workflow.
type KafkaWorkerRoutesConfig struct {
	NotificationTopic string
	ActivityTopic     string
}

// kafkaWorkflow is the single list entry used by both consumer styles.
// Add a workflow's binding and default groups here when extending the template.
type kafkaWorkflow struct {
	prefix            string
	topic             string
	defaultGroup      string
	defaultShareGroup string
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

func (cfg KafkaWorkerRoutesConfig) workflows() []kafkaWorkflow {
	return []kafkaWorkflow{
		{
			prefix:            "KAFKA_NOTIFICATION_",
			topic:             cfg.NotificationTopic,
			defaultGroup:      "notification-sender-v1",
			defaultShareGroup: "shared-workers-v1",
		},
		{
			prefix:            "KAFKA_ACTIVITY_",
			topic:             cfg.ActivityTopic,
			defaultGroup:      "activity-workers-v1",
			defaultShareGroup: "activity-share-workers-v1",
		},
	}
}

// Topics derives subscriptions from enabled workflows.
func (cfg KafkaWorkerRoutesConfig) Topics() []string {
	workflows := cfg.workflows()
	topics := make([]string, 0, len(workflows))
	for _, workflow := range workflows {
		if workflow.topic != "" {
			topics = append(topics, workflow.topic)
		}
	}
	return topics
}

func loadKafkaWorkerFailurePolicy() (KafkaWorkerFailurePolicy, error) {
	policy := KafkaWorkerFailurePolicy(strings.TrimSpace(getEnv("KAFKA_WORKER_FAILURE_POLICY", string(KafkaWorkersStopAll))))
	switch policy {
	case KafkaWorkersStopAll, KafkaWorkersIndependent:
		return policy, nil
	default:
		return "", fmt.Errorf("KAFKA_WORKER_FAILURE_POLICY must be stop-all or independent")
	}
}

// loadKafkaSecurity reads one connection's settings from its environment prefix.
func loadKafkaSecurity(prefix string) (KafkaSecurityConfig, error) {
	enabled, err := strconv.ParseBool(strings.TrimSpace(getEnv(prefix+"TLS_ENABLED", "false")))
	if err != nil {
		return KafkaSecurityConfig{}, fmt.Errorf("%sTLS_ENABLED must be a boolean", prefix)
	}
	cfg := KafkaSecurityConfig{
		TLS: KafkaTLSConfig{
			Enabled:        enabled,
			ServerCAFile:   strings.TrimSpace(getEnv(prefix+"TLS_SERVER_CA_FILE", "")),
			ClientCertFile: strings.TrimSpace(getEnv(prefix+"TLS_CLIENT_CERT_FILE", "")),
			ClientKeyFile:  strings.TrimSpace(getEnv(prefix+"TLS_CLIENT_KEY_FILE", "")),
			ServerName:     strings.TrimSpace(getEnv(prefix+"TLS_SERVER_NAME", "")),
		},
		SASL: KafkaSASLConfig{
			Mechanism: KafkaSASLMechanism(strings.ToLower(strings.TrimSpace(getEnv(prefix+"SASL_MECHANISM", "none")))),
			// Credentials are opaque: whitespace may be part of a username or password.
			Username: getEnv(prefix+"SASL_USERNAME", ""),
			Password: getEnv(prefix+"SASL_PASSWORD", ""),
		},
	}
	return cfg, cfg.validate(prefix)
}

// A typo such as "ten seconds" must fail startup instead of using a default.
// Durations use Go notation, e.g. "500ms", "10s", or "1m".
func kafkaDuration(key, fallback string) (time.Duration, error) {
	value, err := time.ParseDuration(strings.TrimSpace(getEnv(key, fallback)))
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", key, err)
	}
	return value, nil
}

// Worker deadlines must be positive. Client-specific duration bounds are checked
// by their own config validation after parsing.
func kafkaPositiveDuration(key, fallback string) (time.Duration, error) {
	value, err := kafkaDuration(key, fallback)
	if err != nil {
		return 0, err
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", key)
	}
	return value, nil
}

func (cfg KafkaSecurityConfig) Validate() error {
	return cfg.validate("KAFKA_")
}

func (cfg KafkaSecurityConfig) validate(prefix string) error {
	if !cfg.TLS.Enabled && (cfg.TLS.ServerCAFile != "" || cfg.TLS.ClientCertFile != "" || cfg.TLS.ClientKeyFile != "" || cfg.TLS.ServerName != "") {
		return fmt.Errorf("%sTLS_ENABLED must be true when TLS files or server name are configured", prefix)
	}
	if (cfg.TLS.ClientCertFile == "") != (cfg.TLS.ClientKeyFile == "") {
		return fmt.Errorf("%sTLS_CLIENT_CERT_FILE and %sTLS_CLIENT_KEY_FILE must be configured together", prefix, prefix)
	}
	switch cfg.SASL.Mechanism {
	case "", KafkaSASLNone:
		if cfg.SASL.Username != "" || cfg.SASL.Password != "" {
			return fmt.Errorf("%sSASL_MECHANISM must be enabled when credentials are configured", prefix)
		}
	case KafkaSASLPlain, KafkaSASLScramSHA256, KafkaSASLScramSHA512:
		if cfg.SASL.Username == "" || cfg.SASL.Password == "" {
			return fmt.Errorf("%sSASL_USERNAME and %sSASL_PASSWORD are required for SASL", prefix, prefix)
		}
	default:
		return fmt.Errorf("%sSASL_MECHANISM must be none, plain, scram-sha-256, or scram-sha-512", prefix)
	}
	return nil
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

// Bound network overhead before SDK construction so startup errors identify the
// setting. This is separate from broker-side waits and overall caller deadlines.
func validateKafkaRequestTimeout(timeout time.Duration) error {
	if timeout < 100*time.Millisecond || timeout > 15*time.Minute {
		return fmt.Errorf("KAFKA_REQUEST_TIMEOUT must be between 100ms and 15m")
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
