#!/usr/bin/env bash
# Purpose: prepare the broker-owned policy for a Kafka Share Group.
# This script creates/updates group configuration, not a worker or topic.
# Repeating it applies the requested policy again; it never resets record progress.
set -euo pipefail

# Step 1: choose the group and its initial policy. Unlike ordinary consumer
# groups, this initial position belongs to Kafka, not to a reader option.
# earliest means old retained records are eligible when the group starts;
# latest means only future records. Neither choice rewinds existing progress.
bootstrap_server="${KAFKA_BOOTSTRAP_SERVER:-kafka:19092}"
group="${KAFKA_SHARE_GROUP_ID:-shared-workers-v1}"
initial_offset="${KAFKA_SHARE_INITIAL_OFFSET:-earliest}"
lock_ms="${KAFKA_SHARE_RECORD_LOCK_MS:-30000}"
if [[ ! "$group" =~ ^[a-zA-Z0-9][a-zA-Z0-9._-]*$ || ${#group} -gt 249 ]]; then
  echo "KAFKA_SHARE_GROUP_ID must be a simple group name (letters, digits, dot, underscore, hyphen; start with a letter/digit; maximum 249 characters)" >&2
  exit 1
fi
if [[ "$initial_offset" != "earliest" && "$initial_offset" != "latest" ]]; then
  echo "KAFKA_SHARE_INITIAL_OFFSET must be earliest or latest" >&2
  exit 1
fi
if [[ ! "$lock_ms" =~ ^[1-9][0-9]*$ ]]; then
  echo "KAFKA_SHARE_RECORD_LOCK_MS must be a positive whole number of milliseconds" >&2
  exit 1
fi

# Step 2: bound each administration operation. A failed connection should make
# setup fail visibly rather than wait indefinitely. Remove the temporary file
# when this script exits.
client_config="$(mktemp)"
trap 'rm -f "$client_config"' EXIT
cat > "$client_config" <<'PROPERTIES'
request.timeout.ms=10000
default.api.timeout.ms=30000
PROPERTIES

# Step 3: enable the Share Groups feature if an older local data volume still
# has it disabled. New 4.2 clusters already enable it. This changes the cluster
# feature level, not topic partition counts or record progress. Do not delete
# the volume to enable a feature; saved messages must remain intact.
features="$(/opt/kafka/bin/kafka-features.sh --bootstrap-server "$bootstrap_server" \
  --command-config "$client_config" describe)"
share_version="$(awk '$2=="share.version" {for(i=1;i<NF;i++) if($i=="FinalizedVersionLevel:") print $(i+1)}' <<< "$features")"
case "$share_version" in
  0)
    /opt/kafka/bin/kafka-features.sh --bootstrap-server "$bootstrap_server" \
      --command-config "$client_config" upgrade --feature share.version=1
    ;;
  1) ;;
  *) echo "Expected a broker with share.version 0 or 1; found '$share_version'" >&2; exit 1 ;;
esac

# Step 4: set the group's initial position policy and per-record lock duration.
# A lock gives a consumer time to handle a record; expiry allows another attempt.
# It does not stop the original handler, so duplicate effects remain possible.
# Kafka validates this duration against the broker's allowed minimum/maximum.
/opt/kafka/bin/kafka-configs.sh --bootstrap-server "$bootstrap_server" \
  --command-config "$client_config" --alter --entity-type groups --entity-name "$group" \
  --add-config "share.auto.offset.reset=$initial_offset,share.record.lock.duration.ms=$lock_ms"

# Step 5: print the group configuration so the chosen values can be reviewed.
# Membership begins when a ShareReader actually connects using this group ID.
/opt/kafka/bin/kafka-configs.sh --bootstrap-server "$bootstrap_server" \
  --command-config "$client_config" --describe --entity-type groups --entity-name "$group"
