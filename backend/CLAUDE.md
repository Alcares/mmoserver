# Backend conventions

Go module `github.com/alcares/mmoserver/backend`. `go.mod` lives here, not at the repo root, so
run `go -C backend ...` from the root or use the `make` targets.

## Rules
- Only `World.Run` mutates game state; `ReadPump`s only push onto queues, `WritePump`s only drain `Client.Send`. The exception is login and sign-up, which run bcrypt and database queries on the `ReadPump` before the client is in a world
- All SQL lives in `internal/store`: the schema in `migrations/NNN_*.sql`, queries in `queries/`. Never edit a migration that has run anywhere; add a new file. `make sqlc` regenerates `gen/go/db`, which is never edited by hand. Nothing outside `store` imports `gen/go/db` or the SQLite driver; see `../docs/ACCOUNTS.md`
- All sends are non-blocking; drop the message when a buffer is full
- Keep function "Doc" comments brief and avoid referencing other files or values in them
- Each tick sends every client a `WorldSnapshot` with only the players inside its FOV radius, found through `SpatialGrid`
- Pricing lives in `internal/game/commodity.go`: `buyPrice(n)`/`sellPrice(n)` depend on the order size, and pool depth per commodity is `poolUnits`. Rounding always favours the pool; see `../docs/PRICING.md`
- Game events go to `events.jsonl`, one JSON object per line, through `WorldConfig.Logger`. `sanitize` defaults it to a discarding logger so the sim and the tests stay silent — never nil-check it at a call site
- The server binary runs with the repo root as its working directory: `backend/events.jsonl`, `backend/server.log`, `rl-training/policy.pb` and the database `db.sql` are all relative to it
