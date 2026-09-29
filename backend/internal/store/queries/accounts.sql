-- name: GetAccount :one
SELECT * FROM accounts
WHERE name = ?;

-- name: CreateAccount :one
INSERT INTO accounts (id, name, password_hash, rating)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: TouchLastSeen :exec
UPDATE accounts SET last_seen = CURRENT_TIMESTAMP WHERE id = ?;
