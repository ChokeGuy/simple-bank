-- name: CreateEntry :one
INSERT INTO
    entries (account_id, amount)
VALUES ($1, $2)
RETURNING *;

-- name: GetEntryByAccountId :one
SELECT
    id,
    account_id,
    amount,
    created_at
FROM 
    entries
WHERE
    account_id = $1
ORDER BY id 
LIMIT 1;


-- name: ListEntriesByAccountId :many
SELECT 
    id,
    account_id,
    amount,
    created_at
FROM
    entries
WHERE
    account_id = $1
ORDER BY id
LIMIT  $2
OFFSET $3;

-- name: UpdateEntry :one
UPDATE entries
SET 
    amount = $2
WHERE
    id = $1
RETURNING *;

-- name: DeleteEntry :exec
DELETE FROM
    entries
WHERE
    id = $1;