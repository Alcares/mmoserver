# Accounts: login, sessions and persistence

Players log in with a global username and a password before they can create or join a game.
Accounts live in SQLite through `../backend/internal/store`; which connection belongs to which
account lives in memory in `game.Sessions`. This records how it fits together and why.

## Flow

```
connect ──▶ Login / CreateAccount ──▶ FindGame / CreateGame / JoinGame ──▶ MovementCommand, TradeRequest, ...
            (ReadSession)               (ReadWorld)                         (ReadCommand)
```

`ReadPump` moves through three stages and drops any message that doesn't belong to the current
one: nothing but `Login` and `CreateAccount` is accepted until `c.Session` is set, and game
commands only once `c.World` is set.

## Decisions

- **One global username, no separate display name.** The username is the login and the name
  other players see; `FindGame`, `CreateGame` and `JoinGame` carry no name. A display name was
  considered and dropped: it needs its own uniqueness rules, and without them anyone can show up
  as anyone.
- **The per-world `Player.ID` stays.** It is the 4-byte handle in every `WorldSnapshot`, the key
  of `w.players` and `w.clients`, and bots have one without having an account. Anything that
  outlives a game (ratings, match results) must key on the account ID instead.
- **Account IDs are UUIDv7**, generated in Go, stored as a 16-byte `BLOB`.
- **Passwords are bcrypt hashes.** The lookup is by name only and `bcrypt.CompareHashAndPassword`
  checks the password in Go; the salt is inside the stored hash. bcrypt rejects passwords over
  72 bytes, which is what `PASSWORD_TOO_LONG` is for.
- **One reply per request, success included.** `LoginResult` and `AccountCreateResult` carry a
  rejection enum where `UNSPECIFIED` means success, like `TradeRejection` in `TradeReceipt`.
  `SERVER_ERROR` covers everything that isn't the player's fault; the cause goes to the log.
- **Uniqueness is the database's job.** `CreateAccount` inserts and maps SQLite's
  `SQLITE_CONSTRAINT_UNIQUE` to `USERNAME_TAKEN`. Checking first and inserting after would let
  two simultaneous sign-ups both win. All other rules (length, characters, password strength)
  are checked in Go, which is the only side that sees the password and can return a reason.
- **One session per account; a new login kicks the old one.** Rejecting the new login would
  lock a player out whenever their old connection is dead but not yet detected. `Sessions.replace`
  swaps the map entry under the lock, then sends the old socket a close frame with code `4001`
  and closes it; the old `ReadPump` fails and runs its normal cleanup. `Sessions.remove` only
  deletes the entry if it still points at the leaving client, so a kicked client's cleanup never
  removes the session that replaced it. Clients must not reconnect automatically on `4001`, or
  two open clients kick each other forever.
- **Login and sign-up block `ReadPump`.** bcrypt (~50-100 ms) and the queries run on that
  client's own read goroutine. They touch no game state and only delay that one client.

## Storage

- **SQLite, `modernc.org/sqlite`.** One server process, a handful of writes per game: SQLite is
  plenty, needs nothing running beside the server, and the pure-Go driver keeps the build free
  of cgo. `store.Open` sets WAL, `busy_timeout`, `foreign_keys` and `_txlock=immediate` in the
  DSN so every pooled connection gets them, and caps the pool at one connection.
- **sqlc** generates `../backend/gen/go/db` from `migrations/` (the schema) and `queries/`
  (`make sqlc`). Nothing outside `store` imports `gen/go/db` or the SQLite driver, and `store`
  imports no wire protocol: it returns Go errors (`ErrInvalidCredentials`, `ErrUsernameTaken`),
  which `transport` maps onto the rejection sent to the client. `game` imports `store` not at all:
  `Join` takes a `game.Account`, and finished games report `game.RatingChange`s that
  `cmd/server` converts for `store`.
- **Migrations** are numbered files in `internal/store/migrations`, applied in order at startup
  and tracked with `PRAGMA user_version`. One that has run anywhere is never edited; a change
  is a new file.

| File | Holds |
| --- | --- |
| `backend/internal/store/store.go` | `Open`, `Accounts` (`Login`, `CreateNewAccount`), `Session`, the errors |
| `backend/internal/transport/account_validation.go` | `validateUsername` / `validatePassword` and the common-password list |
| `backend/internal/store/migrations/*.sql` | the schema, one numbered file per change |
| `backend/internal/store/queries/*.sql` | the sqlc queries |
| `backend/sqlc.yaml` | sqlc config; output goes to `backend/gen/go/db` |
| `backend/internal/transport/session.go` | `Sessions`: the account ID to connection registry |
| `backend/internal/transport/client.go` | `ReadSession`, which validates sign-ups and maps store errors onto rejections, and the `Sessions.remove` call in `closeConnection` |

## Moving to Postgres

Only `store`, the SQL files and `sqlc.yaml` change:

| Piece | SQLite | Postgres |
| --- | --- | --- |
| `store.Open` | DSN pragmas, one connection | `pgxpool` or `pgx/stdlib` |
| Placeholders | `?` | `$1, $2` |
| `sqlc.yaml` | `engine: sqlite` | `engine: postgresql` |
| Unique violation | `sqlite.Error` code 2067 | `pgconn.PgError` code `23505` |
| Migration tracking | `PRAGMA user_version` | a `schema_migrations` table, or goose |
| Types | `BLOB`, `REAL`, `COLLATE NOCASE` | `uuid`, `double precision`, `citext` |

Keep SQLite-only syntax in the migrations, not the queries, and keep every driver error check
inside `store`, and the switch stays this small.

Switch when a second server process exists, not before. The database is not the bottleneck:
`maxGames` is, and after it two things that are in-memory today:

- **`Sessions` is per process.** Two servers would each allow the same account once. It needs a
  shared table plus a cross-server kick (Postgres `LISTEN/NOTIFY`, or Redis).
- **A world lives in the process that created it.** A game code would have to name its server,
  and a lobby would route clients there.

## Open

Tracked in `../TODO.md`:

- Apply the migrations in `store.Open`; a fresh database has no `accounts` table yet.
- Real `validateUsername` / `validatePassword`; both accept everything.
- `wss://`: passwords currently cross the network in plain text.
- Rejoining after a disconnect: keep the `Player` for a grace period and reattach a login for
  the same account to it, instead of removing it. Also changes what a kick does.
- `accountID` on `Player`, so Elo and match results key on the account.
- Ping/pong: `joinWorld` clears the read deadline, so a silently dead connection stays in its
  game until the round ends.
- A limit on failed logins per connection.
- The Unity client: a login screen, and dropping `PlayerName` from `CreateGame` / `JoinGame`.
