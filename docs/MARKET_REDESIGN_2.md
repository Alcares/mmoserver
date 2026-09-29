# Market Redesign: Proposal, Second Edition

**Status: proposal. Nothing here is implemented.** This supersedes `MARKET_REDESIGN.md`, which
stays as it is for the record. It changes the rules in `PRICING.md`, and it replaces the market
that Stage 2 of `BOT_TRAINING.md` assumes. Stage 2 should not start until the validation gates at
the end of this document pass.

## What changed from the first edition

A review aimed at game feel and skill expression found four structural problems in the first
edition, plus a handful of smaller ones:

1. **Parking in oil was a new safe passive strategy.** Cash decay only forces players into *some*
   pool, not into risk. A round trip in oil's 2000-unit pool costs about 0.1%, and a lone holder
   there can't be profitably attacked, because the attack pays in proportion to the holder's
   price impact. The game as a whole lost money to the pool and to decay, so a strategy that makes
   about 0 ranked above the average active player. The gate "buy-and-hold ranks last" would have
   failed for an oil basket.
2. **There was no fundamental value, only flow.** Settlement paid the pool's own price, and that
   price only moves when players trade. A commodity was worth what the others would do next and
   nothing more: musical chairs. Nobody could be right that something was cheap, and the answer
   to "why did I lose?" was "you were late", which isn't a lesson.
3. **The exits were reflex races.** The first seller out of a crowd gets the best price, the first
   visible sell starts a run for the door, and margin cascades reward whoever reacts first. The
   blackout window only removed the last-tick dodge. At 20 Hz a human needs 5+ ticks to react,
   plus the network, plus the clicks, so "delay the bot a few ticks" wasn't parity.
4. **The mid-round net-worth display leaked positions.** Marking net worth at "what a settlement
   now would pay" needs each commodity's net position, and from their own mark any player could
   work it out. That was most of what hidden positions and position tips were protecting.

This edition answers them with two new core features, **D. batch clearing** and **E. settlement
value**, and one decision up front, **open interest is public**. Smaller changes:

- The "squeeze at settlement" wording was wrong: netting matches shorts against longs at one
  price, so settlement is a crowd penalty for net shorts, not a squeeze. Real squeezes come from
  margin calls. Fixed in B and C.
- Trades are visible in the field of view by default, not as a knob; it's the core signal for
  reading other players.
- Tips are bought at an information station, not handed out at random, and receiving one is
  public.
- Events no longer move the price. They move the settlement value and tell people about it.
- New open questions: collusion, and newcomers structurally funding veterans.
- The harness gains an **oil parker**, a **first-out exit racer**, and a reflex-sensitivity metric.
  The rollout gains human playtests with concrete questions.

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
proposal below adds both, through rules that players act on, not through outside NPC traders.

The first edition added the "have to", but not a fundamental value to be right or wrong about.
Without one, the game is a pure contest of guessing flow, and a passive strategy that makes
nothing beats a field that, on average, loses. This edition adds both halves.

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
- **Skill over reflexes, by construction.** A race decided by reaction time is won by a bot with
  zero latency. Rules should make speed irrelevant, not merely less relevant; tuning delays is
  the fallback, not the plan.
- **Legible.** A new player has to understand why they lost money, and what to do differently.
- **Manageable.** A human can actively watch one or two positions. A 5-minute round shouldn't
  demand more.

## The features at a glance

| | Feature | Breaks | Player vs player, or player vs game |
|---|---|---|---|
| **A** | Cash loses value | Idling is safe | The game applies it to everyone equally; it just forces players to act |
| **B** | Settlement auction | Crowding is free | Player vs player: the crowd prices its own exit |
| **C** | Short selling | A lone holder can't be touched; prices only go one way | Player vs player |
| **D** | Batch clearing | Reflex races | Neutral: it changes how trades happen, not who pays |
| **E** | Settlement value | Nothing to be right about; the parking lot | Mixed: the game sets the value, players race each other to it |
| **1** | Staggered settlements per commodity | The round's middle is dead | Player vs player (turns B from once a round into a rhythm) |
| **2** | Bought tips | Everyone sees the same information | Player vs player: the edge exists only because others don't know |
| **3** | Staged events | Events reward luck, not play | Mixed: the game supplies the news, players fight over it |
| **4** | Stations spread apart | Everyone sees everyone | Player vs player: information depends on where you stand |

**B is a special case of D:** a settlement is a batch in which every open position is forcibly
included. **B is also a special case of 1:** one settlement, at the end of the round. The rest of
this document treats B and 1 as one feature. The final settlement at the end of the round always
exists; 1 adds more during the round.

**What's public.** Prices, each commodity's open interest (total long and total short units), the
settlement schedule, and the fact that someone bought a tip. **What's private.** Who holds what,
the contents of tips, and anything outside your field of view.

## How a round plays

Here is a 5-minute round with all features on, from one player's point of view.

1. **Start.** Everyone spawns at the centre with $1000. The stations sit on a wide ring, so from
   the centre you see only part of it. The settlement schedule shows that lithium settles first,
   at 1:10, gold at 1:45, and so on. A public signal says lithium's settlement value is "likely
   above today's price". Cash starts losing value on the first tick.
2. **Getting into positions.** You can't sit in cash. The public signal is the same for
   everyone, so lithium will be crowded. You walk to the information station first and buy a tip
   about silver. The board shows "a tip was bought" at the moment you do. The tip says silver's
   value is likely lower than its price. You walk to silver.
3. **Orders.** At silver you short 10 units. Your order goes into the current batch; a bar shows
   more sells than buys pending, and in under a second the batch clears and everyone in it gets
   the same price. Nobody could beat you by clicking faster. Anyone at silver saw your trade
   flash, and anyone who saw the tip notice can guess that someone's informed.
4. **Before the first settlement.** Lithium's open interest shows 45 units long and 5 short, and
   its price is well above where the signal pointed. The crowd has overshot. Players who read
   that start shorting lithium or leave it; a run for the door clears in one or two batches, at
   one price each, instead of rewarding whoever clicked first.
5. **Settlement.** At 1:10, lithium's settlement value is revealed and the pool is re-priced to
   it. Every open lithium position closes in one combined order from there. Those who read the
   value right, and left the crowd in time, win. Those who stayed late in an overshot crowd pay
   for it twice: once to the reveal, once to the crowd's own selling. Their cash starts decaying,
   so they have to re-enter somewhere.
6. **The middle of the round.** Cycles repeat, commodity by commodity. Someone buys a tip; a
   public rumour says an event is coming to oil. The event changes oil's settlement value, not
   its price, so the price moves only as players act on the news, and the ones who read it first
   and right take the money.
7. **The end.** Everything left open settles in the final auction. The final ranking is net worth
   after that auction. The round summary shows who bought tips on what, and at what times you
   were traded against by someone holding one.

What a player is thinking about the whole time: what is this worth, who's in it, when will they
leave, what do I know that they don't, and where should I stand to see it.

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
On its own, parking in the deepest pool is a safe substitute for cash (see "What changed"). E is
what makes every pool risky. A needs E.

**Knobs.**
- **Rate.** It has to beat a round trip in the deepest pool, or holding cash is better than
  holding anything. It also has to stay small enough that walking to an information station, or
  waiting ten seconds for a signal, is still reasonable, or the round's start becomes a rush.
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

**Mechanism.** Trades no longer fill the moment they arrive. The server collects orders for
`clearInterval` (say 1 s, 20 ticks), then clears each commodity in one step:

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

**What it does.**
- **Speed stops mattering inside a window.** A run for the door clears in one batch at one price.
  Being 50 ms faster than someone else buys nothing. This is the frequent-batch-auction result
  (Budish, Cramton & Shim, 2015), and it's the single biggest lever for "skill over reflexes".
- **Margin calls fire at the next clear, not the next tick,** so a cascade unfolds over seconds,
  in steps a human can watch and act in.
- **Players trade with each other directly.** Buys and sells in the same batch match against each
  other at `P`, and only the difference touches the pool. An informed buyer and a forced seller
  in the same batch are trading with each other. This is what makes E mostly player vs player.
- **Latency parity comes for free.** A bot and a human who both decide inside the same window get
  the same price.

**Feel.** The cost is the click-and-instant-fill feel. To keep trading snappy:
- A visible countdown to the next clear on every station ("clears in 0.8 s").
- Your pending order shown on screen until it clears, then the fill animated at `P`.
- An **imbalance bar**: the pending buy/sell imbalance per commodity during the window, public.
  This is a new thing to read, and because orders are irrevocable it can't be spoofed: showing
  size costs real exposure. Without irrevocability, the bar would reward fake orders pulled on the
  last tick, which is a reflex race again.

**Knobs.**
- **`clearInterval`.** Longer is calmer and fairer; shorter feels more responsive. 0.5–2 s.
- **Imbalance bar.** Public, delayed, coarse (direction only), or off.
- **Cross-commodity.** All commodities clear on the same tick, or staggered. Same tick is easier
  to reason about.

**Protocol.** A trade becomes two steps: the server acknowledges the order into the batch, then
answers it with the `TradeReceipt` when the batch clears. The receipt keeps its meaning: the whole
order filled at one per-unit price, or was rejected with a reason. A new `ServerMessage` member
acknowledges a pending order. `PriceQuote` gains the time to the next clear and, if the knob is on,
the pending imbalance.

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
- **Active commodities.** Not every commodity has to be live every round. Four out of six keeps
  the round manageable (see "Manageable" in the principles).

**Alternative routes.**
- **Settle everything at once, periodically.** Simpler, but the whole map resets together, and
  there's never a "somewhere else is about to settle" to plan around.
- **Rolling over positions.** Positions carry past a settlement for a fee instead of being closed.
  Softer; it weakens the crowding penalty.
- **Settle holders one by one in random order.** Easier to implement, but where you land in the
  order is luck. Rejected.

**Scoring and the UI during the round.** Net worth shown mid-round is what a settlement right now
would pay at the current pool price: each commodity's `net` traded as one order. Because open
interest is public, this mark leaks nothing new, and it doesn't flatter crowded positions. It
can't include `V`, which is unknown; the UI says so ("marked at today's price, before the
reveal"). The same value, with `V` substituted where the training env knows it, is the bot's
per-step reward (see "Training the trading bot").

**Protocol.** New `ServerMessage` members for a settlement being scheduled and its result (`V`,
`P`, `net`, the player's own proceeds). Each `PriceQuote` gains the time to its commodity's next
settlement and its open interest (total long, total short).

### E. Settlement value

**Mechanism.** Each commodity has a hidden settlement value `V` for its next settlement. At the
settlement tick (B, step 1), the pool is re-priced: `cashReserve` is set so that the marginal
price equals `V`, keeping `unitReserve`. Then the netted order trades on the curve from there.

- `V` for each settlement is drawn when the previous one ends: a step from the current price, of
  a size the knobs control. A commodity's `V` is not known to anyone until the reveal.
- **Signals** tell players about `V` before the reveal: a public signal, noisy and vague, for
  everyone; bought tips (2) for some; staged events (3) that shift `V` and announce it in stages.
- Between settlements, the pool price moves only when players trade. **The game never moves the
  price directly.** It only sets `V` and tells people about it.

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
- **Reading others matters more, not less.** Price is now a mix of `V` and crowd noise, and the
  open interest shows how much crowd there is. "The price is above the signal *and* the crowd is
  45 long" is a trade. "Somebody just bought a tip and silver's imbalance flipped" is a trade.

**The honest cost.** Some money now comes from the game: the pool loses to informed trading at the
reveal. That's player vs game, like events today. It's acceptable because the pool's loss is
captured by whoever read the signals first and best, at the expense of those who didn't; rank is
relative, so what matters is who gets it. The harness should measure how much of the profit comes
from the pool and how much from other players (see "Validation").

**Knobs.**
- **Size of `V`'s step.** Relative to the spread in each pool. Too small and E doesn't matter; too
  large and the reveal is a lottery. It can scale with depth, or not.
- **Public signal quality.** How noisy, how vague (direction only, or a range), how early.
- **Blend.** The reveal can re-price the pool fully to `V`, or a fraction of the way. A fraction
  keeps more of the outcome in players' hands, but makes `V` harder to explain.

**Alternative routes.**
- **Settle at `V` directly, without the pool.** Pay every position `V`, and the pool (the house)
  absorbs the net at `V`. Simpler, but removes the crowding penalty entirely. Rejected.
- **No reveal; only events.** Keep the first edition's flow-only market and rely on events for
  value. The dead middle and the parking lot come back when no event is running.

**Protocol.** The settlement result carries `V`. A public signal is a new `ServerMessage` member
(or a tip kind in 2's payload, sent to everyone).

### C. Short selling

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
| **A value** | "Silver's settlement value is likely below $9.50" | Mostly: about `V` (E), but the profit is in trading before the others work it out, against those who don't. |
| **A position** | "Player X is long 30 lithium" | Fully: it's only about other players. Open interest says how much crowd there is; the tip says who. |
| **The schedule** | "Silver's next settlement is at 2:40", before it's public | Mostly: the profit is in acting before the others. |
| **An event** | "Shortage in oil in about 15 s" | Partly: the edge comes from the game; see 3. |

**What it does.** It creates the informed-trader game. The tipped player profits only if they
trade without giving the tip away, and everyone else profits by spotting who got it from how they
move and trade. Buying is a decision rather than luck: the price, the walk to the station, and
the notice that tells everyone you're informed are all costs. It's also a cash sink that pairs
with A.

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

**Mechanism.** Today the server's stations sit on a 25×16 ellipse around the centre
(`NewTradingStations`) and `DefaultFOVRadius` is 37.5. From one station you can't see the opposite
one, but from the centre you see almost everything. Moving the ring out to a radius of around
60–80 means that from any station you see only that station and its neighbours.

Within field of view, **every trade shows as a flash at the station**, attributed to the player
who made it. This is on by default, not a knob: it's the core signal for reading other players.

**What it does.**
- Seeing who is trading where becomes information you earn by being there. `MarketState` still
  goes to everyone, so trades are visible as anonymous price moves and open interest; the map
  hides who made them.
- Where you stand becomes a decision. With 1 you can't be at every settling commodity, and with C
  you can only attack where you are. The information station (2) is one more place to be.
- **Bluffing becomes possible.** Standing at a station, or a small buy-then-sell to fake interest,
  costs only the spread and some time. Cheap bluffs, answered by reading, are good skill
  expression.
- It feeds 2: a position tip is worth more when you can't just look.

**Costs.** At `MoveSpeed` 15.6, crossing a ring of radius 70 takes about 9 s, roughly 3% of a
5-minute round. Much further and the game starts being about walking.

**Knobs.**
- **Ring radius, or FOV radius.** A smaller FOV does the same thing without moving stations, but
  also changes how the game looks.
- **`PriceQuote.dominant_whale_id`.** Already in the protocol and unused. Filling it with the
  largest holder makes positions public and undoes most of the point of this feature and of
  position tips. It stays empty.

**Alternative routes.**
- **Hubs:** clusters of two or three stations with distance between clusters. You watch a whole
  hub at once, and choose which hub to be at.
- **Scouting tools:** a way to see a distant station briefly, for a price.

**For the bot.** Other players drop out of view more often, so the observation needs memory of
who was last seen where. Pathing (Stage 1b/1c) matters more on a wider map.

## Skill vs randomness

| Source | Luck or skill | What keeps it from being luck |
|---|---|---|
| Spawn and the start of the round | Luck plus reflexes | Low decay rate (A), several settlement cycles (1) so the first one is a small part of the round, D |
| Settlement timing | Skill | A schedule that is known or announced, a blackout window |
| Settlement value (E) | Luck without signals, skill with them | Public signals, bought tips, a step size that isn't a lottery, several settlements per round |
| Crowding | Skill | Public open interest makes it readable; 4 makes who's crowded costly to learn |
| Short squeezes | Skill with risk | Visible margin state in the UI, a margin multiple that isn't brutal, D making cascades step by step |
| Who gets tips | Skill | Bought, not given (2) |
| Events | Skill with 3 | Staging, events moving `V` not price |
| Reaction time | Nothing, within a batch | D |

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
| Settlement value (E) | Uninformed → informed, in batches; pool → informed, for the part that didn't cross | Mostly player vs player |
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
- **Self-play has something to learn.** With B + 1 and C, any position can be punished, so the
  best play depends on what the others do. With E there's also a right answer to learn about
  each commodity, which gives the gradient something stable to climb before the opponent game
  matters.
- **Latency is a non-issue.** D makes decisions inside a batch equal; the bot gains nothing from
  its zero reaction time.
- **Training finds exploits.** PPO is good at finding degenerate strategies. Short runs are an
  automated playtest: if the bot converges on something dull, humans would have found it too.

**What gets harder.**
- **The payout is lumpy.** Settlement pays at specific ticks. The per-step reward should be the
  change in net worth marked at what a settlement right now would pay, including `V`, which the
  env knows even though the policy doesn't. The reward may use information the observation can't.
  Only the policy is bound by what a human would know.
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
- Open interest per commodity, and the batch imbalance if it's public.
- Time to the next clear, to each commodity's next settlement, and to the end of the round.
- The public signal about each commodity's `V`.
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

**Latency parity.** D handles it within a batch. The bot's decision for a batch still has to be
made before the batch closes, on the same observation a human has, and no later.

**Staging the curriculum.**
1. Speculation: A, B + 1, C, D, E with the public signal only, and 4. No tips, no events. This is
   the question Stage 2 was asking: can the bot beat other traders by trading alone?
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
  shelter from decay; the first edition's blind spot.
- **Lone holder:** claims an unclaimed commodity and holds it.
- **Early rotator:** gets into a commodity before others, leaves before settlement.
- **First-out exit racer:** sits in crowds and sells on the first sign of anyone else selling.
  Pure reflex; it should gain nothing once D is on.
- **Signal follower:** trades on the public signal about `V` (once E exists).
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
- **Reflex sensitivity:** each strategy against a copy of itself delayed by 5 ticks. The gap is
  how much reaction time is worth. It should be close to zero.
- **Where profit comes from:** how much of the winners' profit came from other players, and how
  much from the pool at reveals. If it's mostly the pool, E is too generous and the game has
  drifted to player vs game.
- **When money is made:** profit and loss over the round. If it's all at tick 0 and the end, the
  middle is dead.

**Gates.**
- Idle, buy-and-hold, the oil parker and the lone holder rank below the median, in every world size
  with at least one competent strategy in it.
- The early rotator, the signal follower and the short attacker win.
- The first-out exit racer beats its 5-tick-delayed copy by less than the seed variance, with D on.
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

## Rollout

1. **The harness, on the current rules.** Measures the problem before anything changes it.
2. **D, batch clearing.** It changes the trade path everything else builds on, so it goes first.
   Client: pending orders, countdown, the fill animation.
3. **A + B** (the final settlement only). Small: decay on balances, and the close in `phase.go`.
   Tune the decay rate with the harness. Expected failures: the lone holder, which C fixes, and
   the oil parker, which E fixes. **Playtest.**
4. **C.** Protocol, rules, UI and `PRICING.md`. The decisions under C come first.
5. **1 + E**, settlements during the round with settlement values and the public signal. Open
   interest in `PriceQuote`. **Playtest.**
6. **4**, the map, with trades visible in field of view. Unity camera and layout; retrain Stage 1c
   pathing if the station ring moves.
7. **2**, bought tips and the information station. **Playtest.**
8. **3**, staged events, reworked to move `V`.
9. **Stage 2 bot training**, following the curriculum above, with short PPO runs from step 4 on
   as exploit probes.

## Open questions

- **Decay rate, settlement count, `V`'s step size and signal quality.** Numbers for the harness to
  find.
- **Margin calls or capped losses.** Squeezes are the fun of shorting, but they're also where new
  players lose badly without understanding why. D makes them slower and more visible; is that
  enough?
- **Does short collateral decay?** It decides which way the market leans.
- **`clearInterval` and the imbalance bar.** How long a batch can be before trading feels sluggish,
  and whether the bar is public, delayed, or off.
- **Collusion.** In a game where money moves between players, two friends, or two accounts, can
  pump and dump into each other: one buys early, the other buys late on purpose to feed them. In
  an MMO this will happen. Options: ranked lobbies filled at random with no chosen parties,
  reporting trades that consistently lose to the same counterparty, and rank rewards that make
  feeding a friend worth less than playing. Nothing is decided.
- **Newcomers fund the veterans.** In a crowd game, new players are structurally the late buyers
  and the uninformed flow. Beyond "the random trader loses, but not too hard": matchmaking by
  rating, lobby sizing, or a practice mode against scripted strategies before ranked play.
- **Active commodities per round.** All six, or a rotating subset to keep the round manageable.
- **Mean reversion as a fallback.** A pool that relaxes towards an anchor price between trades
  also kills lone holding, without shorting. It's the house moving money, so it's kept as a tuning
  tool if C, E and 1 leave a passive strategy standing. With C it needs care: a pump that fades on
  its own makes shorting it nearly free money. E already re-anchors at every reveal, which may be
  enough.
