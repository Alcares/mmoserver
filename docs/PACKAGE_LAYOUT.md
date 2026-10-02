# Package layout: simulation and transport

The server is split into two packages along the one boundary that exists in the code:

- **`../backend/internal/game`, simulation.** The world, the tick, the round state machine,
  trading, bots and the game registry (`Master`). It knows nothing about sockets or the
  database: it imports neither `gorilla/websocket` nor `internal/store`.
- **`../backend/internal/transport`, the WebSocket edge.** `WebsocketClient` with its read
  and write pumps, `Sessions` (the login registry, see `ACCOUNTS.md`), sign-up validation, and
  the mapping from `game` and `store` errors onto the rejections sent to the client. It imports
  `game` and `store`; nothing imports it except `cmd`.

## `game`

| File | Holds |
| --- | --- |
| `world.go` | the `World` struct, `NewWorld`, `Run`, `EnqueueMovement`, `EnqueueTrade` |
| `config.go` | world dimensions, tick rate, limits, `WorldConfig` and its clamps |
| `tick.go` | `Tick` and its stages: `drainInputs`, `stepMovement`, `decayCash`, `replicate`, `broadcastMarket` |
| `phase.go` | the round state machine: `setPhase`, `advancePhase`, `finish`, `netWorth` |
| `session.go` | `Join`, `Leave`, `addPlayer`, `removePlayer`, `initialState` |
| `send.go` | `Client`, `SendQueue`, `sendTo`, `sendToAll`; the non-blocking send rule lives here |
| `player.go` | `Player`, and `Account`: who a player is outside the game |
| `elo.go` | `updateElo` and `RatingChange` |
| `game_master.go` | `Master`, the registry of running worlds, and the end-of-game `Result` callback |

## `transport`

| File | Holds |
| --- | --- |
| `client.go` | `WebsocketClient`, `ReadPump` / `WritePump`, `ReadSession` / `ReadWorld` / `ReadCommand`, the rejection mappings |
| `session.go` | `Sessions`: one connection per account, close code `4001` for the replaced one |
| `account_validation.go` | `validateUsername` / `validatePassword` and the embedded common-password list |

## The seam

`World` never sees a `WebsocketClient`. It holds `clients map[uint32]Client`, where `Client`
is only `Enqueue(payload []byte)`, so bots, the training sim and websocket clients all plug in
the same way. Transport talks to a world only through its exported, goroutine-safe surface:
`Join`, `Leave`, `EnqueueMovement`, `EnqueueTrade` and `SpawnBot`.

Identity crosses as plain values: transport turns a logged-in `store.Session` into a
`game.Account` on `Join`, and `cmd/server` turns a finished game's `game.RatingChange`s into
`store.RatingChange`s before saving them.
