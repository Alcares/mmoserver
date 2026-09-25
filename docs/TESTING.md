# Tests the Bot Pipeline Still Needs

Everything listed here guards one kind of failure: code that keeps running and is quietly
wrong. A crash announces itself and needs no test to find it; a misaligned array or a
transposed matrix produces valid-looking numbers, trains a worse policy, and surfaces days
later as "the bot seems bad." That is the whole selection criterion below, and it is why the
list is short and why some obvious-looking tests are deliberately absent.

Current coverage, for contrast: `internal/game` (world, trading, pricing), `internal/bot`
(`Observer.Encode`, `ScriptedPolicy`, `Action.Valid`), `internal/sim` (`Env` lifecycle, reward,
the scripted walk in `TestEnvReachesGoal`). Untested today: **`sim.Server`**, **`bot.MLPPolicy`**,
and **all of `rl-training/`**, plus the client-side copies of shared constants (section 5).

---

## 1. `sim.Server` - `backend/internal/sim/service_test.go`

The gRPC service has no tests at all. It is also where every silent failure in the system
lives, because it is the only place that reorders, pairs and restarts things. No socket is
needed: `Reset` and `Step` are ordinary methods on `*Server`.

### 1.1 Autoreset puts each observation in the right field

Drive an env to termination, then assert `final_observations[i]` holds the observation the
episode *ended* on and `observations[i]` holds the *next* episode's first.

**Why.** Swap them and nothing breaks. Both are five valid floats, the wire format is
identical, no error is returned. PPO then bootstraps a truncated episode's value from the wrong
state, which biases every advantage estimate that episode contributes.

**Why nothing else finds it.** `env_test.go` never constructs a `Server`. More importantly,
`make baseline` cannot find it either: `ScriptedPolicy` reads `GoalDist` and ignores rewards
and values entirely, so the bridge check would still print 1.05 with this bug present. The only
existing signal is the final quality of a trained policy, which is the most expensive possible
place to learn about it.

### 1.2 The six parallel arrays stay index-aligned

Arrange a batch where envs finish on *different* steps - one terminating, one truncating, one
still running - and assert `observations`, `rewards`, `terminated`, `truncated`,
`final_observations` and `optimal_steps` at index `i` all describe env `i`.

**Why.** `Step` fills six slices in one loop. Any off-by-one pairs env 0's observation with env
1's reward, and PPO learns from noise. Every slice has the same length and type, so nothing
detects the mismatch.

**Why the mixed batch matters.** A batch where all envs finish together, or none do, exercises
neither branch boundary. The bug lives in the `continue` path that skips the autoreset block.

Assert also that `optimal_steps[i]` is **zero for unfinished envs**. Python only reads it when
the episode ended, so a leak into a running env's slot would be divided into an episode length
downstream - the one place this class of bug becomes a visible `ZeroDivisionError` rather than
bad training.

### 1.3 Seed streams are per env and stay independent

`Reset` with consecutive seeds - `[0, 1, 2, 3]`, which is what Python sends - then force
autoresets and assert env `i`'s second episode does not replay env `i+1`'s first.

**Why.** `Step` draws the next seed from `s.seedStreams[i]`. Written as `seed + episode`
instead, env 0's second episode becomes env 1's first, and the batch stops reducing variance
because the envs are correlated. `BOT_TRAINING.md` step 12 calls this out as one of two things
that "fail silently rather than erroring."

Pair it with determinism: the same seeds twice must give byte-identical first observations.

### 1.4 `Reset` reports the constants the network is built from

Assert `obs_size == bot.ObsSize` and `action_count == bot.ActionCount`.

**Why.** `env.py` builds `observation_space` and `action_space` from these two numbers. A wrong
`obs_size` would at least produce a shape error. A wrong *smaller* `action_count` would not: SB3
would build a policy head that is physically unable to emit the last action, and training would
converge happily to a bot that never uses one of its directions.

### 1.5 Error paths return the codes they claim

`Step` before `Reset` is `FailedPrecondition`; a mismatched action count and an out-of-range
action are `InvalidArgument`.

**Why, honestly: lower value than the four above.** These fail loudly either way. The test buys
diagnosis, not detection - a wrong code sends whoever hits it to debug the server instead of
their client. Write it last; it is three cheap table rows once the fixture exists.

### 1.6 Concurrent calls are safe

Hammer `Step` from several goroutines under `-race`.

**Why.** gRPC serves each call on its own goroutine, so concurrency is the server's problem,
not the client's. `s.mu` covers it today. This pins that, cheaply, against a future change that
moves work outside the lock.

---

## 2. `bot.MLPPolicy` - `backend/internal/bot/mlp_test.go`

Written, verified once by hand, guarded by nothing.

### 2.1 Golden forward pass

Fixed observation vectors in, expected action indices out, generated once from Python's
`predict(deterministic=True)` and committed.

**Why.** This is the single highest-value test in this document. It is the only thing that
separates *arithmetic* from *policy quality*. A transposed weight matrix on the square 64x64
layer passes every dimension check, loads without complaint, and yields a bot that still
wanders roughly goalward - the episode-level test would report ~1.4x and be misread as "needs
more training." The same is true of a wrong activation, an argmax over the wrong slice, or a
layer chained one position off.

**Write it before the episode test.** Once the arithmetic is pinned, a bad episode number means
a bad policy; until then it means either, and you cannot tell which.

**Practical note.** `Observation`'s fields are unexported, so this must live in `package bot`.
Set `worldSize: 1` and `Vectorise` becomes the identity, so the committed vectors are exactly
what Python saw. This has been run: 0 mismatches over 4000 cases.

### 2.2 Validation rejects every malformed policy

A table: transposed weights, stale `obs_size`, stale `action_count`, a broken layer chain, a
wrong bias width, an unsupported activation, no layers.

**Why.** Those checks in `NewMLPPolicy` are the only barrier between a stale `policy.pb` and a
silently wrong bot, and they are exactly the kind of code a later reader trims as defensive
noise. A test that asserts each one *fires* is what keeps them. Verified once already, producing
e.g. `layer 1 has 100 weights, want 64*64 - exported transposed?`.

### 2.3 Concurrency

`var _ Policy = (*MLPPolicy)(nil)` costs nothing. Then call `Act` from many goroutines under
`-race`.

**Why.** `Act` allocates its own scratch, and the doc comment explicitly invites replacing that
with a shared buffer if a profile ever complains. That change would be correct-looking and
race-prone. This test is what makes it fail loudly instead.

---

## 3. Pipeline regression - `backend/internal/sim/`

### 3.1 The trained policy still performs

Load `policy.pb`, run ~2000 episodes through `Env` under `MLPPolicy`, assert success >= 99% and
mean `steps/optimal` <= 1.10. About 3 s at 229k steps/s.

**Why.** It covers the whole chain end to end - observation encoding, env dynamics, export,
load, forward pass - with no venv, no torch and no socket, so nothing in it can rot
independently of the code it checks. It supersedes `make baseline` as the guard.

**Its weakness, stated plainly.** It is a verdict, not a diagnosis. When it fails it says
"worse than 1.10" and nothing about which of six layers moved. That is why 2.1 comes first.

Skip it when `policy.pb` is absent rather than failing, so a fresh clone without a trained
policy still runs green.

---

## 4. Python - `rl-training/tests/`

There are no Python tests. `env.py` is 165 lines that every future training run passes through.

### 4.1 The `VecEnv` mapping, against a stub

With a fake stub in place of the real server, construct a single step where one env terminates,
one truncates, and one keeps running. Assert `final_observations[i]` lands in
`infos[i]["terminal_observation"]`; `dones == terminated | truncated`;
`infos[i]["TimeLimit.truncated"]` appears only when truncated and *not* terminated;
`info["episode"]["l"]` counts steps since **that env's** last reset; and that the per-env
counters reset individually rather than globally.

**Why.** These are the silent bridge failures. Every one leaves shapes correct and training
running, and degrades the result instead of raising. `TimeLimit.truncated` in particular
changes whether PPO bootstraps - getting it backwards is invisible except in final quality.

**Why a stub rather than the real server.** The interesting case is a *mixed* batch on a single
step. Waiting for that to occur by chance makes the test slow and flaky; constructing it makes
it instant and exhaustive.

### 4.2 `export.py` finds every layer

Assert `_layer_prefixes` returns indices `0, 2` for a two-hidden-layer net and `0, 2, 4` for
three.

**Why this one is nastier than it looks.** `policy_net` numbers entries by position in the
`Sequential`, with a `Tanh` between each pair. Hardcoding `0` and `2` and then widening
`net_arch` to three hidden layers would export a *different, smaller network* than the one
trained - and Go would accept it, because `[5->64], [64->64], [64->9]` is a perfectly
self-consistent chain. Nothing in `NewMLPPolicy` can detect a missing middle layer. Only 2.1
catches it afterwards; this catches it at the source.

### 4.3 Export round-trip

`build(model)` -> serialise -> parse -> numpy forward -> compare against `model.predict`.

**Why.** Catches reshape order (C versus Fortran), float32 truncation, and fields assigned to
the wrong layer. Verified once by hand: 3000/3000. Worth keeping, because it fails at the
export step rather than in Go, which is a much shorter path to the cause.

---

## 5. Shared constants - the copies no test spans

`TradeRange` exists four times, in three languages:

| file | |
|---|---|
| `backend/internal/game/config.go:20` | the authority |
| `backend/internal/bot/policy.go:16` | `stopRange`, because `internal/bot` must not import `internal/game` |
| `unity/Assets/Scripts/Client/GameState.cs:19` | gates the nearby-station highlight and E/Q |
| `web/index.html:427` | the same, in the browser client |

This is not hypothetical. When the range moved from 5 to 3, the first two were updated and the
last two were not, and both clients spent that window offering trades from 3-5 units out that
the server then rejected.

**What already catches half of it.** `sim/env_test.go:47` asserts that a state the env calls
arrived is a state where `ScriptedPolicy` returns `ActionStop`. That is a real two-sided check
and it fired on the 5-to-3 change. It covers the Go pair only.

**What catches the other half: nothing, and probably nothing should.** A Go test cannot read a
C# `const` or a JS `const` without scraping source, which is a test that asserts two spellings
of a number match - the `optimalSteps` mistake again, in a worse form.

The fix is to delete the copies rather than test them. `InitialGameState` already carries
`world_size` for exactly this reason, and `unity/CLAUDE.md` already says order-size multipliers
come from `PriceQuote.orders` and are "never hard-coded". A `trade_range` field alongside
`world_size` would put both clients on the server's number and leave one constant to keep in
step instead of three.

Until then, the two client comments are also wrong about where the authority lives: both say
`world.go`, and it is `config.go`.

---

## Suggested order

By "silent and expensive" first, not by convenience:

1. **1.1, 1.2** - autoreset and alignment. Silent, and they corrupt training itself.
2. **2.1** - the golden forward pass. Turns every later failure into a diagnosis.
3. **4.1** - the wrapper mapping. Same class as 1.1, on the other side of the wire.
4. **1.3** - seed independence.
5. **2.2, 4.2, 4.3** - the checks that keep the other checks honest.
6. **3.1** - the end-to-end guard, once the pieces beneath it are pinned.
7. **1.4, 1.5, 1.6, 2.3** - cheap rows once each fixture exists.

Section 5 is not in this list on purpose: it asks for a proto field, not a test.

## What not to write

- **Proto serialisation.** Generated code; testing it tests `protoc`.
- **That a step moves 0.6 units.** `TestMovementDistanceTraveled` and `TestEnvReachesGoal`
  already cover it, from the game side and the env side.
- **That `ScriptedPolicy` picks the nearest direction.** `TestScriptedPolicyDirections` and
  `TestScriptedPolicySectorBoundaries` cover it.
- **A Python reimplementation of the reward.** It would duplicate `env.go` and assert the two
  copies agree, which is the `optimalSteps` mistake with more steps.
