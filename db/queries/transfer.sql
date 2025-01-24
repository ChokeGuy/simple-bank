-- name: CreateTransfer :one
INSERT INTO
    transfers (from_account_id, to_account_id, amount)
VALUES
    ($1, $2, $3)
RETURNING *;

-- name: GetTransfers :many
SELECT
    from_account_id,
    to_account_id,
    amount,
    created_at
FROM
    transfers
WHERE
    from_account_id = $1 AND to_account_id = $2
ORDER BY
    created_at DESC;

-- name: GetTransfersByFromAccountId :many
SELECT
    from_account_id,
    to_account_id,
    amount,
    created_at
FROM
    transfers
WHERE
    from_account_id = $1
ORDER BY
    created_at DESC;

-- name: GetTransfersByToAccountId :many
SELECT
    from_account_id,
    to_account_id,
    amount,
    created_at
FROM
    transfers
WHERE
    to_account_id = $1
ORDER BY
    created_at DESC;