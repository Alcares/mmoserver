# Commodity Pricing: Constant-Product AMM

This describes how commodity prices move in the trading floor game, implemented in
`../internal/game/commodity.go` and wired into trade execution in `internal/game/world.go`.

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
directions, always keeping their product equal to `k`. This is the same mechanism
Uniswap and similar decentralized exchanges use, minus the parts that don't apply
here (no LP tokens, no external liquidity providers — the "pool" is just an internal
bookkeeping device, not real deposited funds).

The **marginal price** is the ratio of the two reserves, `cashReserve / unitReserve`.
It is only the price of an infinitesimal sliver, so it is not what players are shown
(see "The price" below), but it explains why the pool behaves as it does.

There's no separate "update the price" step. Price is never stored as its own
number that something has to remember to change — it's a computed read of whatever
the reserves currently are. This means it's structurally impossible for price and
reserves to drift out of sync with each other.

## Cash is cents, quantities are micro-units

Cash is whole **cents** everywhere: balances, prices, order amounts and the pool's
cash reserve (`$10.11` is `1011`). Commodity quantities are fixed-point
**micro-units** (`UnitScale` = 1,000,000 per unit) in the pool and in inventories, so
holdings are fractional (1.0009, 14.55 units).

The smallest order the pool fills is **one whole unit** (`MinBuyUnits`). Any order
worth less than that is rejected with no charge and the pool untouched. Orders above
the minimum are filled at whatever the pool's price is *when the order executes*, not
when it was placed.

## Buying

When a player spends `cashIn` to buy units, the trade adds `cashIn` to `cashReserve`
and removes enough from `unitReserve` to keep `k` constant:

```
newCashReserve = cashReserve + cashIn
newUnitReserve = ceil(k / newCashReserve)
unitsOut       = unitReserve - newUnitReserve        (micro-units)
```

`newUnitReserve` rounds **up**, so `unitsOut` rounds down: rounding always stays in
the pool's favour. Rounding the other way lets a trader buy a fractional unit as a
whole one and then hold it at full market value. `k` may grow slightly from rounding
but must never shrink. If `unitsOut` is under the one-unit minimum, the order is
rejected.

Because `k` is fixed, buying more of something is progressively more expensive within
a single trade ("slippage"), on top of raising the *next* trade's starting price.

## The price

There is one price per commodity, `price_cents`: **the exact cost of one whole unit
right now**, rounded up to the cent.

```
price = ceil(k / (unitReserve - MinBuyUnits)) - cashReserve
```

It is slightly above the marginal price because taking a unit out of the pool moves
the price while you buy it. This is the number on the station label, the amount the
client sends when you press E, and the price holdings are valued at, so the price you
see is the price you pay.

Sending exactly `price_cents` fills a hair over one unit (the cent rounding, about
1.0009 units at $10). If another trade pushed the price up before the order arrived,
the same cash no longer buys a whole unit and the order is rejected with no charge; if
the price fell, the same cash fills more.

The client only offers one-unit buys. The server accepts any larger order too; it
fills at the pool's average price for that size, so bulk costs more per unit than the
displayed price.

## Selling

Selling `unitsIn` micro-units back into the pool is the mirror image:

```
newUnitReserve = unitReserve + unitsIn
newCashReserve = ceil(k / newUnitReserve)
cashOut        = cashReserve - newCashReserve
```

Same rule as buying: the pool keeps the rounding, so `cashOut` is rounded down.

The math is implemented (`CommodityState.sell`) but not yet wired into any trade
intent — `INTENT_SELL_RATIO` and `INTENT_DUMP_ALL` are still TODOs in
`World.Run`. The pool model already supports it; it just needs a call site.

## Worked example

Starting state for a fresh commodity: `unitReserve` = 100 units, `cashReserve` =
`100000` cents ($1000.00), so the marginal price is $10.00.

The price (cost of one unit):

```
price = ceil(100000 * 100 / 99) - 100000 = ceil(101010.10) - 100000 = 1011 cents
```

A player sends exactly that, `cashIn = 1011`:

```
newUnitReserve = 100000 * 100 / 101011 = 98.999119 units (rounded up at micro-unit precision)
unitsOut       = 1.000881 units
new price      = $10.31
```

One cent less (`cashIn = 1010`) buys under one unit and is rejected. A bigger order,
`cashIn = 10000` ($100.00), buys 9.09 units. These cases are pinned by tests in
`internal/game/commodity_test.go`, alongside a randomized check that no buy, or buy
followed by sell, is ever worth more than it cost.

## What gets broadcast

Every tick, `World.Run` builds one `PriceQuote` per commodity and broadcasts the
same `MarketState` message to every connected client (not just whoever just
traded — price is global, so everyone needs to see it move):

- `price_cents` — the cost of one whole unit right now, in cents (see "The price")
- `delta_basis_points` — the percent change of that price vs. the end of the
  *previous* tick (`CommodityState.lastPrice`), in basis points (1% = 100)
- `available_pool_units` — the current `unitReserve` in whole units, i.e. how much liquidity is
  left before buying gets punishingly expensive

## The one tuning knob: pool depth

`initialUnits` (currently `100` whole units) is the only number that controls game feel.
It's not something to derive analytically — it's a balance decision:

- **Shallow pools** (small reserve) mean a single trade swings price a lot. Feels
  chaotic and exciting, but a couple of players can dominate a market fast.
- **Deep pools** (large reserve) mean price barely moves per trade. Feels stable
  and fair, but can also feel dead — nothing to react to.

Tune this by playtesting, not by math.

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
- **`dominant_whale_id` is still unset (always 0).** The proto has a field for
  showing who's cornering a market; nothing populates it yet.
