# Send Dispatcher (proposal)

Moves `proto.Marshal` off the tick. Today `sendTo`/`sendToAll` (`../backend/internal/game/send.go`)
marshal on the tick goroutine under `World.Mu`; in `BenchmarkStep` that is ~18% of CPU and
about half of `World.Tick`, and it grows with player count since `replicate` marshals one
snapshot per player. `Enqueue` is already non-blocking, so the marshalling is the only cost worth moving.

## Design

One dispatcher goroutine per `World`, fed by a buffered channel of `{target, msg}`
(target is a player ID, or "all").

- `sendTo`/`sendToAll` only push onto that channel; the dispatcher marshals and calls `Enqueue`.
  `sendToAll` still marshals once.
- The dispatcher **owns the clients map**. Join and leave are messages on the same channel,
  and leave is where `close(c.Send)` happens, not in `ReadPump`. Nothing can enqueue onto a
  closed `Send`, which would panic.
- One FIFO keeps per-client order, so `InitialState` and `PlayerInventory` still arrive
  before the first `WorldSnapshot`.
- `Flush()` blocks until everything queued so far has been enqueued. `sim.Env` calls it after
  each `Tick`, before `drain()`, so the RL env stays synchronous and deterministic.

## Rules this adds

- A message handed to the dispatcher must not alias live world state. Every `ToProto*` helper
  has to build fresh values, because marshalling now happens after `World.Mu` is released.
- If the dispatcher's channel is full, drop rather than block the tick, like `Enqueue` does.
  Join/leave must never be dropped, so they block instead.
