# Backend conventions

Go module `github.com/alcares/mmoserver/backend`. `go.mod` lives here, not at the repo root, so
run `go -C backend ...` from the root or use the `make` targets.

## Rules
- Only `World.Run` mutates game state; `ReadPump`s only push onto queues, `WritePump`s only drain `Client.Send`
- All sends are non-blocking; drop the message when a buffer is full
- Each tick sends every client a `WorldSnapshot` with only the players inside its FOV radius, found through `SpatialGrid`
- Pricing lives in `internal/game/commodity.go`: `buyPrice(n)`/`sellPrice(n)` depend on the order size, and pool depth per commodity is `poolUnits`. Rounding always favours the pool; see `../docs/PRICING.md`
- Game events go to `events.jsonl`, one JSON object per line, through `WorldConfig.Logger`. `sanitize` defaults it to a discarding logger so the sim and the tests stay silent — never nil-check it at a call site
- The server binary runs with the repo root as its working directory: `./web`, `./backend/api/proto/game` and `backend/events.jsonl` are all relative to it
