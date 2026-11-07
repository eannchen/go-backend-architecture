#!/usr/bin/env bash
# Purpose: prepare the Kafka topic before publishers or consumers use it.
# This script runs in the kafka-init container. It sends administrative requests
# to the broker; it does not publish messages or start a consumer.
# You can run it again safely: it creates a missing topic, checks an existing
# topic, and fails if the existing settings differ from the requested settings.

# Stop on a failed command or missing variable, including failures in a pipeline.
# A failed setup must be reported to Make rather than appear successful.
set -euo pipefail

# Step 1: choose where to connect and which topic to prepare.
# Environment variables can override these defaults:
# - KAFKA_BOOTSTRAP_SERVER: kafka:19092, the broker address reachable from Docker;
# - KAFKA_TOPIC: notifications.requested.v1, the topic name;
# - KAFKA_TOPIC_PARTITIONS: 3, the number of ordered message streams in the topic.
# An unset or empty variable uses its default. Three partitions can all live on
# one broker; three partitions do not require three brokers.
# Creating topics is an infrastructure setup task, separate from sending or
# receiving messages in the Go application.
bootstrap_server="${KAFKA_BOOTSTRAP_SERVER:-kafka:19092}"
topic="${KAFKA_TOPIC:-notifications.requested.v1}"
partitions="${KAFKA_TOPIC_PARTITIONS:-3}"

# Step 2: reject invalid settings before asking Kafka to create anything.
# Topic names may contain letters, digits, dots, underscores, and hyphens,
# have at most 249 characters, and cannot be just "." or "..".
if [[ ! "$topic" =~ ^[a-zA-Z0-9._-]+$ || "$topic" == "." || "$topic" == ".." || ${#topic} -gt 249 ]]; then
  echo "KAFKA_TOPIC must be a valid Kafka topic name" >&2
  exit 1
fi
# The partition count must be a whole number greater than zero.
if [[ ! "$partitions" =~ ^[1-9][0-9]*$ ]]; then
  echo "KAFKA_TOPIC_PARTITIONS must be a positive integer" >&2
  exit 1
fi

# Step 3: give Kafka's administrative tools time limits for their requests.
# A single request may wait up to 10 seconds; an administrative operation may
# wait up to 30 seconds overall, including retries. These limits apply to each
# tool invocation, not to the entire script or our Go publisher/consumer.
# Save these settings in a temporary file and remove it when this script exits.
client_config="$(mktemp)"
trap 'rm -f "$client_config"' EXIT
cat > "$client_config" <<'PROPERTIES'
request.timeout.ms=10000
default.api.timeout.ms=30000
PROPERTIES

# Step 4: create the topic if it does not already exist, using these settings:
# - partitions: the count chosen in Step 1 (three by default);
# - replication factor 1: each partition has one copy, with no backup replica;
# - min.insync.replicas 1: when a producer requests acknowledgments from all
#   in-sync replicas, at least one up-to-date copy must be available.
# With our single broker, that one copy is the partition leader itself.
# These are topic settings; listing broker addresses in a Go writer does not
# create extra copies or change the topic's replication factor.
# If the topic already exists, this command leaves its settings unchanged.
# That alone does not prove the settings match, so we check them next.
/opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server "$bootstrap_server" --command-config "$client_config" \
  --create --if-not-exists --topic "$topic" \
  --partitions "$partitions" --replication-factor 1 \
  --config min.insync.replicas=1

# Step 5: ask Kafka for the topic's actual settings and print its description.
# The output includes the partition count, copy count, and each partition's
# leader and in-sync replicas. This lets you inspect what Kafka actually created.
description="$(/opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server "$bootstrap_server" --command-config "$client_config" \
  --describe --topic "$topic")"
printf '%s\n' "$description"

# Read the actual partition and copy counts from that response. Then make a
# separate request for the effective minimum in-sync replica count, including
# any value inherited from the broker. Do not assume creation used our settings:
# an existing topic may have been created earlier with different values.
actual_partitions="$(awk '{for(i=1;i<NF;i++) if($i=="PartitionCount:") print $(i+1)}' <<< "$description")"
actual_replicas="$(awk '{for(i=1;i<NF;i++) if($i=="ReplicationFactor:") print $(i+1)}' <<< "$description")"
actual_min_isr="$(/opt/kafka/bin/kafka-configs.sh \
  --bootstrap-server "$bootstrap_server" --command-config "$client_config" \
  --describe --entity-type topics --entity-name "$topic" --all \
  | sed -n 's/^[[:space:]]*min\.insync\.replicas=\([^[:space:]]*\).*/\1/p')"

# Step 6: compare Kafka's actual settings with the settings we requested.
# If any differ, print the actual and expected values and exit with a failure.
# This script never changes an existing topic to fix a mismatch. For example,
# requesting four partitions for an existing three-partition topic fails here.
# Changing partition counts can move keys to different partitions for future
# messages, so that change should be deliberate rather than part of startup.
if [[ "$actual_partitions" != "$partitions" || "$actual_replicas" != "1" || "$actual_min_isr" != "1" ]]; then
  printf 'Topic %s settings differ: partitions=%s replicas=%s min.insync.replicas=%s; expected %s/1/1. No existing settings were altered.\n' \
    "$topic" "$actual_partitions" "$actual_replicas" "$actual_min_isr" "$partitions" >&2
  exit 1
fi
# Success: the topic exists and all three checked settings match.
# Publishers and consumers can now use it. This confirms topic setup only;
# it does not verify application message handling or provide broker redundancy.
printf 'Verified topic %s: partitions=%s replication-factor=1 min.insync.replicas=1\n' "$topic" "$partitions"
