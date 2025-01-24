-- name: CreateAccount :one
INSERT INTO
    accounts (owner, balance, currency)
VALUES ($1, $2, $3) RETURNING *;

-- name: GetAccount :one
SELECT
    id,
    owner,
    balance,
    currency,
    created_at
FROM
    accounts
WHERE
    id = $1 LIMIT 1;

-- name: ListAccounts :many
SELECT
    id,
    owner,
    balance,
    currency,
    created_at
FROM
    accounts
ORDER BY
    id
LIMIT  $1
OFFSET $2;

-- name: UpdateAccount :one
UPDATE accounts
SET 
    balance = $2
WHERE
    id = $1
RETURNING *;

-- name: DeleteAccount :exec
DELETE FROM
    accounts
WHERE
    id = $1;