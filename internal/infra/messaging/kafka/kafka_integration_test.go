//go:build integration

package kafka

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eannchen/go-backend-architecture/internal/infra/messaging/kafka/kafkatest"
	"github.com/testcontainers/testcontainers-go"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

var integrationBrokers []string
var integrationAdmin *kgo.Client
var integrationContainer testcontainers.Container
var integrationTopicSequence atomic.Uint64

// TestMain owns one disposable broker for this package, separate from Compose
// data. Each test creates/deletes its own topic and group; unavailable Docker fails.
func TestMain(m *testing.M) { os.Exit(runKafkaIntegrationTests(m)) }

func runKafkaIntegrationTests(m *testing.M) int {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	container, brokers, err := kafkatest.StartBroker(ctx)
	integrationBrokers = brokers
	if err != nil {
		fmt.Fprintf(os.Stderr, "start Kafka integration dependency: %v\n", err)
		if container != nil {
			if err := testcontainers.TerminateContainer(container); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
		return 1
	}
	integrationContainer = container
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
