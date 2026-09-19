# Project conventions

## Commands
- Run: `make run` (server on :8080, serves `web/` as static files)
- Regenerate protobuf: `make proto` (regenerates both Go and C# output)
- Clean generated code: `make clean`
- Tests: `make test`

## Stack
- Go 1.26, gorilla/websocket, google.golang.org/protobuf
- Protobuf schemas in `api/proto/game/`, Go output in `gen/go/`
- Browser client in `web/`, plain JS, loads the `.proto` at runtime with protobufjs (no JS codegen)
- Unity client in `unity-client/` (Unity 6000.6.2f1, URP 2D template, new Input System only, Steam desktop target), C# proto output generated
  via `protoc --csharp_out`. Game code in `Assets/Scripts/{Networking,Client}`; server y is down, Unity world is `(x, -y)`.
  Player art is `Assets/Art/Player/{idle,walk}.png` (rows = 8 directions, columns = frames), see `DirectionalAnimationSet`.
  Commodity icons are `Assets/Art/Commodities/<name>.png`, mapped by the `CommodityIcons` asset; Game > Commodity Art > Create Missing Icons never overwrites real art.

## Rules
- Never edit `gen/` or `unity-client/Assets/Scripts/Generated/` by hand; edit `api/proto/` and run `make proto`
- One `oneof` envelope per direction (`ClientMessage`, `ServerMessage`); add new messages as oneof members
- Only `World.Run` mutates game state; `ReadPump`s only push onto queues, `WritePump`s only drain `Client.Send`
- All sends are non-blocking; drop the message when a buffer is full
- Each tick sends every client a `WorldSnapshot` with only the players inside its FOV radius, found through `SpatialGrid`
- A client's own player must stay first in its `WorldSnapshot.players`; the Unity client identifies itself that way
- Cash is whole cents everywhere (clients only format it as dollars); commodity quantities are whole units everywhere, never fractional. AMM rounding must always favour the pool (buys round up, sells round down, `k` never shrinks); see `docs/PRICING.md`
- Trading: `TradeRequest` is `INTENT_BUY`/`INTENT_SELL` of whole `units` plus the per-unit `price_cents` the client saw; the whole order fills at one per-unit price (`buyPrice(n)`/`sellPrice(n)`, which depend on order size) or is rejected with a `TradeReceipt` reason. Pool depth per commodity is `poolUnits` in `commodity.go`
- Each tick's `MarketState` quotes every size in `OrderSizes` (1, 2, 5, 10) as `PriceQuote.orders`; clients take their multipliers from those quotes, never hard-code them
