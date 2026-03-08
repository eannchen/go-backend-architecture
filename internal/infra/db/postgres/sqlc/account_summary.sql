-- name: GetAccountSummaryByID :one
SELECT
    id,
    email,
    display_name,
    plan,
    status,
    EXTRACT(EPOCH FROM updated_at)::BIGINT AS updated_at_unix
FROM account_summaries
WHERE id = $1;
