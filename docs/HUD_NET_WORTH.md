# HUD net worth

**Status: planned for the next PR.**

## The problem

The HUD's PORTFOLIO and the final standings value holdings differently:

- **HUD** (`GameState.ValueOf`): units × the one-unit sell price.
- **Server** (`netWorth` in `phase.go`): units × `sellPrice(units)`, what selling the whole holding
  in one order would pay.

A player who buys heavily into a shallow pool pushes its price up, and the HUD marks every unit at
that inflated price. In one round a player saw a $2881 portfolio and finished at $1001: selling
back walks the price down the curve, and returns about what was paid.

The HUD rewards pumping, then the standings take it back, and the player can't tell why.

## The plan

- Add `net_worth_cents` to `PlayerInventory`, computed by the same `netWorth` the standings use,
  so there is one source of truth and no pricing maths in the client. Inventory already goes out on
  every trade and every second with cash decay.
- Show both values on the HUD:

  ```
  CASH        $1.25
  HOLDINGS    $2881.44   (at today's price)
  NET WORTH   $1001.57   (if sold now)
  ```

  The gap between them is the player's own price impact, which is worth learning to read.
- It leaks nothing: it's built from the player's own holdings and public pool state, and goes only
  to that player.
- When settlement auctions (B in `MARKET_REDESIGN.md`) land, only the server's calculation changes;
  the client keeps showing whatever it's sent.
