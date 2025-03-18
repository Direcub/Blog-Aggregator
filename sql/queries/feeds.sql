-- name: GetNextFeedToFetch :one
SELECT id, created_at, updated_at, name, url, user_id, last_fetched_at FROM feeds
ORDER BY feeds.last_fetched_at ASC NULLS FIRST
LIMIT 1;