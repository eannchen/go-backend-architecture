-- name: Ping :one
SELECT 1;

-- name: GetServerStatus :one
SELECT
    current_database() AS database_name,
    pg_is_in_recovery() AS in_recovery,
    EXTRACT(EPOCH FROM (NOW() - pg_postmaster_start_time()))::BIGINT AS uptime_seconds;

-- name: IsVectorExtensionEnabled :one
SELECT CASE
    WHEN to_regtype('vector') IS NULL THEN FALSE
    ELSE TRUE
END AS enabled;