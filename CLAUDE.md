# Project conventions

## Commands
@Makefile

## Layout
- `backend/` — Go server and the protobuf schemas.
- `unity/` — Unity 6 desktop client.
- `web/` — browser client, plain JS, loads the `.proto` at runtime with protobufjs (no JS codegen)
- `docs/` — past and future design decision

## Wire protocol
Rules both the server and every client have to hold up:

- Never edit `backend/gen/` or `unity/Assets/Scripts/Generated/` by hand; edit `backend/api/proto/` and run `make proto`
- One `oneof` envelope per direction (`ClientMessage`, `ServerMessage`); add new messages as oneof members
- A client's own player must stay first in its `WorldSnapshot.players`; the Unity client identifies itself that way
- Trading: `TradeRequest` is `INTENT_BUY`/`INTENT_SELL` of whole `units` plus the per-unit `price_cents` the client saw; the whole order fills at one per-unit price or is rejected with a `TradeReceipt` reason

## Domain
- Cash is whole cents everywhere; only clients format it as dollars
- Commodity quantities are whole units everywhere, never fractional
- AMM rounding must always favour the pool (buys round up, sells round down, `k` never shrinks); see `docs/PRICING.md`

## Git Operations
- Keep commit messages brief - keep them around half a dozen sentences.