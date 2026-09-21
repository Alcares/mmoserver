# Commodity Pricing: Constant-Product AMM

This describes how commodity prices move in the trading floor game, implemented in
`../backend/internal/game/commodity.go` and wired into trade execution in `../backend/internal/game/trade.go`.

## Why not a flat price

Before this, every commodity had one fixed price (`CommodityPrice = 10`) that never
changed no matter how much anyone bought. That made trading pointless — there was
nothing to react to, no reason to hurry, and no way for one player's activity to
affect another's. The whole point of a trading floor is that prices move because
people trade, and that movement is visible to everyone, in real time. That requires
an actual pricing algorithm, not a constant.

## The algorithm: constant-product AMM

Each commodity has its own liquidity pool with two reserves instead of a price field:

- `cashReserve` — a virtual amount of cash sitting in the pool
- `unitReserve` — a virtual amount of the commodity sitting in the pool

These are tied together by one invariant, the "constant product":

```
cashReserve * unitReserve = k
```

`k` never changes during a trade — only the two reserves move, always in opposite
directions, always keeping their product at least `k`. This is the same mechanism
Uniswap and similar decentralized exchanges use, minus the parts that don't apply
here (no LP tokens, no external liquidity providers — the "pool" is just an internal
bookkeeping device, not real deposited funds).

The **marginal price** is the ratio of the two reserves, `cashReserve / unitReserve`.
It is only the price of an infinitesimal sliver, so it is not what players are shown
(see "Order prices" below), but it explains why the pool behaves as it does.

There's no separate "update the price" step. Price is never stored as its own
number that something has to remember to change — it's a computed read of whatever
the reserves currently are. This means it's structurally impossible for price and
reserves to drift out of sync with each other.

## Cash is cents, quantities are whole units

Cash is whole **cents** everywhere: balances, prices, orders and the pool's cash reserve
(`$10.11` is `1011`). Clients only format it as dollars for display.

Commodity quantities are whole **units** everywhere: the pool, inventories and orders.
Nobody ever holds a fraction of a unit.

## Order prices

A player buys or sells a whole number of units, and **every unit in an order trades at
the same price**, so the order's total is exactly `units * price`. That per-unit price
depends on the order size, because a bigger order moves the pool further:

```
buy cost of N   = ceil(k / (unitReserve - N)) - cashReserve
buy price       = ceil(buy cost of N / N)

sell payout of N = cashReserve - ceil(k / (unitReserve + N))
sell price       = floor(sell payout of N / N)
```

Buying rounds up and selling rounds down, so rounding always stays in the pool's favour:
`k` may grow slightly but never shrinks. The pool always keeps at least one unit, so a
buy of `N >= unitReserve` has no price (quoted as 0) and is rejected.

For the same size, the sell price is below the buy price; the spread is what the pool
keeps. Bigger orders buy higher and sell lower per unit, but buying 10 at once costs
about the same as buying one unit ten times in a row — bulk isn't punished, it just
can't dodge the price move it causes.

### Why the price depends on order size

Charging a whole order the current one-unit price would be exploitable: the order still
moves the pool, so the price jumps as soon as it fills. Buying 10 at $10.11 in a
100-unit pool and immediately selling them back at the moved price (about $12.10) would
make ~$20 per round trip, forever. Pricing each order from its own cost on the curve
makes any buy followed by a sell lose money.

## Pool depth: the tuning knob

Each commodity's starting depth is set in `poolUnits` (all start at $10.00 per unit,
`initialPriceCents`). Depth is the one number that controls how a market feels:

| Commodity | Pool units | Buy 1 | Buy 10 (each) | 1-unit price after buying 10 |
|-----------|-----------:|------:|--------------:|-----------------------------:|
| Oil       |       2000 | $10.01 |       $10.06 | $10.11 |
| Wheat     |       1000 | $10.02 |       $10.11 | $10.22 |
| Coffee    |        500 | $10.03 |       $10.21 | $10.44 |
| Gold      |        400 | $10.03 |       $10.26 | $10.55 |
| Silver    |        250 | $10.05 |       $10.42 | $10.90 |
| Lithium   |         60 | $10.17 |       $12.00 | $14.70 |

- **Shallow pools** (lithium) mean a single trade swings price a lot. Feels chaotic and
  exciting, but a couple of players can dominate a market fast.
- **Deep pools** (oil) mean price barely moves per trade. Feels stable and fair, but
  can also feel dead — nothing to react to.

Tune this by playtesting, not by math. A commodity missing from `poolUnits` gets
`defaultPoolUnits`.

## Trading

The client sends a `TradeRequest` with `INTENT_BUY` or `INTENT_SELL`, a whole number
of `units`, and `price_cents`: the per-unit price it saw for that order size. The
server trades against the station the player is standing at, and fills the whole
order or none of it. It is rejected, with nothing changed, when:

- the player is not within `TradeRange` of a station
- `units` is 0 or the intent is unknown
- **buy:** the pool can't sell that many, the price rose above `price_cents`, or the
  player can't afford `units * price`
- **sell:** the player holds fewer than `units`, each unit is worth under a cent, or
  the price fell below `price_cents`

A better price than the one sent fills at the better price. Every request is answered
with a `TradeReceipt` (execution price, total and new balances, or a `TradeRejection`
reason), followed by a fresh `PlayerInventory` on success.

## Worked example

Starting state: `unitReserve` = 100 units, `cashReserve` = `100000` cents ($1000.00),
so `k` = 10,000,000 and the marginal price is $10.00.

Buying 1 unit:

```
cost  = ceil(10000000 / 99) - 100000 = 101011 - 100000 = 1011 cents
price = 1011 cents ($10.11)
```

Buying 10 units:

```
cost  = ceil(10000000 / 90) - 100000 = 111112 - 100000 = 11112 cents
price = ceil(11112 / 10) = 1112 cents ($11.12 each, $111.20 total)
```

The pool is now 90 units and 111120 cents. Selling those 10 straight back:

```
payout = 111120 - ceil(10000800 / 100) = 111120 - 100008 = 11112 cents
price  = floor(11112 / 10) = 1111 cents ($11.11 each, $111.10 total)
```

The round trip loses $0.10. These cases are pinned by tests in
`backend/internal/game/commodity_test.go`, alongside a randomized check that no sequence of
buys and sells ever ends up worth more than it cost.

## What gets broadcast

Every tick, `World.Run` builds one `PriceQuote` per commodity and broadcasts the
same `MarketState` message to every connected client (not just whoever just
traded — price is global, so everyone needs to see it move):

- `orders` — one `OrderQuote` per size in `OrderSizes` (1, 2, 5 and 10 units), each
  with the per-unit `buy_price_cents` and `sell_price_cents` for an order that big.
  Clients offer exactly these sizes as their multipliers and show the quote for the
  one selected.
- `delta_basis_points` — the percent change of the one-unit buy price vs. the end of
  the *previous* tick (`CommodityState.lastPrice`), in basis points (1% = 100)
- `available_pool_units` — the current `unitReserve`, i.e. how much liquidity is left
  before buying gets punishingly expensive

## Design notes / things this intentionally doesn't do (yet)

- **One global pool per commodity, not per station.** Every `TradingStation` of a
  given commodity type reads from and trades against the same
  `World.Commodities[type]` pool. Right now there's exactly one station per
  commodity, so this is invisible, but if multiple stations ever sell the same
  commodity, they'd share one price rather than have independent regional
  markets. That's a deliberate simplification, not an oversight — splitting it
  into per-station pools is a future fork point if regional price divergence
  ever becomes a design goal.
- **No mean reversion or ambient drift.** Price only moves when someone trades;
  it never drifts back toward a baseline or wiggles on its own between trades.
  With a small number of concurrent players this can mean the market goes quiet
  for stretches. If that turns out to hurt game feel, the fix is additive (a
  small per-tick random walk layered on top of the AMM price) rather than a
  rework of the pricing model itself.
- **No slippage tolerance.** An order fills only at the price the client saw or
  better. In a busy market that can reject orders often; the fix is a client-side
  tolerance (sending a slightly worse `price_cents`), with no server change.
- **`dominant_whale_id` is still unset (always 0).** The proto has a field for
  showing who's cornering a market; nothing populates it yet.
