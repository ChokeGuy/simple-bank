-- name: CreateUser :one
INSERT INTO
    users (username,hashed_password,full_name,email)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetUserByUserName :one
SELECT 
    username,
    hashed_password,
    role,
    full_name,
    email,
    is_email_verified,
    password_changed_at,
    created_at
FROM 
    users
WHERE 
    username = $1;

-- name: UpdateUser :one
UPDATE 
    users
SET
    hashed_password = COALESCE(sqlc.narg(hashed_password), hashed_password),
    is_email_verified = COALESCE(sqlc.narg(is_email_verified), is_email_verified),
    full_name = COALESCE(sqlc.narg(full_name), full_name),
    email = COALESCE(sqlc.narg(email), email)
WHERE
    username = sqlc.arg(username)
RETURNING *;
