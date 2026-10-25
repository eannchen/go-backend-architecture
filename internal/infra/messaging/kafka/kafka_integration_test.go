//go:build integration

package kafka

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

var integrationBrokers []string
var integrationAdmin *kgo.Client
var integrationTopicSequence atomic.Uint64

// TestMain owns one disposable broker for this package, separate from Compose
// data. Each test creates/deletes its own topic and group; unavailable Docker fails.
func TestMain(m *testing.M) { os.Exit(runKafkaIntegrationTests(m)) }

func runKafkaIntegrationTests(m *testing.M) int {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	// Docker chooses a free host port. Kafka must advertise that mapped address,
	// so copy a startup script after the container starts but before Kafka starts.
	container, err := testcontainers.Run(ctx, "apache/kafka:4.1.2",
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
			"KAFKA_AUTO_CREATE_TOPICS_ENABLE": "false", "KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS": "0",
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
				integrationBrokers = []string{address}
				return wait.ForListeningPort("9092/tcp").WithStartupTimeout(90*time.Second).WaitUntilReady(ctx, c)
			},
		}}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start Kafka integration dependency: %v\n", err)
		if container != nil {
			if err := testcontainers.TerminateContainer(container); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
		return 1
	}
	integrationAdmin, err = kgo.NewClient(kgo.SeedBrokers(integrationBrokers...))
	if err == nil {
		err = integrationAdmin.Ping(ctx)
	}
	code := 1
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect Kafka integration admin: %v\n", err)
	} else {
		code = m.Run()
	}
	if integrationAdmin != nil {
		integrationAdmin.Close()
	}
	if code != 0 {
		logs, err := container.Logs(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			if _, err := io.Copy(os.Stderr, logs); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			if err := logs.Close(); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}
	if err := testcontainers.TerminateContainer(container); err != nil {
		fmt.Fprintf(os.Stderr, "terminate Kafka integration dependency: %v\n", err)
		code = 1
	}
	return code
}

func createIntegrationTopic(t *testing.T, partitions int32) string {
	t.Helper()
	topic := fmt.Sprintf("template-test-%d", integrationTopicSequence.Add(1))
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	req := kmsg.NewPtrCreateTopicsRequest()
	req.TimeoutMillis = 10000
	item := kmsg.NewCreateTopicsRequestTopic()
	item.Topic = topic
	item.NumPartitions = partitions
	item.ReplicationFactor = 1
	req.Topics = []kmsg.CreateTopicsRequestTopic{item}
	response, err := integrationAdmin.Request(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range response.(*kmsg.CreateTopicsResponse).Topics {
		if err := kerr.ErrorForCode(result.ErrorCode); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		req := kmsg.NewPtrDeleteTopicsRequest()
		req.TimeoutMillis = 10000
		item := kmsg.NewDeleteTopicsRequestTopic()
		item.Topic = kmsg.StringPtr(topic)
		req.Topics = []kmsg.DeleteTopicsRequestTopic{item}
		response, err := integrationAdmin.Request(ctx, req)
		if err != nil {
			t.Error(err)
			return
		}
		for _, result := range response.(*kmsg.DeleteTopicsResponse).Topics {
			if err := kerr.ErrorForCode(result.ErrorCode); err != nil {
				t.Error(err)
			}
		}
	})
	return topic
}

func cleanupIntegrationGroup(t *testing.T, group string) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		req := kmsg.NewPtrDeleteGroupsRequest()
		req.Groups = []string{group}
		response, err := integrationAdmin.Request(ctx, req)
		if err != nil {
			t.Error(err)
			return
		}
		for _, result := range response.(*kmsg.DeleteGroupsResponse).Groups {
			if err := kerr.ErrorForCode(result.ErrorCode); err != nil && err != kerr.GroupIDNotFound {
				t.Error(err)
			}
		}
	})
}
