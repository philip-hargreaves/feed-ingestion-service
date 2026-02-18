-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, name)
VALUES (
           $1,
           $2,
           $3,
           $4
       )
    RETURNING *;

-- name: GetUser :one
SELECT * FROM users WHERE NAME = $1;

-- name: GetUsers :many
SELECT * FROM users ORDER BY created_at ASC;

-- name: ResetUsers :exec
DELETE FROM users;