# Market Redesign: Proposal, Third Edition

**Status: proposal. Nothing here is implemented.** This is a delta on `MARKET_REDESIGN_2.md`:
everything there stands unless a section below replaces or amends it. Read the second edition
first. As before, Stage 2 of `BOT_TRAINING.md` should not start until the validation gates pass.

## What changed from the second edition

A review of the second edition found four remaining problems, and one principle was restated:

1. **Reflexes were meant to be irrelevant; they only need to be secondary.** Reaction speed is an
   acceptable part of skill between humans, as long as trading decisions matter clearly more.
   "Close to zero" becomes a ratio (see "Principles" and "Validation").
2. **The imbalance bar made the last tick of a batch a free look.** Irrevocable orders stop
   spoofing, but not sniping: wait until the window's last tick, read the bar, then submit.
   A zero-latency bot always wins that.
3. **Signals started a race to the station.** Trading needs you at the station, so whoever
   happened to stand at silver when silver's signal appeared took the first batch. D doesn't
   help; the race is in the walking, not the clicking.
4. **E could crowd out the player-vs-player layer.** With one public signal, everyone has the
   same information about `V`. If the signal is good, "trade the signal, ignore everyone" wins,
   and the game becomes player vs puzzle. Public open interest made it worse: the crowd became a
   number, and who was in it stopped mattering.
5. **Too much at once for a 5-minute round,** against the second edition's own "Manageable".

The answers, one per problem:

- **Random batch close** (D): the clear lands on a random tick, so there is no known last tick.
- **Scheduled signal drops and opening batches** (D, E): signals arrive on a published
  schedule, and the first clear after one runs long enough for anyone to walk there.
- **Dispersed private signals** (E): every player gets their own noisy signal about `V`. Other
  players' trades become evidence about `V`, so reading them is how you learn the value.
- **Open interest at the station** (4): live only where you stand, lagged everywhere else.
- **Rotating commodities and unlocks**, and C and 4 made conditional on the harness.
- **A human-like reaction delay for bots** wherever speed still counts.

## Principles

Replaces the second edition's "Skill over reflexes, by construction":

- **Decisions over reflexes.** Reaction speed may count, but a better decision must be worth
  clearly more than a faster one. Measured, not argued: the reflex ratio in "Validation". Rules
  that shrink the reflex share are preferred to delays tuned per player.

The rest stand.

## D. Batch clearing: amendments

**Interval.** `clearInterval` defaults to 0.5 s (10 ticks), shorter than the second edition's
1 s. With reflexes allowed to count a little, the snappier feel is worth more than the last bit
of fairness. The knob's range stays 0.5–2 s.

**Random close.** Each batch clears at a tick drawn uniformly from a window around
`clearInterval`, say 0.3–0.7 s after the previous clear. The draw is secret until the clear
happens, and comes from the world's seeded RNG so harness runs stay reproducible. Waiting for the
last tick to read the bar is now a gamble on missing the batch, not a free look. Real exchanges
close auctions the same way.

- The countdown on the station shows the window ("clears in 0.3–0.7 s"), not a tick.
- An order that arrives after the draw fires goes into the next batch. Its acknowledgement says
  which batch it landed in.

**The imbalance bar is coarse by default:** direction only (more buys, more sells, balanced),
not size. Size stays a knob. A lagged bar (the previous batch's imbalance) is the fallback if
sniping still pays under random close.

**Opening batches.** The first batch in a commodity after its signal drop (see E) runs
`openingInterval`, about 8 s, roughly the walk from the centre out to the ring and back
towards a neighbour. Its close is random over its last second. Everyone who decides to act on
the signal can get there and gets the same price. It's the moment the game builds up to: players
converge on a station, the bar fills, and each decides whether to join the crowd or fade it.

- Orders in an opening batch are irrevocable, like any batch.
- Margin calls in that commodity wait for the opening clear, so a squeeze can't fire in the
  middle of the walk.
- Other commodities keep clearing on their normal interval during it.

**Protocol.** The pending-order acknowledgement carries the batch it landed in. `PriceQuote`'s
time to the next clear becomes the window's earliest and latest time, and flags an opening
batch.

## E. Settlement value: amendments

**Signal drops.** Each commodity's cycle has a published signal time, announced with the
settlement schedule, a fixed offset before the settlement it's about. At the signal time, all
signals for that commodity arrive at once, and an opening batch starts. Being at the right station
is now planning, not luck: you know where the next signal lands, and walking there early is a
decision with a cost (you aren't somewhere else).

**Dispersed private signals.** At each signal drop, every player gets their own signal about
`V`: `V` plus independent noise of spread `σ`, rounded to whole cents. Everyone's signal is equally
good and costs nothing. One signal is weak; the average of all of them is close to `V`.

- **Why this fixes the PvP problem.** A player's trades leak their signal. When someone buys
  silver after the drop, it's evidence that their signal was high. Trade flashes in field of view,
  open interest and the imbalance bar all become ways to learn what the others know. Reading other
  players stops competing with reading `V`; it becomes the best way to read `V`. This is the
  classic informed-trading setup (Grossman & Stiglitz, 1980; Kyle, 1985).
- **Why it isn't luck.** The second edition rejected random tips because some players got them
  and others didn't. Here everyone gets the same quality at the same time. Luck is in the noise
  draw, which several settlements per round average out.
- **Bluffing.** Trading against your own signal to mislead the others costs you the gap between
  your trade and your best guess at `V`. That makes a bluff a real bet, answered by reading.
- **In the UI.** Shown as a read, not a number to four figures: "your read: silver settles
  around $9.40". The round summary reveals every player's signals, `V`, and who traded which way,
  so a player can see what the crowd knew and what they missed.

**Knobs.**
- **`σ`, relative to `V`'s step.** Large `σ` makes your own signal nearly useless and reading the
  others everything; small `σ` makes your own signal enough, and the PvP layer thins out.
- **`σ` with player count.** A fixed `σ` makes the crowd's average sharper in bigger worlds.
  Scaling `σ` with the square root of the player count keeps the average equally good at every
  world size, at the cost of weaker individual signals in big worlds.
- **The public signal.** Now optional. Default: kept, but vague (direction only), so a player who
  ignores everything still has something. Off entirely is a knob.

**`V`'s step size.** Sized to about the price impact of a typical crowd in that pool, not larger.
Then crowding and value matter about equally, and neither dominates. The harness finds the
number; the second edition's "too small, E doesn't matter; too large, a lottery" still applies.

**Protocol.** One new per-player `ServerMessage` member carries the private signal. The signal
schedule rides on the settlement-scheduled message.

## 2. Bought tips: amendments

A value tip is now a sharper private signal: one more draw about `V`, with less noise than the
free one. Its value is relative to what the crowd's trades already reveal, which makes the price
decision harder and more interesting.

New tip kind, a candidate:

| Tip | Example | How player vs player |
|---|---|---|
| **A read** | "Player X's read on silver was high" | Fully: it tells you whether X's trades are information or a bluff. |

## 4. Stations spread apart: amendments

**Open interest is live only at the station.** Players at a station see its open interest
update with every clear. Everyone else sees a copy lagged by `oiLag`, say 10 s. This replaces the
second edition's "open interest is public". Being there matters again, and the crowd is people you
can see rather than a number on a board.

**The mid-round net-worth mark uses the lagged open interest.** The second edition marked at what
a settlement now would pay, which is safe only if open interest is public. With live open interest
at the station only, a live mark would leak it again. Marking with the lagged copy leaks nothing
new. The bot's reward still uses the true positions, as before.

**What's public, updated.** Prices, the settlement and signal schedules, lagged open interest,
the public signal, and the fact that someone bought a tip. **What's private.** Who holds what,
live open interest away from the station, your own signal, the contents of tips, and anything
outside your field of view.

## Managing the load

Amends the second edition's "Active commodities" knob and its C and 4.

- **Rotating active commodities, on by default.** Four of six live each round, chosen at round
  start and shown with the schedule.
- **Unlocks.** New players play long-only: A, B + 1, D, E. Short selling (C) unlocks after a number
  of ranked rounds. Tips (2) can follow the same path. It also softens the second edition's open
  question of newcomers funding veterans: nobody meets a squeeze in their first rounds.
- **C and 4 are conditional.** Each goes in only if the harness shows what it's for is missing:
  C if a lone holder or a pumped price still can't be punished, 4 if reading other players is
  still worth too little with dispersed signals. Dispersed signals may already give players enough
  to read.

## Training the trading bot: amendments

**Latency parity.** Replaces the second edition's paragraph. Where speed still counts (inside a
batch window, walking to an opening batch, reacting to a margin cascade), the bot acts with a
human-like reaction delay, 250–400 ms (5–8 ticks), the same in the sim and in production.
Otherwise the reflex share of skill becomes the bot's share. Random close makes the delay matter
less, but it stays from the first run.

**Observation additions.**
- Own private signal per commodity, and the time to each commodity's next signal drop.
- Live open interest where the bot stands, lagged elsewhere.
- Whether the current batch is an opening batch, and its window.

**Curriculum, stage 1,** becomes: A, B + 1, D, E with dispersed signals, rotating commodities, no
tips, no events. C and 4 join if the harness keeps them.

## Validation: amendments

**New strategies.**
- **Own-signal follower:** trades its own signal and ignores everyone else.
- **Signal reader:** its own signal plus what it infers from open interest, the bar and trades in
  view. The strategy the game should reward most.
- **Pre-positioner:** walks to the next signal drop's station early.
- **Last-tick sniper:** submits as late in the window as it can, after reading the bar.

The second edition's signal follower becomes the public-signal follower.

**Metrics.** The second edition's reflex sensitivity is kept as a measurement and turned into a
ratio:
- **Reflex value:** each strategy against a copy of itself delayed by 5 ticks.
- **Decision value:** the signal reader against the random trader, at the same latency.
- **Reflex ratio:** reflex value over decision value.

**Gates.** Replacing the second edition's first-out exit racer gate:
- The reflex ratio is under a quarter, in every world size.
- The last-tick sniper gains less than the seed variance over a copy that submits at a random
  moment in the window.

Added:
- The signal reader beats the own-signal follower by more than the seed variance. If it doesn't,
  reading other players is worthless and `σ` is too small.
- The own-signal follower beats the random trader. If it doesn't, `σ` is too large and signals
  are noise.
- The pre-positioner does better than a copy that walks only after the drop, but by less than the
  signal reader beats the own-signal follower. Being there early should help, not decide.

## Playtests: amendments

Added questions:
- **Did you use what others were doing to guess the value?** If players never do, dispersed
  signals aren't legible yet.
- **Did the opening batches feel like a moment, or like waiting?**

## Rollout

Replaces the second edition's steps 2 and 5, and adds one:

2. **D, batch clearing,** with random close and the coarse bar. Client: pending orders, the
   window countdown, the fill animation.
5. **1 + E,** with the signal schedule, dispersed private signals, the vague public signal and
   opening batches. Lagged open interest in `PriceQuote`. **Playtest.**
10. **Unlocks and rotating commodities,** once playtests show where new players struggle.

Steps 4 (C) and 6 (4) stay in place but run only if the harness gates above call for them.

## Open questions

Added:
- **`σ`, the signal offset and `openingInterval`.** Numbers for the harness to find.
- **Is the round summary's reveal of every signal too much?** It teaches, but it also shows
  players each other's habits, which regulars will use against each other across rounds.
- **Collusion gets easier.** Friends can share private signals out of band and average them. The
  second edition's options still apply; scaling `σ` with player count doesn't help against it.

Removed: the second edition's "`clearInterval` and the imbalance bar" is answered above, apart
from tuning.
