-- name: Ping :one
SELECT 1;

-- name: GetServerStatus :one
SELECT
    current_database() AS database_name,
    pg_is_in_recovery() AS in_recovery,
    EXTRACT(EPOCH FROM (NOW() - pg_postmaster_start_time()))::BIGINT AS uptime_seconds;

-- name: GetRuntimeValue :one
SELECT key, value
FROM system_runtime_kv
WHERE key = $1;
