package config

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// KafkaInitialOffset chooses where to begin for a partition when this consumer
// group has no saved position: earliest reads the oldest retained messages;
// latest waits for new ones. It does not rewind committed progress.
type KafkaInitialOffset string

const (
	KafkaInitialEarliest KafkaInitialOffset = "earliest"
	KafkaInitialLatest   KafkaInitialOffset = "latest"
)

type KafkaConsumerConfig struct {
	// Addresses used to find the Kafka cluster, e.g. localhost:9092.
	Brokers []string
	Topic   string
	// Workers with the same GroupID share the topic's partitions and progress.
	// A different GroupID gets its own progress and can read the same messages.
	GroupID string
	// Ask Kafka to collect at least this many bytes before replying to a fetch
	// (a request for messages). With 1, it can reply when a message is available.
	// A larger value may make responses bigger, but can add waiting time.
	FetchMinBytes int
	// Ask Kafka to send up to this many bytes in a fetch response. The reader
	// also uses this as its per-partition fetch limit. This is not a memory cap:
	// Kafka may send a larger first batch so a large message can be read, and
	// decompressing data can use more memory.
	FetchMaxBytes int
	// Maximum time Kafka waits for FetchMinBytes before replying with what it
	// has. It can reply sooner. On an empty topic, FetchRecord keeps making
	// requests until a message arrives, an error occurs, or its context ends.
	FetchMaxWait time.Duration
	// Time allowed for consumers to finish current work, commit, and rejoin
	// AFTER a rebalance begins, e.g. when another worker joins the group.
	// It is not a per-message processing timeout or a timer started by fetching.
	// Example: with 1m, group members have roughly a minute from the start of
	// the rebalance, not a fresh minute for each record. Detection takes some
	// of that time, so finish handling and committing well before the limit.
	// Kafka uses the largest allowance configured by any member in the group.
	// A member that does not rejoin in time can be removed. Its Go handler is
	// not automatically canceled, nor is AllowRebalance automatically called.
	RebalanceTimeout time.Duration
	// Allow this much network time in addition to Kafka's fetch wait.
	// Example: FetchMaxWait=10s plus RequestTimeout=10s allows about 20s
	// for a fetch reply. It does not limit the total wait for the next message.
	RequestTimeout time.Duration
	// Maximum broker fetch requests in progress or still holding unread replies.
	// This counts requests, not partitions or records. One request can ask a
	// broker for several partitions and receive many records in one response.
	// A slot stays occupied until polling takes all records from that response
	// out of the client's buffer; it does not wait for handling or committing.
	// Example: a response contains 10 records, but PollRecords(ctx, 1) returns
	// only one. The other 9 stay buffered for later polls and still exist in Kafka.
	// Default 1 limits fetching ahead. This does not create parallel handlers
	// or limit how many records each poll can return; those are separate choices.
	MaxConcurrentFetches int
	// Choose the starting point for a partition only when this group has no
	// saved position for it. The default is earliest.
	// Changing this does not rewind an existing group. Use a new group or
	// deliberately reset its saved position if you want to read old messages again.
	InitialOffset KafkaInitialOffset
}

// LoadKafkaConsumer reads only settings needed to receive messages.
// A consumer does not need the publisher's batching or acknowledgment settings.
func LoadKafkaConsumer() (KafkaConsumerConfig, error) {
	for _, setting := range [][2]string{
		{"KAFKA_START_OFFSET", "use KAFKA_INITIAL_OFFSET; this chooses a starting point only without saved group progress and does not rewind commits"},
		{"KAFKA_READ_BATCH_TIMEOUT", "there is no batch-read timer; use KAFKA_REQUEST_TIMEOUT for network overhead and KAFKA_REBALANCE_TIMEOUT for partition reassignment"},
		{"KAFKA_QUEUE_CAPACITY", "there is no record-count queue; use KAFKA_MAX_CONCURRENT_FETCHES and KAFKA_FETCH_MAX_BYTES for fetch buffering"},
	} {
		if err := rejectKafkaSetting(setting[0], setting[1]); err != nil {
			return KafkaConsumerConfig{}, err
		}
	}
	cfg := KafkaConsumerConfig{
		Brokers:       getCSV("KAFKA_BROKERS", []string{"localhost:9092"}),
		Topic:         strings.TrimSpace(getEnv("KAFKA_TOPIC", "notifications.requested.v1")),
		GroupID:       strings.TrimSpace(getEnv("KAFKA_GROUP_ID", "notification-sender-v1")),
		InitialOffset: KafkaInitialOffset(strings.TrimSpace(getEnv("KAFKA_INITIAL_OFFSET", string(KafkaInitialEarliest)))),
	}
	// Start with small responses and one fetch at a time. MinBytes=1 lets Kafka
	// reply as soon as data is available; FetchMaxWait is not a fixed delay.
	for _, setting := range []struct {
		key, fallback string
		target        *int
	}{
		{"KAFKA_FETCH_MIN_BYTES", "1", &cfg.FetchMinBytes},
		{"KAFKA_FETCH_MAX_BYTES", "1000000", &cfg.FetchMaxBytes},
		{"KAFKA_MAX_CONCURRENT_FETCHES", "1", &cfg.MaxConcurrentFetches},
	} {
		value, err := strconv.Atoi(strings.TrimSpace(getEnv(setting.key, setting.fallback)))
		if err != nil {
			return KafkaConsumerConfig{}, fmt.Errorf("%s must be an integer: %w", setting.key, err)
		}
		*setting.target = value
	}
	for _, setting := range []struct {
		key, fallback string
		target        *time.Duration
	}{
		{"KAFKA_FETCH_MAX_WAIT", "10s", &cfg.FetchMaxWait},
		{"KAFKA_REBALANCE_TIMEOUT", "1m", &cfg.RebalanceTimeout},
		{"KAFKA_REQUEST_TIMEOUT", "10s", &cfg.RequestTimeout},
	} {
		value, err := time.ParseDuration(strings.TrimSpace(getEnv(setting.key, setting.fallback)))
		if err != nil {
			return KafkaConsumerConfig{}, fmt.Errorf("%s must be a duration: %w", setting.key, err)
		}
		*setting.target = value
	}
	if err := cfg.Validate(); err != nil {
		return KafkaConsumerConfig{}, err
	}
	return cfg, nil
}

// Validate rejects invalid settings before we construct the Kafka client.
// Constructors also call this when configuration was built directly in Go.
func (cfg KafkaConsumerConfig) Validate() error {
	if err := validateKafkaConnection(cfg.Brokers, cfg.Topic); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.GroupID) == "" {
		return fmt.Errorf("KAFKA_GROUP_ID must not be empty")
	}
	if cfg.FetchMinBytes <= 0 {
		return fmt.Errorf("KAFKA_FETCH_MIN_BYTES must be > 0")
	}
	if cfg.FetchMaxBytes <= 0 || cfg.FetchMaxBytes > 1<<30 {
		return fmt.Errorf("KAFKA_FETCH_MAX_BYTES must be between 1 and 1073741824")
	}
	if cfg.FetchMinBytes > cfg.FetchMaxBytes {
		return fmt.Errorf("KAFKA_FETCH_MIN_BYTES must be <= KAFKA_FETCH_MAX_BYTES")
	}
	if cfg.MaxConcurrentFetches <= 0 {
		return fmt.Errorf("KAFKA_MAX_CONCURRENT_FETCHES must be > 0")
	}
	if cfg.FetchMaxWait < 10*time.Millisecond || cfg.FetchMaxWait > time.Duration(math.MaxInt32)*time.Millisecond {
		return fmt.Errorf("KAFKA_FETCH_MAX_WAIT must be between 10ms and 2147483647ms")
	}
	if cfg.RebalanceTimeout < 100*time.Millisecond || cfg.RebalanceTimeout > time.Duration(math.MaxInt32)*time.Millisecond {
		return fmt.Errorf("KAFKA_REBALANCE_TIMEOUT must be between 100ms and 2147483647ms")
	}
	if err := validateKafkaRequestTimeout(cfg.RequestTimeout); err != nil {
		return err
	}
	switch cfg.InitialOffset {
	case KafkaInitialEarliest, KafkaInitialLatest:
	default:
		return fmt.Errorf("KAFKA_INITIAL_OFFSET must be earliest or latest, got %q", cfg.InitialOffset)
	}
	return nil
}
