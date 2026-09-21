# Project conventions

## Commands
@Makefile

## Stack
- Protobuf schemas in `api/proto/game/`, Go output in `gen/go/`
- Browser client in `web/`, plain JS, loads the `.proto` at runtime with protobufjs (no JS codegen)

## Rules
- Never edit `gen/` or `unity-client/Assets/Scripts/Generated/` by hand; edit `api/proto/` and run `make proto`
- One `oneof` envelope per direction (`ClientMessage`, `ServerMessage`); add new messages as oneof members
- Only `World.Run` mutates game state; `ReadPump`s only push onto queues, `WritePump`s only drain `Client.Send`
- All sends are non-blocking; drop the message when a buffer is full
- Each tick sends every client a `WorldSnapshot` with only the players inside its FOV radius, found through `SpatialGrid`
- A client's own player must stay first in its `WorldSnapshot.players`; the Unity client identifies itself that way
- After editing the `unity-client` code, it may be necessary to recompile the client. Do that using the Unity MCP server.
- Cash is whole cents everywhere (clients only format it as dollars); commodity quantities are whole units everywhere, never fractional. AMM rounding must always favour the pool (buys round up, sells round down, `k` never shrinks); see `docs/PRICING.md`
- Trading: `TradeRequest` is `INTENT_BUY`/`INTENT_SELL` of whole `units` plus the per-unit `price_cents` the client saw; the whole order fills at one per-unit price (`buyPrice(n)`/`sellPrice(n)`, which depend on order size) or is rejected with a `TradeReceipt` reason. Pool depth per commodity is `poolUnits` in `commodity.go`
- Each tick's `MarketState` quotes every size in `OrderSizes` (1, 2, 5, 10) as `PriceQuote.orders`; clients take their multipliers from those quotes, never hard-code them
