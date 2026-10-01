# Market Redesign

**Status: proposal. Nothing here is implemented.** It changes the rules in `PRICING.md`, and it
replaces the market that Stage 2 of `BOT_TRAINING.md` assumes. Stage 2 should not start until the
validation gates at the end of this document pass. Earlier editions of this proposal are in git
history.

## How the proposal got here

Two reviews, aimed at game feel and skill expression, shaped it. The problems they found, and the
answers:

1. **Parking in the deepest pool was a safe passive strategy.** Cash decay only forces players into
   *some* pool, not into risk. A round trip in oil's 2000-unit pool costs about 0.1%, and a lone
   holder there can't be profitably attacked. A strategy that makes about 0 ranked above a field
   that, on average, loses. **Answer:** a settlement value (E) makes every pool risky.
2. **There was no fundamental value, only flow.** Settlement paid the pool's own price, so a
   commodity was worth what the others would do next and nothing more: musical chairs. Nobody
   could be right that something was cheap. **Answer:** the settlement value (E).
3. **The exits were reflex races.** The first seller out of a crowd got the best price, and margin
   cascades rewarded whoever reacted first. **Answer:** batch clearing (D).
4. **The mid-round net-worth display leaked positions.** **Answer:** the mark uses only open
   interest the player could see anyway (B + 1, 4).
5. **The imbalance bar made the last tick of a batch a free look.** Irrevocable orders stop
   spoofing, but not sniping: wait for the window's last tick, read the bar, then submit.
   **Answer:** batches close on a random tick, and the bar shows direction only (D).
6. **Signals started a race to the station.** Trading needs you at the station, so whoever
   happened to stand at silver when silver's signal appeared took the first batch. **Answer:**
   signals arrive on a published schedule, and the first batch after one runs long enough for
   anyone to walk there (D, E).
7. **One public signal could crowd out the player-vs-player layer.** With everyone holding the
   same information about the value, "trade the signal, ignore everyone" wins, and the game
   becomes player vs puzzle. **Answer:** every player gets their own noisy signal, so other
   players' trades become evidence about the value (E), and open interest is live only at the
   station (4).
8. **Too much at once for a 5-minute round.** **Answer:** rotating commodities, unlocks, and
   short selling (C) and the spread-out map (4) made conditional on the harness (see "Managing
   the load").

One principle was also restated: reaction speed may count between humans, as long as decisions
count clearly more (see "Design principles").

## The problem

Under the current rules, not trading is close to the best strategy, and a trading bot that
learned to idle would just be reporting on the game.

- **The pool is everyone's only counterparty, and it takes a cut.** Any buy followed by a sell
  loses the spread (`commodity_test.go` checks that no sequence of trades beats that). There is
  one station per commodity, so there is no arbitrage between stations. Taken together, players
  can only lose money to the pool.
- **Every price move is permanent.** A buy lifts the price until someone sells. A player alone
  in a commodity keeps their position's value because nobody else touches it, and a pump pays
  nobody, because the only exit is selling back into your own impact.
- **Everyone starts in cash, and can only buy.** Not trading ends the round at exactly
  `StartingBalance`. Nothing forces anyone to act.
- **The final valuation ignores crowding.** `netWorth` values each player's holdings as if they
  alone sold into the pool, so piling into the same commodity costs nothing at the end.
- **Events are the only source of profit from outside the players,** and 70% of each event's
  move lands on the tick it is announced (`frontLoadBasisPoints`). Most of an event goes to
  whoever already held the commodity by luck.

With events off, this is the no-trade theorem (Milgrom & Stokey, 1982): traders with the same
information have no reason to speculate against each other, because whoever takes the other side
of your trade is evidence that you are wrong. Real markets trade because some participants have
to (hedgers, savers, people who need cash) and because some know things others don't. The
proposal below adds both, through rules that players act on, not through outside NPC traders, plus
a fundamental value for players to be right or wrong about.

## Design principles

- **Players compete against each other.** Money should mostly move between players. The game
  may push players (to act, to take risk) and may supply information, but outside traders with
  their own agenda are rejected, because they dilute the sense of beating other people.
- **No safe passive strategy.** Idling, buy-and-hold, parking in the deepest pool, and quietly
  owning one commodity alone must all lose to competent play.
- **Something to be right about.** A player should be able to think "this is too cheap" and be
  proven right or wrong, not only "the others will leave soon".
- **Skill over luck.** Outcomes should be repeatable. The better strategy should win across
  many seeds by more than seeds differ from each other.
- **Decisions over reflexes.** Reaction speed may count, but a better decision must be worth
  clearly more than a faster one. Measured, not argued: the reflex ratio in "Validation". Rules
  that shrink the reflex share are preferred to delays tuned per player.
- **Legible.** A new player has to understand why they lost money, and what to do differently.
- **Manageable.** A human can actively watch one or two positions. A 5-minute round shouldn't
  demand more.

## The features at a glance

| | Feature | Breaks | Player vs player, or player vs game |
|---|---|---|---|
| **A** | Cash loses value | Idling is safe | The game applies it to everyone equally; it just forces players to act |
| **B** | Settlement auction | Crowding is free | Player vs player: the crowd prices its own exit |
| **C** | Short selling (conditional) | A lone holder can't be touched; prices only go one way | Player vs player |
| **D** | Batch clearing | Reflex races | Neutral: it changes how trades happen, not who pays |
| **E** | Settlement value, with dispersed signals | Nothing to be right about; the parking lot | Mixed: the game sets the value, players learn it from each other |
| **1** | Staggered settlements per commodity | The round's middle is dead | Player vs player (turns B from once a round into a rhythm) |
| **2** | Bought tips | Everyone sees the same information | Player vs player: the edge exists only because others don't know |
| **3** | Staged events | Events reward luck, not play | Mixed: the game supplies the news, players fight over it |
| **4** | Stations spread apart (conditional) | Everyone sees everyone | Player vs player: information depends on where you stand |

**B is a special case of D:** a settlement is a batch in which every open position is forcibly
included. **B is also a special case of 1:** one settlement, at the end of the round. The rest of
this document treats B and 1 as one feature. The final settlement at the end of the round always
exists; 1 adds more during the round.

C and 4 go in only if the harness shows they're needed (see "Managing the load").

**What's public.** Prices, the settlement and signal schedules, open interest lagged by `oiLag`,
the vague public signal, and the fact that someone bought a tip. **What's private.** Who holds
what, live open interest away from the station you stand at, your own signal, the contents of
tips, and anything outside your field of view.

## How a round plays

Here is a 5-minute round with all features on, from one player's point of view.

1. **Start.** Everyone spawns at the centre with $1000. Four of the six commodities are live this
   round. The stations sit on a wide ring, so from the centre you see only part of it. The
   schedule shows that lithium's signal drops at 0:30 and it settles at 1:10, gold's signal drops
   at 1:05, and so on. A vague public signal says lithium is "likely up". Cash starts losing
   value on the first tick.
2. **Getting into positions.** You can't sit in cash. You walk to the information station first
   and buy a tip about silver. The board shows "a tip was bought" at the moment you do. The tip
   is a sharper read than the free one: silver's value is likely below its price. You walk to
   silver.
3. **Orders.** At silver you short 10 units. Your order goes into the current batch; a bar shows
   more sells than buys pending, and within half a second or so, at a moment nobody knows in
   advance, the batch clears and everyone in it gets the same price. Nobody could beat you by
   clicking faster or by waiting for the last tick. Anyone at silver saw your trade flash, and
   anyone who saw the tip notice can guess that someone's informed.
4. **A signal drop.** At 0:30 every player gets their own read on lithium: "your read: lithium
   settles around $41". Lithium's opening batch runs about 8 s, long enough for anyone to walk
   there. Players converge on the station, the bar fills, and each decides whether to join the
   crowd or fade it. When it clears, everyone in it gets the same price.
5. **Before the settlement.** Standing at lithium, you see its open interest live: 45 units long
   and 5 short. Players elsewhere see a copy 10 s old. The price is well above your read, and the
   crowd's buying says most reads were high, but not this high. The crowd has overshot. Players
   who read that start shorting lithium or leave it; a run for the door clears in one or two
   batches, at one price each, instead of rewarding whoever clicked first.
6. **Settlement.** At 1:10, lithium's settlement value is revealed and the pool is re-priced to
   it. Every open lithium position closes in one combined order from there. Those who read the
   value right, and left the crowd in time, win. Those who stayed late in an overshot crowd pay
   for it twice: once to the reveal, once to the crowd's own selling. Their cash starts decaying,
   so they have to re-enter somewhere.
7. **The middle of the round.** Cycles repeat, commodity by commodity. Someone buys a tip; a
   public rumour says an event is coming to oil. The event changes oil's settlement value, not
   its price, so the price moves only as players act on the news, and the ones who read it first
   and right take the money.
8. **The end.** Everything left open settles in the final auction. The final ranking is net worth
   after that auction. The round summary shows every player's signals, each settlement value, who
   traded which way, who bought tips on what, and at what times you were traded against by
   someone holding one.

What a player is thinking about the whole time: what is this worth, what do the others' trades say
they know, who's in it, when will they leave, and where should I stand to see it.

## Feature details

### A. Cash loses value

**Mechanism.** Every `decayInterval` (say one second), each player's cash balance shrinks by
`decayBasisPoints`. The amount lost is rounded up, so that cash stays whole cents and rounding
never favours the player, matching the pool's rule. The lost cash disappears. The ranking is by
net worth, so it doesn't matter where the money goes.

**What it does.**
- Idling now loses by a known amount, so any strategy that trades at a profit beats it.
- It makes every player a forced buyer. Everyone has to end up in some position. These forced
  trades are the uninformed flow that informed players (E, 2) profit from, and they come from the
  players themselves, not from an outside trader.
- With 1, settlement turns positions back into cash, the cash decays, and players must re-enter,
  so the round becomes cycles instead of one rush.

**What it doesn't do.** It doesn't force anyone to take risk; it only forces them into a pool.
On its own, parking in the deepest pool is a safe substitute for cash. E is what makes every pool
risky. A needs E.

**Knobs.**
- **Rate.** It has to beat a round trip in the deepest pool, or holding cash is better than
  holding anything. It also has to stay small enough that walking to an information station, or
  waiting for a signal drop, is still reasonable, or the round's start becomes a rush.
- **Profile.** A flat rate, or rising towards the end of the round, or only on cash above a
  threshold (a "savings tax").
- **What counts as cash.** Whether short collateral decays (see C) changes whether shorting is
  also a shelter from decay.

**Alternative routes.**
- **A holding bonus instead of a cash penalty.** Commodities pay a small per-tick yield. The effect
  on rank is the same, but the money is new money, so everyone's net worth rises. It feels
  better ("you earned") and is harder to reason about.
- **Storage costs on commodities too.** This punishes sitting in anything, but also punishes the
  forced re-entry that A exists to cause. Probably too much.
- **Scoring against an index.** Rank by net worth relative to a basket of all commodities. It has
  no mechanical effect and is hard to explain.

### D. Batch clearing

**Mechanism.** Trades no longer fill the moment they arrive. The server collects orders into a
batch and clears each commodity in one step:

1. Each player's orders in that commodity net to one signed change. A player can't trade against
   themselves.
2. Sum the buys `B` and sells `S` across players. The difference, `net = B − S`, trades with the
   pool as one order at per-unit price `P`, from the same `buyPrice`/`sellPrice` the pool already
   uses, so rounding favours the pool as it does now.
3. Every order in the batch fills at `P`: buyers pay `P` per unit, sellers receive `P`. Buyers pay
   `B·P`, sellers receive `S·P`, the pool receives `net·P`, so the cash balances exactly.
4. **Limits.** Each order still carries `price_cents`, its worst acceptable price. If `P` breaks
   some orders' limits, the order with the worst-violated limit is dropped, and the batch is
   recomputed, until no limit is broken. Ties are broken deterministically. Dropped orders are
   rejected with a `TradeReceipt` reason, as today.

When `net` is 0, the pool doesn't trade and `P` needs defining another way; the one-unit sell
price is a reasonable choice. That case needs pinning down with a test, and it's shared with B.

Orders are **irrevocable** once submitted. An order submitted at the station stays in the batch
even if the player walks out of range before it clears.

**Random close.** `clearInterval` defaults to 0.5 s (10 ticks). Each batch clears at a tick drawn
uniformly from a window around it, say 0.3–0.7 s after the previous clear. The draw is secret
until the clear happens, and comes from the world's seeded RNG so harness runs stay reproducible.
Waiting for the last tick to read the bar is then a gamble on missing the batch, not a free look.
Real exchanges close auctions the same way. An order that arrives after the draw fires goes into
the next batch, and its acknowledgement says which batch it landed in.

**Opening batches.** The first batch in a commodity after its signal drop (see E) runs
`openingInterval`, about 8 s, roughly the walk from the centre out to the ring and back towards a
neighbour. Its close is random over its last second. Everyone who decides to act on the signal can
get there and gets the same price. It's the moment the game builds up to: players converge on a
station, the bar fills, and each decides whether to join the crowd or fade it.

- Orders in an opening batch are irrevocable, like any batch.
- Margin calls in that commodity wait for the opening clear, so a squeeze can't fire in the
  middle of the walk.
- Other commodities keep clearing on their normal interval during it.

**What it does.**
- **Speed stops mattering inside a window.** A run for the door clears in one batch at one price.
  Being 50 ms faster than someone else buys nothing. This is the frequent-batch-auction result
  (Budish, Cramton & Shim, 2015), and it's the single biggest lever for "decisions over reflexes".
- **Margin calls fire at the next clear, not the next tick,** so a cascade unfolds in steps a
  human can watch and act in.
- **Players trade with each other directly.** Buys and sells in the same batch match against each
  other at `P`, and only the difference touches the pool. An informed buyer and a forced seller
  in the same batch are trading with each other. This is what makes E mostly player vs player.
- **Latency parity inside a batch.** A bot and a human who both decide inside the same window get
  the same price. Where speed still counts, see "Latency parity" under the bot.

**Feel.** The cost is the click-and-instant-fill feel. To keep trading snappy:
- A visible countdown to the next clear on every station, showing the window ("clears in
  0.3–0.7 s"), not a tick.
- Your pending order shown on screen until it clears, then the fill animated at `P`.
- An **imbalance bar**: the pending buy/sell imbalance per commodity during the window, public,
  and by default coarse: direction only (more buys, more sells, balanced), not size. Because
  orders are irrevocable it can't be spoofed: showing direction costs real exposure. Without
  irrevocability, the bar would reward fake orders pulled on the last tick, which is a reflex race
  again.

**Knobs.**
- **`clearInterval`.** Longer is calmer and fairer; shorter feels more responsive. 0.5–2 s,
  default 0.5 s: with reflexes allowed to count a little, the snappier feel is worth more than the
  last bit of fairness.
- **Close window.** How wide the random window around `clearInterval` is.
- **Imbalance bar.** Direction only (default), with size, or off. A lagged bar (the previous
  batch's imbalance) is the fallback if sniping still pays under random close.
- **`openingInterval`.**
- **Cross-commodity.** All commodities clear on the same tick, or staggered. Same tick is easier
  to reason about.

**Protocol.** A trade becomes two steps: the server acknowledges the order into the batch, then
answers it with the `TradeReceipt` when the batch clears. The receipt keeps its meaning: the whole
order filled at one per-unit price, or was rejected with a reason. A new `ServerMessage` member
acknowledges a pending order and carries the batch it landed in. `PriceQuote` gains the earliest
and latest time of the next clear, a flag for an opening batch and, if the knob is on, the pending
imbalance.

### B + 1. Settlement auctions

**Mechanism.** Every commodity has a settlement schedule. A settlement is a batch (D) with every
open position in that commodity forcibly included. At a settlement tick, the server:

1. Reveals the commodity's settlement value `V` and re-prices the pool to it (see E).
2. Sums every player's long units `L` and short units `S` in that commodity.
3. Trades the difference, `net = L − S`, with the re-priced pool as one order: a sell of `net` if
   it's positive, a buy of `−net` if it's negative. That order's per-unit price is `P`.
4. Settles every position at `P`: longs receive `P` per unit, shorts pay `P` per unit.

Longs receive `L·P`, shorts pay `S·P`, and the pool pays `net·P`, so the cash balances exactly.
Longs and shorts are matched against each other at the pool's price, and only the difference
touches the pool. The `net = 0` case is the same as D's.

After settlement the commodity reopens at whatever price the pool is left at, which is near `V`.

**What it does.**
- **A crowd pays for its own crowding.** Everyone in the same commodity gets the same price. With
  the reveal, a crowd long at the right value roughly breaks even among itself, since an AMM round
  trip loses only rounding. So entry order and exit timing decide who wins within the crowd.
- **Both sides pay for crowding.** A net-long crowd sells down into its own impact; a net-short
  crowd buys up into its own. This is a crowding penalty, symmetric for longs and shorts. It is
  not a squeeze: shorts are matched against longs at `P`, and only the net short pays extra. Real
  squeezes come from margin calls (C).
- **Leaving before the others is a decision that matters, and it recurs at every settlement.**
  With D it's a decision about *which batch* to leave in, not a race to leave first.

**Knobs.**
- **Number of settlements per round.** Too many is chaos, too few brings back the dead middle.
- **Schedule.** A fixed calendar can be solved by a script. A random one, announced some time
  ahead, can't; how far ahead it's announced is itself a knob.
- **Blackout window.** No new positions in the last few seconds before a settlement. With D this
  matters less, but it still stops the last batch before settlement from being a free look at the
  final imbalance.
- **Active commodities.** See "Managing the load".

**Alternative routes.**
- **Settle everything at once, periodically.** Simpler, but the whole map resets together, and
  there's never a "somewhere else is about to settle" to plan around.
- **Rolling over positions.** Positions carry past a settlement for a fee instead of being closed.
  Softer; it weakens the crowding penalty.
- **Settle holders one by one in random order.** Easier to implement, but where you land in the
  order is luck. Rejected.

**Scoring and the UI during the round.** Net worth shown mid-round is what a settlement right now
would pay at the current pool price, each commodity's `net` traded as one order, using the lagged
open interest. A live mark would leak the live open interest that only players at the station can
see (4); the lagged copy leaks nothing new, and the mark doesn't flatter crowded positions. It
can't include `V`, which is unknown; the UI says so ("marked at today's price, before the
reveal"). The same value, with true positions and `V` substituted where the training env knows
them, is the bot's per-step reward (see "Training the trading bot").

**Protocol.** New `ServerMessage` members for a settlement being scheduled (with its signal time,
see E) and its result (`V`, `P`, `net`, the player's own proceeds). Each `PriceQuote` gains the
time to its commodity's next settlement and its open interest (total long, total short): live at
the station the player stands at, lagged elsewhere.

### E. Settlement value

**Mechanism.** Each commodity has a hidden settlement value `V` for its next settlement. At the
settlement tick (B, step 1), the pool is re-priced: `cashReserve` is set so that the marginal
price equals `V`, keeping `unitReserve`. Then the netted order trades on the curve from there.

- `V` for each settlement is drawn when the previous one ends: a step from the current price, of
  a size the knobs control. A commodity's `V` is not known to anyone until the reveal.
- Between settlements, the pool price moves only when players trade. **The game never moves the
  price directly.** It only sets `V` and tells people about it.

**Signals.** Players learn about `V` before the reveal through:
- **Dispersed private signals**, the main source (below).
- **A public signal**, vague by default (direction only), so a player who ignores everything still
  has something.
- **Bought tips** (2): a sharper private signal.
- **Staged events** (3), which shift `V` and announce it in stages.

**Signal drops.** Each commodity's cycle has a published signal time, announced with the
settlement schedule, a fixed offset before the settlement it's about. At the signal time, all
signals for that commodity arrive at once, and an opening batch starts (D). Being at the right
station is planning, not luck: you know where the next signal lands, and walking there early is a
decision with a cost (you aren't somewhere else).

**Dispersed private signals.** At each signal drop, every player gets their own signal about `V`:
`V` plus independent noise of spread `σ`, rounded to whole cents. Everyone's signal is equally good
and costs nothing. One signal is weak; the average of all of them is close to `V`.

- **Why this keeps the game player vs player.** A player's trades leak their signal. When someone
  buys silver after the drop, it's evidence that their signal was high. Trade flashes in field of
  view, open interest and the imbalance bar all become ways to learn what the others know. Reading
  other players doesn't compete with reading `V`; it is the best way to read `V`. This is the
  classic informed-trading setup (Grossman & Stiglitz, 1980; Kyle, 1985).
- **Why it isn't luck.** Random tips are rejected because some players get them and others don't.
  Here everyone gets the same quality at the same time. Luck is in the noise draw, which several
  settlements per round average out.
- **Bluffing.** Trading against your own signal to mislead the others costs you the gap between
  your trade and your best guess at `V`. That makes a bluff a real bet, answered by reading.
- **In the UI.** Shown as a read, not a number to four figures: "your read: silver settles
  around $9.40". The round summary reveals every player's signals, `V`, and who traded which way,
  so a player can see what the crowd knew and what they missed.

**What it does.**
- **There is something to be right about.** "Lithium is overpriced" becomes a claim that the reveal
  settles. Losing because your read was wrong is a lesson; losing because you were late is not.
- **Every pool carries risk, so there is no parking lot.** Holding oil through a settlement is
  exposed to `V` like anything else. A parker's expected result is still about zero minus the
  spread, but the field's isn't negative any more: informed players take money from the reveal,
  so competent play is profitable, and a zero-sum parker ranks below it.
- **Informed players take from uninformed players, not just from the pool.** With D, an informed
  buyer and a forced seller in the same batch trade with each other. The pool pays the gap
  between its price and `V` only for the part that didn't cross. The better the players collectively
  read `V`, the more of the money moves between players, and the less comes from the pool.
- **Reading others matters more, not less.** Price is a mix of `V` and crowd noise, and each
  player's trades say something about their read. "The price is above my read *and* the crowd is
  45 long" is a trade. "Somebody just bought a tip and silver's imbalance flipped" is a trade.

**The honest cost.** Some money now comes from the game: the pool loses to informed trading at the
reveal. That's player vs game, like events today. It's acceptable because the pool's loss is
captured by whoever read the signals first and best, at the expense of those who didn't; rank is
relative, so what matters is who gets it. The harness should measure how much of the profit comes
from the pool and how much from other players (see "Validation").

**Knobs.**
- **Size of `V`'s step.** Sized to about the price impact of a typical crowd in that pool, not
  larger, so that crowding and value matter about equally and neither dominates. Too small and E
  doesn't matter; too large and the reveal is a lottery. The harness finds the number.
- **`σ`, relative to `V`'s step.** Large `σ` makes your own signal nearly useless and reading the
  others everything; small `σ` makes your own signal enough, and the PvP layer thins out.
- **`σ` with player count.** A fixed `σ` makes the crowd's average sharper in bigger worlds.
  Scaling `σ` with the square root of the player count keeps the average equally good at every
  world size, at the cost of weaker individual signals in big worlds.
- **The public signal.** Vague (default), sharper, or off.
- **Signal offset.** How long before a settlement its signal drops.
- **Blend.** The reveal can re-price the pool fully to `V`, or a fraction of the way. A fraction
  keeps more of the outcome in players' hands, but makes `V` harder to explain.

**Alternative routes.**
- **Settle at `V` directly, without the pool.** Pay every position `V`, and the pool (the house)
  absorbs the net at `V`. Simpler, but removes the crowding penalty entirely. Rejected.
- **No reveal; only events.** A flow-only market that relies on events for value. The dead middle
  and the parking lot come back when no event is running.
- **One public signal only.** Everyone knows the same thing, and the game drifts to player vs
  puzzle. Rejected.

**Protocol.** The settlement result carries `V`. One new per-player `ServerMessage` member carries
the private signal. The public signal is another member (or a tip kind in 2's payload, sent to
everyone). The signal schedule rides on the settlement-scheduled message.

### C. Short selling

**Conditional:** goes in only if the harness shows a lone holder or a pumped price still can't be
punished without it (see "Managing the load"). Unlocked for players after a number of ranked
rounds.

**Mechanism.** Holdings become signed. Selling below zero opens a short: the sale proceeds are
credited as usual. A player's collateral (proceeds plus free cash) must always cover
`marginMultiple × |units| × buy price`. If a rising price breaks that, the server force-covers
the short at the next clear (D): a buy into the pool, which pushes the price up further, which can
break the next short's margin at the clear after. That cascade is the squeeze, and with D it
unfolds batch by batch, where a human can watch it coming and act.

**Pool limit.** At settlement, net shorts become a buy from the pool, and the pool must keep at
least one unit, so `buyPrice` exists. A per-commodity cap on total open shorts, as a fraction of
`unitReserve`, keeps that buy fillable. With lithium's pool of 60 units, shorts in lithium stay
small, which makes it the commodity longs are safe in. That's worth knowing, not necessarily
worth fixing.

**What it does.**
- A lone holder is attackable: short into their pumped price, and at settlement the netting
  (their sell against your cover) brings the price back to about where they bought, and they
  lose what you gained. In a deep pool that gain is small, which is why C alone doesn't kill
  parking and E is needed.
- Every pumped price invites a bear; every crowded short invites a squeeze. Prices move both ways.
- **Engineering a squeeze is a skill.** Buying to push shorts into their margin calls, then selling
  into the forced covers, is a real play that rewards reading open interest and margin levels.

**Knobs.**
- **`marginMultiple`.** Low is brutal: fast margin calls, big cascades. High is gentle.
- **Open-short cap per commodity.**
- **Borrow fee.** A per-tick cost on open shorts, the counterweight to A's cost on cash.
- **Decay on collateral.** If locked proceeds decay like cash, shorting costs a bit to hold, the
  way longs are free to hold. If they don't, a short is also a shelter from decay. Either choice
  biases the market one way.

**Alternative routes.**
- **No margin calls, capped losses.** Shorts can't lose more than their collateral, and nobody is
  force-covered. Gentler, but no squeezes, which removes most of the fun.
- **Shorts only in the last part of the round.** Keeps early rounds simple for new players.

**Protocol.** `OwnedCommodity.amount` is `uint64` today; it has to carry a sign, or shorts get
their own field. Commodity quantities stay whole units. Two ways to open a short:
- **Let `INTENT_SELL` go below zero.** Fewer messages, but easier to short by accident.
- **New `INTENT_SHORT` and `INTENT_COVER`.** Explicit, and clearer in the UI.

`TradeRejection` gains reasons for margin and the short cap, and a forced cover needs its own
message, so the player knows why their position changed.

**UI.** The biggest risk to fun here is a player who doesn't understand why they lost. The client
has to show "you owe 20 units; at the current price that's $X; you'll be force-covered at $Y"
plainly, before it happens.

### 2. Bought tips

**Mechanism.** An **information station** sits on the map. A player standing at it can buy a tip
for cash. Tips are generated from the true game state, with deliberate noise when reliability is
a knob. When anyone buys a tip, every player sees that a tip was bought, and by whom if they're in
field of view, but not what it says.

| Tip | Example | How player vs player |
|---|---|---|
| **A value** | "Silver's settlement value is likely below $9.50": one more draw about `V`, with less noise than the free signal | Mostly: about `V` (E), and its worth depends on what the crowd's trades already reveal. |
| **A position** | "Player X is long 30 lithium" | Fully: it's only about other players. Open interest says how much crowd there is; the tip says who. |
| **A read** (candidate) | "Player X's read on silver was high" | Fully: it tells you whether X's trades are information or a bluff. |
| **The schedule** | "Silver's next settlement is at 2:40", before it's public | Mostly: the profit is in acting before the others. |
| **An event** | "Shortage in oil in about 15 s" | Partly: the edge comes from the game; see 3. |

**What it does.** It creates the informed-trader game. The tipped player profits only if they
trade without giving the tip away, and everyone else profits by spotting who got it from how they
move and trade. Buying is a decision rather than luck: the price, the walk to the station, and
the notice that tells everyone you're informed are all costs. A value tip's worth is relative to
what the crowd's trades already reveal, which makes the decision to buy harder and more
interesting. It's also a cash sink that pairs with A.

**Why buying is public.** Being traded against by someone with a tip you didn't know existed feels
unfair and teaches nothing. A public "a tip was bought" makes the buyer a target and makes bluffing
possible (buy a cheap tip, trade the other way). The round summary goes further and reveals, after
the fact, when you were traded against by a tipped player.

**Knobs.**
- **Price.** Flat, or rising with how many were bought this round.
- **Reliability.** Some tips wrong. Acting on a tip becomes a risk, and whoever is reading you
  can't be sure either.
- **Lifetime.** How long a tip stays true. A position tip goes stale as soon as the player trades.
- **Who is named in the notice.** Everyone sees the buyer, only those in field of view do, or
  nobody does (only the fact).

**Alternative routes.**
- **Random tips.** Luck. Rejected.
- **Rotation, so everyone gets the same number.** Fair, but passive; nobody decides anything.
- **Tips as rewards** for a good trade, a settlement won, or scouting. Rich get richer; bad for
  newcomers.

**Fairness.** Humans see tips in the UI; the bot gets them as observation slots, with the same
timing. Neither gets them sooner.

**Protocol.** One new per-player `ServerMessage` member for the tip, whose payload is a `oneof`
over tip kinds, and one broadcast member for the notice that a tip was bought.

### 3. Staged events

**Mechanism.** An event changes a commodity's settlement value `V` (E). It never moves the price
directly; `frontLoadBasisPoints` and the drift go away. It arrives in stages:

1. **A tip** (2) available at the information station, about 15 s before.
2. **Public rumour** about 7 s before. It could be vague on purpose: the commodity but not the
   direction, or the direction but not the commodity.
3. **The event** is announced, with its size. `V` changes.
4. **The reveal** at the commodity's next settlement pays it out.

**What it does.** Players price the event in themselves: they trade on the warning, and the
latecomers pay them. Between the announcement and the reveal, the price is wherever players have
pushed it, and anyone who thinks the crowd over- or under-reacted can trade against it. The event
becomes a contest of reading, not luck of already holding, and with D it isn't a click race
either.

**Knobs.**
- **Stage timing.**
- **False rumours.** A public rumour that doesn't come true punishes a crowd that trusts it
  blindly.
- **How vague each stage is.**
- **Time from announcement to reveal.** Short makes the event a sprint; long gives the crowd time
  to overshoot, which is where the skill is.

**For the bot.** Stage 2 training starts without events (see below). Events come back at this
feature.

### 4. Stations spread apart

**Conditional:** goes in only if the harness shows reading other players is still worth too
little with dispersed signals (see "Managing the load"). Open interest being live only at the
station applies either way.

**Mechanism.** Today the server's stations sit on a 25×16 ellipse around the centre
(`NewTradingStations`) and `DefaultFOVRadius` is 37.5. From one station you can't see the opposite
one, but from the centre you see almost everything. Moving the ring out to a radius of around
60–80 means that from any station you see only that station and its neighbours.

Within field of view, **every trade shows as a flash at the station**, attributed to the player
who made it. This is on by default, not a knob: it's the core signal for reading other players.

**Open interest is live only at the station.** Players at a station see its open interest update
with every clear. Everyone else sees a copy lagged by `oiLag`, say 10 s. Being there matters, and
the crowd is people you can see rather than a number on a board.

**What it does.**
- Seeing who is trading where becomes information you earn by being there. `MarketState` still
  goes to everyone, so trades are visible as anonymous price moves and lagged open interest; the
  map hides who made them.
- Where you stand becomes a decision. With 1 you can't be at every settling commodity, and with C
  you can only attack where you are. The information station (2) is one more place to be.
- **Bluffing becomes possible.** Standing at a station, or a small buy-then-sell to fake interest,
  costs only the spread and some time. Cheap bluffs, answered by reading, are good skill
  expression.
- It feeds 2: a position tip is worth more when you can't just look.

**Costs.** At `MoveSpeed` 15.6, crossing a ring of radius 70 takes about 9 s, roughly 3% of a
5-minute round. Much further and the game starts being about walking. `openingInterval` (D) has to
stay at least the walk from the centre to the ring.

**Knobs.**
- **Ring radius, or FOV radius.** A smaller FOV does the same thing without moving stations, but
  also changes how the game looks.
- **`oiLag`.**
- **`PriceQuote.dominant_whale_id`.** Already in the protocol and unused. Filling it with the
  largest holder makes positions public and undoes most of the point of this feature and of
  position tips. It stays empty.

**Alternative routes.**
- **Hubs:** clusters of two or three stations with distance between clusters. You watch a whole
  hub at once, and choose which hub to be at.
- **Scouting tools:** a way to see a distant station briefly, for a price.

**For the bot.** Other players drop out of view more often, so the observation needs memory of
who was last seen where. Pathing (Stage 1b/1c) matters more on a wider map.

## Managing the load

- **Rotating active commodities, on by default.** Four of six live each round, chosen at round
  start and shown with the schedule.
- **Unlocks.** New players play long-only: A, B + 1, D, E. Short selling (C) unlocks after a number
  of ranked rounds. Tips (2) can follow the same path. It also softens the problem of newcomers
  funding veterans: nobody meets a squeeze in their first rounds.
- **C and 4 are conditional.** Each goes in only if the harness shows what it's for is missing:
  C if a lone holder or a pumped price still can't be punished, 4 if reading other players is
  still worth too little with dispersed signals. Dispersed signals may already give players enough
  to read.

## Skill vs randomness

| Source | Luck or skill | What keeps it from being luck |
|---|---|---|
| Spawn and the start of the round | Luck plus reflexes | Low decay rate (A), several settlement cycles (1) so the first one is a small part of the round, D |
| Settlement timing | Skill | A schedule that is known or announced, a blackout window |
| Settlement value (E) | Luck without signals, skill with them | Dispersed private signals, the public signal, bought tips, a step size that isn't a lottery, several settlements per round |
| Being at the right station for a signal | Planning | A published signal schedule, opening batches long enough to walk to |
| Crowding | Skill | Open interest, live at the station and lagged elsewhere; 4 makes who's crowded costly to learn |
| Short squeezes | Skill with risk | Visible margin state in the UI, a margin multiple that isn't brutal, D making cascades step by step |
| Who gets tips | Skill | Bought, not given (2) |
| Events | Skill with 3 | Staging, events moving `V` not price |
| Reaction time | Secondary | D with random close, opening batches, a human-like reaction delay for the bot; measured by the reflex ratio |

Two failure modes to watch for:
- **The start becomes a rush.** A decay rate that's too high forces everyone to buy on tick 0,
  before any signal or tip. Keeping it low enough that waiting for information pays is the fix.
- **Luck dominates.** A `V` step too large for the signals makes every settlement a lottery.
  Measured, not argued: see "Validation".

## Player vs player, and player vs game

Where money comes from and goes, per feature:

| Flow | Direction | Feel |
|---|---|---|
| Pool spread and rounding | Players → pool | Player vs game, small and constant, as today |
| Cash decay (A) | Players → nowhere | Player vs game, but applied equally; it matters only relatively |
| Batch crossing (D) | Between players in the same batch | Player vs player |
| Settlement crowding (B + 1) | Late crowd → early crowd; longs ↔ shorts | Player vs player |
| Settlement value (E) | Worse readers → better readers, in batches; pool → informed, for the part that didn't cross | Mostly player vs player |
| Shorting and margin calls (C) | Losers of a squeeze → winners | Player vs player |
| Tips (2) | Uninformed → informed; buyers → nowhere (the price) | Player vs player |
| Events (3) | As E | Mostly player vs player |
| The map (4) | None directly | Player vs player, through information |

The game's own flows (spread, decay, tip prices) set the pressure, and the reveal pays the players
who read it. Almost every cent a player wins, another player loses, or the player who read the
signals first took it from the pool before others could.

## Training the trading bot

These changes are what make Stage 2 learnable at all. Under the current rules, pure speculative
self-play has a stable outcome where nobody trades, and PPO would likely find it.

**What gets easier.**
- **No no-trade trap.** A costs reward every tick spent idle, so from the first episode the
  gradient pushes the bot into positions.
- **Self-play has something to learn.** With B + 1 (and C, if it's in), any position can be
  punished, so the best play depends on what the others do. With E there's also a right answer to
  learn about each commodity, which gives the gradient something stable to climb before the
  opponent game matters.
- **Training finds exploits.** PPO is good at finding degenerate strategies. Short runs are an
  automated playtest: if the bot converges on something dull, humans would have found it too.

**What gets harder.**
- **The payout is lumpy.** Settlement pays at specific ticks. The per-step reward should be the
  change in net worth marked at what a settlement right now would pay, with true positions and
  `V`, which the env knows even though the policy doesn't. The reward may use information the
  observation can't. Only the policy is bound by what a human would know.
- **High-variance rewards.** Margin cascades and reveals can swing a round. That needs reward
  normalisation, maybe clipping, and more samples per update.
- **Cycles become more likely.** Bulls, bears and squeezers have a rock-paper-scissors shape the
  long-only game didn't. The win-rate matrix and opponent sampling from `BOT_TRAINING.md` move
  from "measure first" to "probably needed".
- **Design churn.** Every rule change moves the optimum, and every observation change throws the
  policy away. No serious training until the rules settle.

**Observation additions.**
- Signed position per commodity, collateral and margin state.
- A short price history per commodity (an MLP sees only the present, and trends are the signal).
- Open interest per commodity, live where the bot stands and lagged elsewhere, and the batch
  imbalance.
- Time to the next clear's window, to each commodity's next signal drop and settlement, and to the
  end of the round. Whether the current batch is an opening batch.
- Own private signal per commodity, and the public signal.
- Tip slots (2), empty when there's no live tip, and the tip-bought notices.
- Last-seen position and time for other players, and trades seen in field of view (4).
- Event stage information (3), once events return.

**Actions.**
- **Macro actions.** The policy picks a station (including the information station) and a signed
  position change; scripted steering walks there. That cuts the horizon from 6000 ticks to a few
  hundred decisions, which matters more than any other design choice here for credit assignment.
  With D, one decision per batch is a natural step.
- **A signed position change, not separate buy/sell/short/cover.** "+10 lithium" and "−10
  lithium" cover all four, and going short is just going below zero. The protocol can differ; the
  env maps the action onto whatever C decides.

**Reward.** The ranking is by net worth, so the objective is rank. Mine minus the mean of the
others' is dense and rewards beating the field; mine minus the best other's is closer to "win",
and makes a bot that's behind gamble. An end-of-round rank bonus on top of either is an option.

**Self-play setup.** One learning policy, sharing weights across several slots, and frozen
opponents in the other slots: past snapshots and the scripted strategies from the harness below.
Frozen opponents run in Go through `MLPPolicy`, so Python only sees the learner's slots and picks
which snapshots to play at each `Reset`. That answers `BOT_TRAINING.md`'s "where the league lives".
Prioritised opponent sampling and exploiter agents come only if the win-rate matrix shows cycles.

**Latency parity.** D makes decisions inside a batch equal. Where speed still counts (inside a
batch window, walking to an opening batch, reacting to a margin cascade), the bot acts with a
human-like reaction delay, 250–400 ms (5–8 ticks), the same in the sim and in production.
Otherwise the reflex share of skill becomes the bot's share. Random close makes the delay matter
less, but it stays from the first run. The bot's decision for a batch is made on the same
observation a human has, and no later.

**Staging the curriculum.**
1. Speculation: A, B + 1, D, E with dispersed signals and the vague public signal, rotating
   commodities. No tips, no events. C and 4 join if the harness keeps them. This is the question
   Stage 2 was asking: can the bot beat other traders by trading alone?
2. Add tips (2).
3. Add staged events (3), so the bot isn't surprised by them in production.

**What to expect.** Beating the scripted strategies: very likely, provided the harness shows the
game has an edge. Beating casual humans: plausible. Beating strong, adapting humans: uncertain;
this is an imperfect-information game, where plain self-play can circle without converging. The
exploitability test (freeze the bot, train a fresh agent to beat it, see how fast it succeeds)
measures robustness better than a win rate does.

## Validation: the strategy harness

Before any of this ships, and before the bot trains seriously, a Go harness plays scripted
strategies against each other in the same world, across many seeds, and reports rank and net
worth per strategy.

**Strategies.**
- **Idle.**
- **Buy-and-hold:** a basket on tick 0, held to the end.
- **Oil parker:** everything in the deepest pool, re-entered after each settlement. The cheapest
  shelter from decay.
- **Lone holder:** claims an unclaimed commodity and holds it.
- **Early rotator:** gets into a commodity before others, leaves before settlement.
- **First-out exit racer:** sits in crowds and sells on the first sign of anyone else selling.
  Pure reflex.
- **Public-signal follower:** trades on the public signal about `V`.
- **Own-signal follower:** trades its own private signal and ignores everyone else.
- **Signal reader:** its own signal plus what it infers from open interest, the bar and trades in
  view. The strategy the game should reward most.
- **Pre-positioner:** walks to the next signal drop's station early.
- **Last-tick sniper:** submits as late in the batch window as it can, after reading the bar.
- **Tip buyer:** buys tips and trades on them (once 2 exists).
- **Trend follower.**
- **Random trader.**
- **Short attacker:** shorts lone holders and crowded pumps (once C exists).
- **Oracle:** cheats by knowing what the others will do next and what `V` is. It measures the skill
  ceiling.

**Metrics.**
- **Rank per strategy**, averaged over seeds, in 2-, 4- and 8+-player worlds.
- **Repeatability:** a strategy's advantage against its variance across seeds. If the variance
  is bigger, the game is mostly luck.
- **Skill ceiling:** how far the oracle beats the field. If even perfect knowledge of the others
  barely helps, reading other players is worthless.
- **Reflex value:** each strategy against a copy of itself delayed by 5 ticks.
- **Decision value:** the signal reader against the random trader, at the same latency.
- **Reflex ratio:** reflex value over decision value.
- **Where profit comes from:** how much of the winners' profit came from other players, and how
  much from the pool at reveals. If it's mostly the pool, E is too generous and the game has
  drifted to player vs game.
- **When money is made:** profit and loss over the round. If it's all at tick 0 and the end, the
  middle is dead.

**Gates.**
- Idle, buy-and-hold, the oil parker and the lone holder rank below the median, in every world size
  with at least one competent strategy in it.
- The early rotator, the public-signal follower and the short attacker (if C is in) win.
- The reflex ratio is under a quarter, in every world size.
- The last-tick sniper gains less than the seed variance over a copy that submits at a random
  moment in the window.
- The signal reader beats the own-signal follower by more than the seed variance. If it doesn't,
  reading other players is worthless and `σ` is too small.
- The own-signal follower beats the random trader. If it doesn't, `σ` is too large and signals
  are noise.
- The pre-positioner does better than a copy that walks only after the drop, but by less than the
  signal reader beats the own-signal follower. Being there early should help, not decide.
- The random trader loses, but not too hard, so new players aren't punished too much.
- The oracle clearly beats the field.
- Most of the winners' profit comes from other players, not from the pool.
- Profit and loss spread over the round.

The same scripted strategies become the bot's baselines and first frozen opponents, so none of
this work is thrown away.

## Playtests

The harness measures balance, not fun. After steps 3, 5 and 7 of the rollout, a round of human
playtests, with the same questions each time:

- **Could you say why you lost?** In one sentence, right after the round. If most players can't,
  the game fails legibility, whatever the harness says.
- **What would you do differently?** A loss that suggests a different decision is a lesson; one
  that suggests clicking faster is a bug.
- **How many real decisions did you make per minute?** Too few is a dead middle; too many, and
  players stop reading and start guessing.
- **When were you bored, and when were you overwhelmed?**
- **Did anything feel unfair?** Especially tips, squeezes and reveals.
- **Did you use what others were doing to guess the value?** If players never do, dispersed
  signals aren't legible yet.
- **Did the opening batches feel like a moment, or like waiting?**

## Rollout

1. **The harness, on the current rules.** Measures the problem before anything changes it.
2. **D, batch clearing,** with random close and the coarse bar. It changes the trade path
   everything else builds on, so it goes first. Client: pending orders, the window countdown, the
   fill animation.
3. **A + B** (the final settlement only). Small: decay on balances, and the close in `phase.go`.
   Tune the decay rate with the harness. Expected failures: the lone holder, which C fixes, and
   the oil parker, which E fixes. **Playtest.**
4. **C,** only if the harness gates call for it. Protocol, rules, UI and `PRICING.md`. The
   decisions under C come first.
5. **1 + E,** with the signal schedule, dispersed private signals, the vague public signal and
   opening batches. Lagged open interest in `PriceQuote`. **Playtest.**
6. **4,** only if the harness gates call for it: the map, with trades visible in field of view.
   Unity camera and layout; retrain Stage 1c pathing if the station ring moves.
7. **2,** bought tips and the information station. **Playtest.**
8. **3,** staged events, reworked to move `V`.
9. **Stage 2 bot training,** following the curriculum above, with short PPO runs from step 4 on
   as exploit probes.
10. **Unlocks and rotating commodities,** once playtests show where new players struggle.

## Open questions

- **Decay rate, settlement count, `V`'s step size, `σ`, the signal offset, `openingInterval` and
  `clearInterval`.** Numbers for the harness to find.
- **Margin calls or capped losses.** Squeezes are the fun of shorting, but they're also where new
  players lose badly without understanding why. D makes them slower and more visible; is that
  enough?
- **Does short collateral decay?** It decides which way the market leans.
- **Is the round summary's reveal of every signal too much?** It teaches, but it also shows
  players each other's habits, which regulars will use against each other across rounds.
- **Collusion.** In a game where money moves between players, two friends, or two accounts, can
  pump and dump into each other: one buys early, the other buys late on purpose to feed them. With
  private signals it gets easier: friends can share their signals out of band and average them,
  and scaling `σ` with player count doesn't help against that. In an MMO this will happen.
  Options: ranked lobbies filled at random with no chosen parties, reporting trades that
  consistently lose to the same counterparty, and rank rewards that make feeding a friend worth
  less than playing. Nothing is decided.
- **Newcomers fund the veterans.** In a crowd game, new players are structurally the late buyers
  and the uninformed flow. Unlocks help; beyond them and "the random trader loses, but not too
  hard": matchmaking by rating, lobby sizing, or a practice mode against scripted strategies
  before ranked play.
- **Active commodities per round.** Four of six is the default; whether that's the right number.
- **Mean reversion as a fallback.** A pool that relaxes towards an anchor price between trades
  also kills lone holding, without shorting. It's the house moving money, so it's kept as a tuning
  tool if C, E and 1 leave a passive strategy standing. With C it needs care: a pump that fades on
  its own makes shorting it nearly free money. E already re-anchors at every reveal, which may be
  enough.
