# Bot Training: Development Plan

This is the plan for training a bot that plays the game, and for plugging the trained bot
back into the server. The code lives in `../backend/internal/bot/` (observation and policy),
`../backend/internal/sim/` (training environment) and `../backend/cmd/sim/` (the sim binary).

## Goal and constraints

The bot has to play under the same rules as a human:

- **Same actions.** It moves with `MovementCommand` and trades with `TradeRequest`, through
  the same queues as a websocket client. It can't set positions or balances directly.
- **Same information.** It only knows what arrives in `ServerMessage`s: the world size from
  `InitialGameState`, its FOV-limited `WorldSnapshot` (itself at `players[0]`), and later
  `MarketState`, its own `PlayerInventory` and `TradeReceipt`s. It never reads `World.players`.
  The station layout arrives in the same message and goes back on the `Observer` in Stage 2,
  when the policy starts choosing its own target.

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
                                             policy.pb exported from Python
```

- **Training** needs gradients and PPO, so it runs in Python (PyTorch).
- **Acting** is a small forward pass, so it runs in Go: the server, the spectator and
  production all use the same `Policy` interface.
- The **sim contains no policy**. It takes actions and returns observations; it doesn't
  matter whether those actions come from PyTorch, a script or a person.

Training in Go was considered and **rejected**; the split above is settled. The bridge is not
what costs - measured, 229k steps/s native, 152k through gRPC under a numpy policy, but 7.4k
once PPO is training, which is PyTorch's per-op overhead on 5-wide tensors rather than
transport. Hand-writing PPO over a 2x64 MLP is only ~250 lines on top of the forward pass step
14 needs anyway, and it would delete step 12 and most of 14. What it would cost is experiment
velocity while rewards and observations are still moving, and the reference implementation that
says whether a bad run is the algorithm or the environment - which Stage 2, having no scripted
control, needs more than Stage 1 did. So the gRPC bridge and everything in `rl-training/` are
permanent infrastructure, not scaffolding.

## Contracts between Go and Python

These must be identical on both sides. Changing any of them invalidates every trained policy.

### Observation (`bot.Observation.Vectorise`)

`[x, y, dx, dy, dist]`, all divided by the world size the server sent in `InitialGameState`,
length `bot.ObsSize`:

| Index | Value | Why |
|---|---|---|
| 0, 1 | own position | Needed near the walls |
| 2, 3 | signed offset to the goal, `goal − bot` | Makes every position the same problem, so learning generalizes |
| 4 | distance to the goal | A small MLP can't cheaply compute `sqrt`; it's what decides "stop here" |

The goal is a `bot.Goal` position, not an observation from the server: it's the task given to
the bot. A trading station is one such point; `Env` samples the whole map so that training
covers every bearing and distance rather than the six the fixed layout produces.

### Actions

9 discrete actions: stop plus 8 directions, matching what a keyboard can send. The table
mapping action index → `(vx, vy)` is defined **once, in Go**. Python only ever sees integers
0–8.

### Episode end

- **Terminated:** the bot is within `TradeRange` (3 units) of the goal.
- **Truncated:** the step limit ran out (`MaxSteps`, 1200 ticks = 60 s).

These are reported separately: PPO treats a real ending and a timeout differently.

## Roadmap

### Stage 1: moving to a trading station

The movement problem itself is simple (`normalize(goal − self)` solves it exactly). The real
purpose of this stage is to prove the entire pipeline end to end before the hard part.

- [x] **1. Steppable world.** `World.Run` is a thin real-time wrapper around `World.Tick`.
      The station shuffle takes an injected `*rand.Rand`.
- [x] **2. Observer.** `bot.Observer`: `Consume` stores what the server sent, `Encode(Goal)`
      turns a goal position into an `Observation`. Errors before the first snapshot or when no
      world size has arrived. Tests in `obs_test.go`.
- [x] **3. `World.Join`.** Adds the player and queues `InitialGameState` and `PlayerInventory`
      before any snapshot. Used by the server; the sim will use it too.
- [x] **4. `World.EnqueueMovement`.** Non-blocking push onto the movement queue, used by
      `ReadPump`.
- [x] **5. Movement test.** Join, enqueue, tick 10×, read the position from the last drained
      `WorldSnapshot`. Cases: straight `(1,0)` moves 6.0; diagonal `(1,1)` moves 6.0 total, not
      8.49; over-sized `(5,0)` is clamped to 6.0. Drain every tick so `Send` (capacity 32) never
      fills.
- [x] **6. Doc comments.** `EnqueueMovement`: safe from any goroutine, no lock needed.
      `Join`: locks `Mu` itself, the caller must not hold it (Go mutexes aren't reentrant).
- [x] **7. Spawn.** Every player joins at `game.SpawnPos`, the centre of the map; the player
      cap is `game.MaxPlayers`. The per-seed shuffle moves commodities between six fixed
      ellipse points, so it varies labels, not geometry; episodes differ because the goal is
      sampled per seed.
      Open: a bot trained only from the centre may not generalize to mid-game starts (after
      trading at one station and heading to the next). If it doesn't, the env can chain
      episodes — continue from where the last one ended — which needs no path the server
      doesn't expose.
- [x] **8. `backend/internal/sim/env.go`.** A Gym-style environment:
      - `Reset(seed)`: new world, `Join` a client, a uniformly random goal point in the
  world, one `Tick` + drain so the first `Encode` succeeds.
  - `Step(Action)`: `EnqueueMovement` → `Tick` → drain `Send`, `Unmarshal`, `Consume` →
        `Encode` → reward, terminated, truncated.
  - Reward: progress `(prevDist − dist)`, a small per-step penalty, a bonus on arrival.
  - The 9 actions are the whole input space, so the env takes nothing a client couldn't send.
- [x] **9. Env tests.** `TestEnvReachesGoal`: walk under `ScriptedPolicy` and assert it
      terminates in about `distance / (MoveSpeed × TickDuration)` steps. Plus: the
      same seed gives the same first observation.
- [x] **10. Throughput benchmark.** `BenchmarkEpisode` and `BenchmarkStep` in
  `internal/sim/env_bench_test.go` (not `cmd/sim`, which is still a stub) walk episodes under
  `ScriptedPolicy` and report steps/s and steps/ep. That number decides whether batching is
  needed before the bridge: currently ~220k steps/s.
- [x] **11. Action table and `Policy` interface.** `type Action int`, the 9-entry direction
      table, `Policy.Act(*Observation) Action`, and `ScriptedPolicy` (nearest of the 8
      directions, stop inside `TradeRange`). It serves as test, baseline and something to
      watch. It does not get stuck near the goal: one step is 0.6 units against a 5-unit stop
      radius, so it cannot hop the band and reverse, and `walkToGoal` asserts no action is ever
      the exact opposite of the one before it.
- [x] **12. Python bridge.** `backend/api/proto/sim/v1/env.proto`, kept separate from the game
      protocol and served over gRPC by `backend/cmd/sim`. The transport is a detail; the batch
      is not. One env step is ~4.5 µs against a ~100 µs local round trip, so a call per env
      would spend most of training in transport. One call carries the whole vector:
      `Reset(seeds[]) → obs[]`, `Step(actions[]) → obs[], reward[], terminated[], truncated[]`.
      Two things the schema has to pin down, because both fail silently rather than erroring:
      - **Seeds are per env.** One seed shared across the vector makes every env run the same
        episode, and the batch stops reducing variance.
      - **Autoreset.** If a finished env restarts in place, `observations` holds the next
        episode's first observation, and the one the episode ended on has to travel separately:
        PPO bootstraps a truncated episode's value from it. Today `Env.Step` errors after the
        episode ends, so this is a decision, not a given.
- [x] **13. Training.** `rl-training/src/rl_training/`: `env.py` wraps the sim as a
      Stable-Baselines3 `VecEnv`, `baseline.py` measures the scripted and random policies
      through that wrapper, `train.py` runs PPO over a 2×64 MLP and logs to TensorBoard.
      `make baseline` and `make train`; both spawn their own sim, so no stale binary survives a
      Go change. The run logs `bot/success_rate` and `bot/steps_over_optimal` beside SB3's own
      curves, because the shaped reward can rise while the bot still fails to arrive. A run
      saves `rl-training/policy.zip`, which is SB3's checkpoint - pickled tensors plus Adam's
      state, larger than the weights themselves - so it is for resuming training, not for Go.
      1.0 is not reachable: `optimal` is a straight line, and quantising to 45° costs up to
      `1/cos(22.5°)` = 1.082, so `ScriptedPolicy` measures 1.05 mean and 1.14 worst over random
      goals. That is the number to match, not to beat. Progress per tick is `0.6 × cos(θ)` for
      the angle between the action and the true bearing, so the nearest of the 8 directions is
      the best single action, and mixing two of them does not help: alternating N and NE toward
      a 22.5° bearing projects the same `cos(22.5°)` whatever the ratio. `ScriptedPolicy` is
      already optimal for this action set, and the only headroom is the final step, landing just
      inside `TradeRange` rather than overshooting into it. What this step buys is the pipeline
      — observation contract, autoreset, reward, gradients, export — checked on the last task
      whose answer is known in advance; Stage 2 has no scripted control to check against.

      Three things the implementation pinned down:
      - **SB3's `VecEnv`, not `gymnasium.vector.VectorEnv`.** Gymnasium 1.0 resets an env on the
        step *after* it terminates and throws that action away. The Go server resets in place
        and sends the terminal observation alongside, which is SB3's convention exactly, so
        `final_observations` maps straight onto `infos[i]["terminal_observation"]` and neither
        side needs a shim.
      - **Go supplies the metric's denominator.** `StepResponse.optimal_steps`, filled only for
        envs that just finished, because deriving it needs `TradeRange` and the per-tick step.
        The numerator is the episode length the wrapper already counts, so it stays in Python.
        `ResetResponse.action_count` joins `obs_size` for the same reason: Python invents no
        constant the game owns. `optimal_steps` is scoped to *this* task, though - "fewest ticks
        to reach a point" means nothing once the objective is net worth - and step 14 takes over
        its regression-test job, leaving it only the live training curve.
      - **No normalisation anywhere in Python.** Whatever the wrapper did to an observation,
        Go's `MLPPolicy` would have to redo at inference, so `policy.pb` stays the complete
        description of the bot.

      Measured over 2000 held-out episodes: PPO 100% success at 1.053 mean / 1.120 worst,
      against `ScriptedPolicy`'s 1.052 / 1.120 through the same wrapper. Matched, as the
      argument above says it must be. Uniform random is 0% at 5.16, which is the floor the
      curve had to climb from. No curriculum was needed: the progress reward is dense from the
      first step of the first episode, so there is no sparse-reward problem for one to solve.
- [x] **14. Export and Go inference.** Python reads `policy.zip` and writes `policy.pb`.
      These are two artifacts, not one renamed: the zip is SB3's checkpoint, unreadable from Go
      without reimplementing Python's pickle format, and it carries optimizer state and
      hyperparameters the bot never uses. Its schema is `api/proto/bot/v1/policy.proto`, so
      both sides use generated code and a field rename breaks the build instead of reading
      zero; binary rather than JSON, since 5k floats are unreadable either way and this needs
      no agreement on field-name casing. The export keeps only what a forward pass needs -
      the policy head's weights and biases - and drops the value function, which exists to
      train the policy and is dead weight at inference.
      `MLPPolicy` in Go does that forward pass (a few matrix multiplications, no cgo; `INFERENCE.md`
      measures why the CPU is the right place for it, and where that stops holding). Compare
      against `ScriptedPolicy`; that comparison then becomes the pipeline's regression test, and
      it belongs in Go: 2000 episodes straight through `Env`, asserting success >=99% and mean
      steps/optimal <=1.10, is about 3 s at 229k steps/s. It supersedes `make baseline` as the
      guard because it needs no venv, no torch and no wire - nothing that can rot independently
      of the code it checks.
      The export runs once, after the checkpoint is saved. Step 16 will want a fresh one
      during a run so a spectator can watch training improve; that is the moment to add a
      periodic export, and it should ride on SB3's CheckpointCallback so the .zip and .pb
      cannot drift apart.
- [x] **15. In-server bot.** `bot.Spawn(world, policy)`: a `Client` with a `Send` buffer and no
      websocket, joined through `World.Join`. A goroutine drains `Send`, runs the Observer and
      policy, and calls `EnqueueMovement`. Needs a removal path (today removal lives in
      `ReadPump`'s defer) and `Client.Conn` must be optional.

### Watching training in Unity

Training envs run far faster than 20 Hz, so they can't be streamed live. Instead training leaves
a trail of snapshots, and a separate binary replays them afterwards as a recap of how the bot
learned.

- [x] **16. Snapshot export.** `train.py`'s `ExportSnapshots` callback writes
      `rl-training/snapshots/policy_<timesteps>.pb` (and `.zip`) about `EXPORTS_PER_RUN` (50)
      times per run, on rollout boundaries right after PPO updates the weights, and overwrites `policy.pb` each
      time. The number in the name orders the snapshots and says how far into training each one
      was taken.
- [x] **17. Spectator server.** `cmd/spectator` (`make spectator`, run from the repo root) is
      its own binary, not the game server: it steps `sim.Env` at 20 Hz, so an episode means
      what it means in training (spawn at the centre, a random goal, arrival or the step
      limit), and every episode gets a fresh world. `bot.SnapshotPolicy` plays the snapshots
      oldest first, the scripted policy if there are none. Every snapshot replays the same seeds
      (`Config.Seed` onwards, 3 by default), so the weights are the only thing that changes
      between them. Episodes are cut at `MaxTicks` (700), since an early snapshot would
      otherwise dither for the whole 1200. Playback starts when the first viewer connects;
      after the newest snapshot it sends `SpectatingOver` and hangs up. Settings are the
      `Config` constants, there are no flags; it logs to `backend/spectator.log`.
- [x] **18. The stream.** `Env.OnMessage` forwards the bot's own outgoing messages, so the bot
      is `players[0]` on the far end: the camera follows it and shows exactly its FOV. Each
      viewer is a `game.WebsocketClient` that never joins a world, so slow viewers drop frames
      like players do. A viewer joining mid-episode is replayed the `InitialGameState`,
      `Episode` and `PlaybackSpeed`. On a reset the new `InitialGameState` makes Unity rebuild
      the stations and discard its player views, so the bot reappears at spawn rather than
      gliding there.
- [x] **19. Spectator API and Unity.** Spectator-only messages live in
      `game/v1/spectator.proto` but ride the normal envelopes; the game server never sends them
      and ignores them if received:
      - `Episode` (goal, `snapshot_timesteps`, `final_timesteps`), once per episode: Unity draws
        the goal as a disc the size of `TradeRange` and shows `SNAPSHOT 1.23M / 1.99M (61%)`.
      - `PlaybackSpeed`, both ways: `,`/`.` ask for 0.25-32x, the spectator clamps it, retimes
        its ticker and tells every viewer the speed applied, since playback is shared. The phase
        timer counts down at that speed.
      - `SpectatingOver`: a RECAP OVER panel.

      Bots are marked with `PlayerState.is_bot`, which makes Unity draw them with the robot art
      and animate them from velocity even as `players[0]`.

Not done: watching a run while it trains, and per-step debug (last action, reward) on screen.

### Stage 2: trading

Trading is a **second environment**, not this one with trades bolted on. `Env` stays, because it
keeps three jobs: the pipeline's regression test, the steering half of the split below, and the
source of behaviour-cloning data. What is specific to reaching a point dies with it -
`optimalSteps`, `startDist`, `pickGoal`, the progress/penalty/arrival reward, `Terminated`
meaning "inside `TradeRange`", and the 1200-tick limit against a round that runs 6000. `World`,
`Observer`, `ScriptedPolicy` and the action table are shared, not copied.

- Observation grows to include `MarketState` quotes, `PlayerInventory` and `TradeReceipt`s
  (the fields already stubbed in `Observer`).
- Actions add buy and sell for each order size in `OrderSizes`. The 9 directions stay as a
  prefix, so `ActionCount` grows and nothing already trained is renumbered.
- Reward becomes the change in net worth.
- **`bot.Goal` inverts.** Today the env hands the bot a destination; a trading bot picks its
  own, so the goal becomes a policy *output* and `Encode(goal)` is a movement-shaped signature.
- A possible split: the policy picks a target station and a trade; scripted steering walks
  there.
- Behaviour cloning needs a scripted *trader* to imitate - `ScriptedPolicy` only steers. Worth
  writing early: it is both the pre-training source and the fixed reference a win rate is
  measured against.

#### Self-play

Several bots per world, so prices respond to other traders. The AMM pool is finite and profit
comes out of it and out of the other bots, so the game is near zero-sum: opponents genuinely
reshape each other's environment rather than just sharing a map.

The wire needs no change. The batch index can mean an **agent slot** rather than an env: each
bot already has its own `Client` and its own FOV-limited `Observer`, so the server maps slot `i`
to (world, bot), and agents sharing a world reach the round's end together - their
`final_observations` all fire on the same step, which is what PPO already expects.

Naive self-play forgets. Beating the current opponent by exploiting its particular weakness can
cost the general ability, and strategies cycle: A beats B beats C beats A. A league (AlphaStar)
exists to stop that, but OpenAI Five reached world-class Dota with plain self-play plus a slice
of past opponents, so **measure whether this game cycles before building for it**: snapshot the
policy periodically, play every snapshot against every other, and read the win-rate matrix.
Transitive, where later always beats earlier, means self-play is converging and a league buys
nothing. Cyclic, where generation 30 beats generation 50, means it earns its keep - and
prioritised opponent sampling, weighting towards opponents currently beating you, is the cheap
majority of one. Dedicated exploiter agents come last, if ever.

## Open questions

- **Latency fairness.** An in-server bot reacts with zero latency; humans have network
  round-trips. If that matters, delay the bot's actions by a few ticks, in the sim and in
  production alike.
- **Decision rate.** Should the bot decide every tick, or repeat each action for a few ticks
  (faster training, closer to human reaction time)?
- **Observation encoding.** `[dx, dy]` gets tiny near the goal. `[dx/dist, dy/dist, dist]`
  keeps the direction at full size; worth trying if the bot is imprecise when arriving.
- **Where the league lives.** Settled that Python trains, but not which side holds the
  opponent pool. Snapshots are PyTorch checkpoints, so they are naturally Python-side; match
  scheduling - which bot occupies which agent slot in which world - is the sim server's job.
  A `ResetRequest` that names an opponent policy per slot would put it in Go; keeping it in
  Python means the pool is just a sampling step before each `Reset`. Decide when self-play
  starts, not before.
