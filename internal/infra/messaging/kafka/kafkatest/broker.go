//go:build integration

// Package kafkatest owns disposable infrastructure shared by Kafka integration suites.
package kafkatest

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// StartBroker starts one real broker with Share Groups enabled. The caller owns
// termination, including a partially started container returned with an error.
func StartBroker(ctx context.Context) (testcontainers.Container, []string, error) {
	var brokers []string
	// Docker chooses a free host port. Kafka must advertise that mapped address,
	// so copy a startup script after the container starts but before Kafka starts.
	container, err := testcontainers.Run(ctx, "apache/kafka:4.2.1",
		testcontainers.WithExposedPorts("9092/tcp"),
		testcontainers.WithEnv(map[string]string{
			"KAFKA_NODE_ID": "1", "KAFKA_PROCESS_ROLES": "broker,controller",
			"CLUSTER_ID":                             "4L6g3nShT-eMCtK--X86sw",
			"KAFKA_CONTROLLER_QUORUM_VOTERS":         "1@localhost:29093",
			"KAFKA_CONTROLLER_LISTENER_NAMES":        "CONTROLLER",
			"KAFKA_LISTENERS":                        "HOST://:9092,INTERNAL://:19092,CONTROLLER://:29093",
			"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP":   "HOST:PLAINTEXT,INTERNAL:PLAINTEXT,CONTROLLER:PLAINTEXT",
			"KAFKA_INTER_BROKER_LISTENER_NAME":       "INTERNAL",
			"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR": "1", "KAFKA_OFFSETS_TOPIC_NUM_PARTITIONS": "1",
			"KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR": "1", "KAFKA_TRANSACTION_STATE_LOG_MIN_ISR": "1",
			// Single-node internal state for per-record Share Group acknowledgments.
			"KAFKA_SHARE_COORDINATOR_STATE_TOPIC_REPLICATION_FACTOR": "1",
			"KAFKA_SHARE_COORDINATOR_STATE_TOPIC_MIN_ISR":            "1",
			"KAFKA_SHARE_COORDINATOR_STATE_TOPIC_NUM_PARTITIONS":     "1",
			// Allow short leases for the expiry test. Other share tests explicitly
			// request the normal 30s lock, leaving time for their assertions.
			"KAFKA_GROUP_SHARE_MIN_RECORD_LOCK_DURATION_MS": "1000",
			"KAFKA_AUTO_CREATE_TOPICS_ENABLE":               "false", "KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS": "0",
		}),
		testcontainers.WithEntrypoint("/bin/bash"),
		// This readiness loop waits for a real file copied by the lifecycle hook;
		// the container startup context bounds it. It is not a fixed startup delay.
		testcontainers.WithCmd("-c", "while [ ! -f /tmp/start-kafka.sh ]; do sleep 0.1; done; exec /bin/bash /tmp/start-kafka.sh"),
		testcontainers.WithLifecycleHooks(testcontainers.ContainerLifecycleHooks{PostStarts: []testcontainers.ContainerHook{
			func(ctx context.Context, c testcontainers.Container) error {
				host, err := c.Host(ctx)
				if err != nil {
					return err
				}
				port, err := c.MappedPort(ctx, "9092/tcp")
				if err != nil {
					return err
				}
				address := net.JoinHostPort(host, port.Port())
				script := fmt.Sprintf("#!/bin/bash\nexport KAFKA_ADVERTISED_LISTENERS='HOST://%s,INTERNAL://localhost:19092'\nexec /etc/kafka/docker/run\n", address)
				if err := c.CopyToContainer(ctx, []byte(script), "/tmp/start-kafka.sh", 0o755); err != nil {
					return err
				}
				brokers = []string{address}
				return wait.ForListeningPort("9092/tcp").WithStartupTimeout(90*time.Second).WaitUntilReady(ctx, c)
			},
		}}),
	)
	return container, brokers, err
}
