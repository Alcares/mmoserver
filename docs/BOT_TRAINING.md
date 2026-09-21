# Bot Training: Development Plan

This is the plan for training a bot that plays the game, and for plugging the trained bot
back into the server. The code lives in `../backend/internal/bot/` (observation and policy),
`../backend/internal/sim/` (training environment) and `../backend/cmd/sim/` (the sim binary).

## Goal and constraints

The bot has to play under the same rules as a human:

- **Same actions.** It moves with `MovementCommand` and trades with `TradeRequest`, through
  the same queues as a websocket client. It can't set positions or balances directly.
- **Same information.** It only knows what arrives in `ServerMessage`s: the station layout
  from `InitialGameState`, its FOV-limited `WorldSnapshot` (itself at `players[0]`), `MarketState`,
  its own `PlayerInventory` and `TradeReceipt`s. It never reads `World.players`.

Both rules are enforced by how the code is organized, not by discipline:

- `backend/internal/bot` doesn't import `backend/internal/game`, so the Observer can only learn from protos.
- `World.Join` and `World.EnqueueMovement` are the only way into the world, for human players,
  the training sim and the in-server bot alike, so all three join and move identically.

## Architecture

Training and playing are separate jobs, and they run in different languages:

```
TRAINING (fast, headless, many worlds)
  Python PPO policy ──action int──► Go sim: EnqueueMovement → World.Tick → drain Client.Send
          ▲                                                                     │
          └────────────── obs, reward, terminated, truncated ◄── Observer ◄─────┘

PLAYING (real time, inside the server)
  World.Run ──ServerMessage──► Observer ──obs──► Go Policy (scripted or MLP) ──► EnqueueMovement
                                                        ▲
                                             policy.json exported from Python
```

- **Training** needs gradients and PPO, so it runs in Python (PyTorch).
- **Acting** is a small forward pass, so it runs in Go: the server, the spectator and
  production all use the same `Policy` interface.
- The **sim contains no policy**. It takes actions and returns observations; it doesn't
  matter whether those actions come from PyTorch, a script or a person.

## Contracts between Go and Python

These must be identical on both sides. Changing any of them invalidates every trained policy.

### Observation (`bot.Observation.Vectorise`)

`[x, y, dx, dy, dist]`, all divided by `worldSize` (500, mirroring `game.WorldMaxX`), length
`bot.ObsSize`:

| Index | Value | Why |
|---|---|---|
| 0, 1 | own position | Needed near the walls |
| 2, 3 | signed offset to the goal station, `station − bot` | Makes every position the same problem, so learning generalizes |
| 4 | distance to the goal | A small MLP can't cheaply compute `sqrt`; it's what decides "stop here" |

The goal commodity is not an observation from the server: it's the task given to the bot.

### Actions

9 discrete actions: stop plus 8 directions, matching what a keyboard can send. The table
mapping action index → `(vx, vy)` is defined **once, in Go**. Python only ever sees integers
0–8.

### Episode end

- **Terminated:** the bot is within `TradeRange` (5 units) of the goal station.
- **Truncated:** the step limit ran out (`MaxSteps`, initially 600 ticks = 30 s).

These are reported separately: PPO treats a real ending and a timeout differently.

## Roadmap

### Stage 1: moving to a trading station

The movement problem itself is simple (`normalize(goal − self)` solves it exactly). The real
purpose of this stage is to prove the entire pipeline end to end before the hard part.

- [x] **1. Steppable world.** `World.Run` is a thin real-time wrapper around `World.Tick`.
      The station shuffle takes an injected `*rand.Rand`.
- [x] **2. Observer.** `bot.Observer`: `Consume` stores what the server sent, `Encode(goal)`
      turns it into an `Observation`. Errors when not ready or when the goal station is
      missing. Tests in `obs_test.go`.
- [x] **3. `World.Join`.** Adds the player and queues `InitialGameState` and `PlayerInventory`
      before any snapshot. Used by the server; the sim will use it too.
- [x] **4. `World.EnqueueMovement`.** Non-blocking push onto the movement queue, used by
      `ReadPump`.
- [ ] **5. Movement test.** Join, enqueue, tick 10×, read the position from the last drained
      `WorldSnapshot`. Cases: straight `(1,0)` moves 6.0; diagonal `(1,1)` moves 6.0 total, not
      8.49; over-sized `(5,0)` is clamped to 6.0. Drain every tick so `Send` (capacity 32) never
      fills.
- [ ] **6. Doc comments.** `EnqueueMovement`: safe from any goroutine, no lock needed.
      `Join`: locks `Mu` itself, the caller must not hold it (Go mutexes aren't reentrant).
- [x] **7. Spawn.** Every player joins at `game.SpawnPos`, the centre of the map; the player
      cap is `game.MaxPlayers`. Episodes still differ because stations are reshuffled per seed.
      Open: a bot trained only from the centre may not generalize to mid-game starts (after
      trading at one station and heading to the next). If it doesn't, the env can start
      episodes from random positions, but through a path the server doesn't expose.
- [x] **8. `backend/internal/sim/env.go`.** A Gym-style environment:
      - `Reset(seed)`: new world, `Join` a client, random goal picked from the stations the
  Observer received, one `Tick` + drain so the first `Encode` succeeds.
  - `Step(vx, vy)`: `EnqueueMovement` → `Tick` → drain `Send`, `Unmarshal`, `Consume` →
        `Encode` → reward, terminated, truncated.
  - Reward: progress `(prevDist − dist)`, a small per-step penalty, a bonus on arrival.
  - Takes `(vx, vy)` for now, so the env can be tested before the action table exists.
- [x] **9. Env tests.** `TestEnvReachesGoal`: step with `(dx, dy)` from the observation and
      assert it terminates in about `distance / (MoveSpeed × TickDuration)` steps. Plus: the
      same seed gives the same first observation.
- [ ] **10. `backend/cmd/sim` benchmark.** Run 1000 episodes with the one-line policy and print
  episodes per second. That number decides whether batching is needed before the bridge.
- [ ] **11. Action table and `Policy` interface.** `type Action int`, the 9-entry direction
      table, `Policy.Act(obs) Action`, and `ScriptedPolicy` (nearest of the 8 directions, stop
      inside `TradeRange`). Test whether it can get stuck going back and forth between two
      directions near the goal. It serves as test, baseline and something to watch.
- [ ] **12. Python bridge.** `backend/api/proto/sim/v1/env.proto` with a batched service
  (`Reset(seed, n_envs)`, `Step(actions[]) → obs[], reward[], terminated[], truncated[]`),
  kept separate from the game protocol. `backend/cmd/sim` serves it over gRPC.
- [ ] **13. Training.** A `gymnasium.vector.VectorEnv` wrapper, PPO from Stable-Baselines3 or
      CleanRL, a 2×64 MLP. Log success rate and `ticks taken / optimal ticks` (should approach
      1.0) to TensorBoard. Curriculum: nearby goals first, then farther ones.
- [ ] **14. Export and Go inference.** Python writes the weights to `policy.json`. `MLPPolicy`
      in Go does the forward pass (a few matrix multiplications, no cgo). Compare against
      `ScriptedPolicy`.
- [ ] **15. In-server bot.** `bot.Spawn(world, policy)`: a `Client` with a `Send` buffer and no
      websocket, joined through `World.Join`. A goroutine drains `Send`, runs the Observer and
      policy, and calls `EnqueueMovement`. Needs a removal path (today removal lives in
      `ReadPump`'s defer) and `Client.Conn` must be optional.

### Watching training in Unity

Training envs run far faster than 20 Hz, so they can't be streamed directly. Instead, watch an
evaluation world:

- [ ] **16. Spectator server.** A 20 Hz world (optionally 2–4× speed) running the in-server bot,
      reloading `policy.json` whenever Python writes a new checkpoint.
- [ ] **17. Spectate endpoint.** `/ws?spectate=<botID>` copies the bot's `Send` stream to the
      websocket instead of adding a player. The bot stays `players[0]`, so the Unity camera
      follows it and shows exactly its FOV. Spectator input is ignored.
- [ ] **18. Episode resets.** Resend `InitialGameState` on reset (Unity already rebuilds the
      stations when it receives it). Snap instead of smoothing when the bot jumps more than
      ~20 units.
- [ ] **19. Unity changes.** A `spectating` flag so "me" animates from velocity instead of
      keyboard input (`WorldView.cs`). A `BotDebug` `ServerMessage` (goal, last action, reward,
      episode, checkpoint), sent only to spectators, drawn as a line to the target plus HUD text.

### Stage 2: trading

- Observation grows to include `MarketState` quotes, `PlayerInventory` and `TradeReceipt`s
  (the fields already stubbed in `Observer`).
- Actions add buy and sell for each order size in `OrderSizes`.
- Reward becomes the change in net worth.
- A possible split: the policy picks a target station and a trade; scripted steering walks
  there.
- Behaviour cloning from `ScriptedPolicy` recordings can pre-train the network, so PPO doesn't
  start from random wandering.
- Self-play: several bots per world, so prices respond to other traders.

## Open questions

- **Latency fairness.** An in-server bot reacts with zero latency; humans have network
  round-trips. If that matters, delay the bot's actions by a few ticks, in the sim and in
  production alike.
- **Decision rate.** Should the bot decide every tick, or repeat each action for a few ticks
  (faster training, closer to human reaction time)?
- **Observation encoding.** `[dx, dy]` gets tiny near the goal. `[dx/dist, dy/dist, dist]`
  keeps the direction at full size; worth trying if the bot is imprecise when arriving.
- **World size.** `worldSize` in `backend/internal/bot` duplicates `game.WorldMaxX`. The server could
  send it in `InitialGameState` instead.
