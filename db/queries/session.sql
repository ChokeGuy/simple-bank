-- name: CreateSession :one
INSERT INTO
    sessions (
        id,
        username,
        refresh_token,
        user_agent,
        client_ip,
        is_blocked,
        expires_at
    )
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetSessionById :one
SELECT
    id,
    username,
    refresh_token,
    user_agent,
    client_ip,
    is_blocked,
    expires_at
FROM
    sessions
WHERE
    id = $1
LIMIT 1;


-- name: GetSessionByUserName :one
SELECT
    id,
    username,
    refresh_token,
    user_agent,
    client_ip,
    is_blocked,
    expires_at
FROM
    sessions
WHERE
    username = $1
LIMIT 1;

-- name: DeleteSession :exec
DELETE FROM
    sessions
WHERE
    id = $1;