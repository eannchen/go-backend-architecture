//go:build integration

package kafkatest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/infra/config"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
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
				// A mapped TCP port can accept connections while Kafka is still
				// starting. Wait until Kafka enables request processing as well.
				return wait.ForAll(
					wait.ForLog("Kafka Server started"),
					wait.ForListeningPort("9092/tcp"),
				).WithDeadline(90*time.Second).WaitUntilReady(ctx, c)
			},
		}}),
	)
	return container, brokers, err
}

// Broker owns the disposable backend and admin client for one test package.
type Broker struct {
	Container testcontainers.Container
	Brokers   []string
	Admin     *kgo.Client
	sequence  atomic.Uint64
}

// RunSuite starts one broker per package. Tests own unique topics and groups.
// Backend unavailability fails the suite, and cleanup also runs after failures.
func RunSuite(m *testing.M, configure func(*Broker)) int {
	setup, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	container, brokers, err := StartBroker(setup)
	if err != nil {
		cancel()
		if container != nil {
			err = errors.Join(err, testcontainers.TerminateContainer(container))
		}
		fmt.Fprintf(os.Stderr, "start Kafka test dependency: %v\n", err)
		return 1
	}
	broker := &Broker{Container: container, Brokers: brokers}
	broker.Admin, err = kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err == nil {
		// Verify Kafka protocol access through the advertised host address before
		// tests make administrative requests. Keep the setup deadline active.
		err = broker.Admin.Ping(setup)
	}
	cancel()
	code := 1
	if err == nil {
		configure(broker)
		code = m.Run()
	} else {
		fmt.Fprintln(os.Stderr, err)
	}
	cleanup, stop := context.WithTimeout(context.Background(), time.Minute)
	defer stop()
	if code != 0 {
		logs, err := container.Logs(cleanup)
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
	if broker.Admin != nil {
		broker.Admin.Close()
	}
	if err := testcontainers.TerminateContainer(container); err != nil {
		fmt.Fprintln(os.Stderr, err)
		code = 1
	}
	return code
}
func (b *Broker) CreateTopic(t *testing.T, partitions int32) string {
	t.Helper()
	topic := fmt.Sprintf("template-test-%d", b.sequence.Add(1))
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	req := kmsg.NewPtrCreateTopicsRequest()
	req.TimeoutMillis = 10000
	item := kmsg.NewCreateTopicsRequestTopic()
	item.Topic = topic
	item.NumPartitions = partitions
	item.ReplicationFactor = 1
	req.Topics = []kmsg.CreateTopicsRequestTopic{item}
	response, err := b.Admin.Request(ctx, req)
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
		response, err := b.Admin.Request(ctx, req)
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
	// Topic creation can finish before its partition leaders are ready for I/O.
	// Wait for metadata rather than adding a fixed delay before every test.
	if err := b.waitForTopic(ctx, topic, partitions); err != nil {
		t.Fatal(err)
	}
	return topic
}

func (b *Broker) waitForTopic(ctx context.Context, topic string, partitions int32) error {
	req := kmsg.NewPtrMetadataRequest()
	req.Topics = []kmsg.MetadataRequestTopic{{Topic: kmsg.StringPtr(topic)}}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		response, err := req.RequestWith(ctx, b.Admin)
		if err != nil {
			return fmt.Errorf("wait for Kafka topic %q: %w", topic, err)
		}
		ready := false
		for _, result := range response.Topics {
			if err := kerr.ErrorForCode(result.ErrorCode); err != nil && !kerr.IsRetriable(err) {
				return fmt.Errorf("Kafka topic %q metadata: %w", topic, err)
			}
			ready = result.ErrorCode == 0 && len(result.Partitions) == int(partitions)
			for _, partition := range result.Partitions {
				if err := kerr.ErrorForCode(partition.ErrorCode); err != nil && !kerr.IsRetriable(err) {
					return fmt.Errorf("Kafka topic %q partition %d metadata: %w", topic, partition.Partition, err)
				}
				ready = ready && partition.ErrorCode == 0 && partition.Leader >= 0 && len(partition.ISR) > 0
			}
		}
		if ready {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for Kafka topic %q partition leaders: %w", topic, ctx.Err())
		case <-ticker.C:
		}
	}
}

// PublisherConfig gives real-broker tests time for metadata discovery, producer
// initialization, and retries on a busy CI runner. Unit-test deadlines are too short
// for this setup. Small batches keep tests quick without weakening acknowledgments.
func (b *Broker) PublisherConfig(topic string) config.KafkaPublisherConfig {
	return config.KafkaPublisherConfig{
		Brokers: b.Brokers, Topic: topic, PartitionStrategy: config.PartitionRoundRobin, RequiredAcks: config.KafkaAcksAll,
		RequestTimeout: 10 * time.Second, ProduceTimeout: 10 * time.Second, DeliveryTimeout: 30 * time.Second,
		MaxBufferedRecords: 25, BatchBytes: 65536, BatchTimeout: time.Millisecond, MaxAttempts: 10,
		WriteBackoffMin: 100 * time.Millisecond, WriteBackoffMax: time.Second, Compression: config.CompressionNone,
	}
}

func (b *Broker) CleanupGroup(t *testing.T, group string) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		req := kmsg.NewPtrDeleteGroupsRequest()
		req.Groups = []string{group}
		response, err := b.Admin.Request(ctx, req)
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

// PrepareShareGroup gives a fresh group explicit broker-owned policy.
func (b *Broker) PrepareShareGroup(t *testing.T, group string, lockMillis int) {
	t.Helper()
	b.CleanupGroup(t, group)
	b.CleanupShareGroupConfig(t, group)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	if err := b.alterShareGroup(ctx, group, lockMillis, false); err != nil {
		t.Fatal(err)
	}
}

// CleanupShareGroupConfig also supports tests that provision policy through the
// CLI script rather than through PrepareShareGroup.
func (b *Broker) CleanupShareGroupConfig(t *testing.T, group string) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := b.alterShareGroup(ctx, group, 0, true); err != nil {
			t.Error(err)
		}
	})
}

func (b *Broker) alterShareGroup(ctx context.Context, group string, lockMillis int, remove bool) error {
	req := kmsg.NewPtrIncrementalAlterConfigsRequest()
	resource := kmsg.NewIncrementalAlterConfigsRequestResource()
	resource.ResourceType = kmsg.ConfigResourceTypeGroupConfig
	resource.ResourceName = group
	for key, value := range map[string]string{"share.auto.offset.reset": "earliest", "share.record.lock.duration.ms": fmt.Sprint(lockMillis)} {
		entry := kmsg.NewIncrementalAlterConfigsRequestResourceConfig()
		entry.Name = key
		entry.Value = kmsg.StringPtr(value)
		if remove {
			entry.Op = kmsg.IncrementalAlterConfigOpDelete
			entry.Value = nil
		}
		resource.Configs = append(resource.Configs, entry)
	}
	req.Resources = append(req.Resources, resource)
	response, err := req.RequestWith(ctx, b.Admin)
	if err != nil {
		return err
	}
	for _, result := range response.Resources {
		if err := kerr.ErrorForCode(result.ErrorCode); err != nil {
			return err
		}
	}
	return nil
}
