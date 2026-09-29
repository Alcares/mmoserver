CREATE TABLE accounts (
    id                  BLOB PRIMARY KEY, -- Stores the 16-byte binary UUIDv7
    name                TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    password_hash       BLOB    NOT NULL,
    rating              REAL    NOT NULL DEFAULT 1000,
    games_played        INTEGER NOT NULL DEFAULT 0,
    created_at          TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen           TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);