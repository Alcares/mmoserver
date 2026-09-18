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

**Spot price** is simply the ratio of the two reserves:

```
price = cashReserve / unitReserve
```

There's no separate "update the price" step. Price is never stored as its own
number that something has to remember to change — it's a computed read of whatever
the reserves currently are. This means it's structurally impossible for price and
reserves to drift out of sync with each other.

## Buying

When a player spends `cashIn` to buy units, the trade adds `cashIn` to `cashReserve`
and removes enough from `unitReserve` to keep `k` constant:

```
newCashReserve = cashReserve + cashIn
newUnitReserve = k / newCashReserve
unitsOut       = unitReserve - newUnitReserve
```

Because `k` is fixed, `newUnitReserve` is always smaller than `unitReserve` for any
positive `cashIn` — you always get a positive number of units, and the pool never
needs a manual "are we out of stock" check the way a flat-price model does. If a
station's reserve is small, the same `cashIn` yields more units per dollar than a
deep, heavily-bought pool would — but because `newUnitReserve` shrinks in the
denominator, taking a large share of a shallow pool moves price sharply. This
"slippage" curve is what makes buying more of something progressively more
expensive within a single trade, on top of raising the *next* trade's starting
price.

If `cashIn` is too small to move `unitsOut` above zero (i.e., it rounds down to
nothing at the current price), `buy()` returns `0` and leaves the pool completely
untouched — no partial trade, no cash silently vanishing into the pool for
nothing.

## Selling

Selling `unitsIn` back into the pool is the mirror image:

```
newUnitReserve = unitReserve + unitsIn
newCashReserve = k / newUnitReserve
cashOut        = cashReserve - newCashReserve
```

The math is implemented (`CommodityState.sell`) but not yet wired into any trade
intent — `INTENT_SELL_RATIO` and `INTENT_DUMP_ALL` are still TODOs in
`World.Run`. The pool model already supports it; it just needs a call site.

## Worked example

Starting state for a fresh commodity: `unitReserve = 100`, `cashReserve = 1000`
(so `k = 100,000`, initial price = `1000 / 100 = 10`).

A player buys with `cashIn = 50`:

```
newCashReserve = 1000 + 50 = 1050
newUnitReserve = 100,000 / 1050 = 95 (floored)
unitsOut       = 100 - 95 = 5 units
new price      = 1050 / 95 ≈ 11.05
```

Confirmed against the running server: this exact trade produced `price: 10 → 11`
(rounded for the wire), `delta_basis_points: +1052` (`(11.05 - 10) / 10 * 10000`),
and pool depth `100 → 95`. A second identical trade pushed price to `12` and depth
to `90` — each successive buy costs more than the last, exactly as the curve
predicts.

## What gets broadcast

Every tick, `World.Run` builds one `PriceQuote` per commodity and broadcasts the
same `MarketState` message to every connected client (not just whoever just
traded — price is global, so everyone needs to see it move):

- `spot_price_cents` — `cashReserve / unitReserve`, truncated to an integer for now
- `delta_basis_points` — the percent change vs. the price at the end of the
  *previous* tick (`CommodityState.lastPrice`), in basis points (1% = 100)
- `available_pool_units` — the current `unitReserve`, i.e. how much liquidity is
  left before buying gets punishingly expensive

## The one tuning knob: pool depth

`initialUnitReserve` (currently `100`) is the only number that controls game feel.
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
