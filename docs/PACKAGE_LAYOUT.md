# Package layout: why simulation and transport still share a package

Everything server-side lives in one package, `../backend/internal/game`. This note records why,
and what the split would look like when it becomes worth doing.

## Current layout

`world.go` used to be ~500 lines holding the world struct, its config, the whole tick,
the round state machine and the send plumbing. It is now split by responsibility, all
still in package `game`:

| File | Holds |
| --- | --- |
| `world.go` | the `World` struct, `NewWorld`, `Run`, `EnqueueMovement` |
| `config.go` | world dimensions, tick rate, limits, `WorldConfig` and its clamps |
| `tick.go` | `Tick` and its four stages: `drainInputs`, `stepMovement`, `replicate`, `broadcastMarket` |
| `phase.go` | the round state machine: `setPhase`, `advancePhase`, `finish`, `netWorth` |
| `session.go` | `Join`, `addPlayer`, `removePlayer`, `initialState` |
| `send.go` | `sendTo`, `sendToAll` — the non-blocking send rule lives here |

## The boundary that actually exists

The package has two kinds of code in it, and they are not "world vs. the rest":

- **Simulation** — `world.go`, `tick.go`, `phase.go`, `session.go`, `config.go`, `send.go`,
  `trade.go`, `player.go`, `station.go`, `commodity.go`, `spatial_grid.go`, `geometry.go`.
  Knows nothing about sockets. `trade.go` is already `World` methods.
- **Transport** — `client.go`, the only file that imports `gorilla/websocket`.
  `game_master.go` sits on the line: it is a registry of worlds that transport calls into.

So the eventual split is `../backend/internal/game` (simulation) and `../backend/internal/transport`, not a
`game/world/` subdirectory. Moving only `world.go` down a level would put the world's own
files on both sides of a package boundary.

## What blocks it today

`World` and `Client` reference each other:

- `Client` holds `World *World` and reaches into unexported state — `c.World.removePlayer(c.ID)`
  and `c.World.tradeQueue <- o`.
- `World` holds `clients map[uint32]*Client`.

That is a genuine import cycle, but a shallow one. `World` only ever touches two things on a
`Client`: `.ID` and `.Send`. The break is therefore:

1. `clients map[uint32]chan []byte` instead of `map[uint32]*Client`.
2. `Join` takes the send channel and returns the `*Player`; the caller assigns `c.ID`.
3. Export `Leave(id)` and `EnqueueTrade(o)` so transport stops reaching into unexported
   fields, matching the existing `EnqueueMovement`.

Step 3 is worth doing on its own merits — it is the `joinQueue`/`exitQueue` TODO in
`world.go`, and it makes the queue surface consistent.

## Why it is deferred

The package is ~1500 lines with one transport. A package boundary would buy nothing that
the file boundaries above do not already buy, and it costs the cycle break plus churn in
`../backend/cmd`, `../backend/internal/sim` and the tests, which use unexported fields.

Revisit when a second transport appears. The browser and Unity clients share the one
WebSocket path today, so that has not happened yet.
