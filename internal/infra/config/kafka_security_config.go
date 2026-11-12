package config

import (
	"fmt"
	"strconv"
	"strings"
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
