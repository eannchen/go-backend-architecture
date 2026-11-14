package kafka

import (
	"crypto/tls"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
)

// securityOptions translates settings already validated by each client constructor
// into broker connection options.
// The writer, consumer group reader, and Share Group reader all use these options.
// Here, "client" means any application connecting to Kafka, including a producer.
// App wiring loads TLS material through the shared tlsconfig package and injects
// it into each constructor. The broker authenticates the connection before allowing
// the client to publish or consume records.
func securityOptions(cfg config.KafkaSecurityConfig, tlsConfig *tls.Config) ([]kgo.Opt, error) {
	if cfg.TLS.Enabled != (tlsConfig != nil) {
		return nil, fmt.Errorf("Kafka TLS settings and injected TLS configuration must agree")
	}
	var opts []kgo.Opt
	if tlsConfig != nil {
		// Add an option telling franz-go to use TLS when connecting to brokers.
		// The config supplies trusted CAs and any client certificate for mTLS.
		// Clone gives this client its own config struct; certificate and CA data
		// can still be shared and should be treated as read-only.
		// For each connection, franz-go makes another copy. If ServerName is empty,
		// it uses that broker's hostname to check the broker's certificate.
		opts = append(opts, kgo.DialTLSConfig(tlsConfig.Clone()))
	}
	// SASL lets the broker verify who is connecting: a producer publishing records
	// or a consumer reading them. Each application supplies its own credentials.
	// SASL does not encrypt traffic; use TLS for credentials sent over an untrusted
	// network, particularly with PLAIN.
	switch cfg.SASL.Mechanism {
	case config.KafkaSASLPlain:
		opts = append(opts, kgo.SASL(plain.Auth{User: cfg.SASL.Username, Pass: cfg.SASL.Password}.AsMechanism()))
	case config.KafkaSASLScramSHA256:
		opts = append(opts, kgo.SASL(scram.Auth{User: cfg.SASL.Username, Pass: cfg.SASL.Password}.AsSha256Mechanism()))
	case config.KafkaSASLScramSHA512:
		opts = append(opts, kgo.SASL(scram.Auth{User: cfg.SASL.Username, Pass: cfg.SASL.Password}.AsSha512Mechanism()))
	}
	return opts, nil
}
