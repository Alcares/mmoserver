# Shared Constants: Where a Value Lives When Several Languages Need It

This records how a constant that Go, Python and Unity all depend on is kept in
one place, and which values are sent to clients instead of hardcoded in each.

## The question

Some values are already sent over the wire (`world_size` in `InitialGameState`, `obs_size` in the
sim's reset response). Others are copied by hand and have to be kept in sync. A root `.env`
read by every sub-project was considered and rejected:

- A Unity build in `unity/Builds/` can't read the repo at runtime, so the file would have to be
  baked in at build time, which makes it codegen with extra steps.
- A client built against one `.env` and a server started with another drift apart exactly as
  hardcoded copies do, only less visibly.
- `.env` is for deployment settings (ports, paths). Game rules don't belong next to them, and
  its values are untyped strings every consumer parses on its own.

## The rule

Sort a constant by who owns it:

1. **Game rules the server owns** go over the wire, in `InitialGameState`. The server is the
   single source, and a client always gets the values of the server it is talking to.
2. **Contracts between tools**, like the `policy_<timesteps>.pb` name shared by `train.py` and
   `backend/internal/bot/snapshot.go`, stay hardcoded, one per language, each with a comment
   naming the other side. There are too few to be worth tooling.
3. **Deliberate copies**, like the heading math in `rl_training/baseline.py` that checks Go
   independently, stay copies. Sharing them would defeat their purpose.

If category 2 grows to around a dozen values, generate them: one `constants.json` that
`make proto` turns into a Go, C# and Python file. Proto3 has no constants, so they can't live
in the `.proto` files themselves.

## Sent: trade range and collision radii

`InitialGameState` carries `trade_range`, `player_radius` and `station_radius` beside
`world_size`, filled from the constants in `backend/internal/game/config.go`. Nothing else
defines them:

| reader | uses |
|---|---|
| Unity `GameState.TradeRange` | the nearby-station check behind E/Q, and the spectator's goal disc |
| Unity `WorldView` | stations and players drawn at the collision radii |
| Go `bot.Observer` | `ScriptedPolicy` stops within `trade_range`; it can't import `game` |

The sim decides arrival with `game.TradeRange` directly, since it is server-side code.
`TestJoinSendsStationsBeforeSnapshots` checks the three values go out, and
`sim/env_test.go` checks the bot stops exactly where the env says it arrived, so a wrong value
on the wire shows up as a failing Go test rather than as trades the server rejects.

`MoveSpeed` and the tick rate aren't read by any client, so they stay server-only until one
needs them.
