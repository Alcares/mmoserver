# Shared Constants: Where a Value Lives When Several Languages Need It

This records how a constant that Go, Python, Unity and the web client all depend on is kept in
one place, and plans the one change that follows from it: sending `TradeRange` to clients
instead of hardcoding it in each.

## The question

Some values are already sent over the wire (`world_size` in `InitialGameState`, `obs_size` in the
sim's reset response). Others are copied by hand and have to be kept in sync. A root `.env`
read by every sub-project was considered and rejected:

- A Unity build in `unity/Builds/` and the browser client can't read the repo at runtime, so
  the file would have to be baked in at build time, which makes it codegen with extra steps.
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

## The change: send `TradeRange`

`TradeRange` is the only category 1 value clients currently copy. `MoveSpeed` and the tick rate
aren't read by either client, so they stay server-only until one needs them.

Current copies:

- `unity/Assets/Scripts/Client/GameState.cs:19` - `public const float TradeRange = 4f;`, used by
  `TryGetNearbyStation`.
- `web/index.html:427` - `const TRADE_RANGE = 4;`, used by the nearest-station lookup.

Steps:

1. **Proto.** Add `float trade_range = 4;` to `InitialGameState` in
   `backend/api/proto/game/v1/server_message.proto`, then run `make proto`.
2. **Server.** Set `TradeRange: TradeRange` where `backend/internal/game/session.go:55` builds
   the `InitialGameState`. Add a test that the initial state carries it.
3. **Unity.** Replace the `const` in `GameState` with a field set from `InitialGameState`.
   Until it arrives, or if it is `0`, `TryGetNearbyStation` finds nothing, so a client never
   offers a trade the server would reject.
4. **Web.** Same in `web/index.html`: read `message.initialState.tradeRange` beside
   `worldSize`, and replace `TRADE_RANGE` with it.
5. **Check.** Run `make test`, build the Unity client, and trade at a station from both clients.

The bot and the sim keep using `game.TradeRange` directly: they're in the same Go module as
the server, so they already share the one definition.
