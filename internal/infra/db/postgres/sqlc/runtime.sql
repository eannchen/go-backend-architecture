-- name: Ping :one
SELECT 1;

-- name: GetRuntimeValue :one
SELECT key, value
FROM system_runtime_kv
WHERE key = $1;
