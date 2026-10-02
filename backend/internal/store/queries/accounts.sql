-- name: GetAccount :one
SELECT * FROM accounts
WHERE name = ?;

-- name: CreateAccount :one
INSERT INTO accounts (id, name, password_hash)
VALUES (?, ?, ?)
RETURNING *;

-- name: TouchLastSeen :exec
UPDATE accounts SET last_seen = CURRENT_TIMESTAMP WHERE id = ?;

-- name: UpdateRating :one
UPDATE accounts SET rating = rating + ? WHERE id = ?
RETURNING rating;
