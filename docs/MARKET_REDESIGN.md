# Market Redesign: Proposal

**Status: proposal. Nothing here is implemented.** This changes the rules in `PRICING.md`, and
it replaces the market that Stage 2 of `BOT_TRAINING.md` assumes. Stage 2 should not start
until the validation gates at the end of this document pass.

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

With events off, which is how the trading bot's first stage should train, this is the
no-trade theorem (Milgrom & Stokey, 1982): traders with the same information have no reason to
speculate against each other, because whoever takes the other side of your trade is evidence
that you are wrong. Real markets trade because some participants have to (hedgers, savers,
people who need cash) and because some know things others don't. The proposal below adds both,
through rules that players act on, not through outside NPC traders.

## Design principles

- **Players compete against each other.** Money should mostly move between players. The game
  may push players (to act, to take risk), but outside traders with their own agenda are
  rejected, because they dilute the sense of beating other people.
- **No safe passive strategy.** Idling, buy-and-hold, and quietly owning one commodity alone
  must all lose to competent play.
- **Skill over luck.** Outcomes should be repeatable. The better strategy should win across
  many seeds by more than seeds differ from each other.
- **Skill over reflexes.** A race decided by reaction time is won by a bot with zero latency.
  The game should reward reading other players and choosing when to act.
- **Legible.** A new player has to understand why they lost money.

## The features at a glance

| | Feature | Breaks | Player vs player, or player vs game |
|---|---|---|---|
| **A** | Cash loses value | Idling is safe | The game applies it to everyone equally; it just forces players to act |
| **B** | Settlement auction | Crowding is free | Player vs player: the crowd prices its own exit |
| **C** | Short selling | A lone holder can't be touched; prices only go one way | Player vs player |
| **1** | Staggered settlements per commodity | The round's middle is dead | Player vs player (turns B from once a round into a rhythm) |
| **2** | Private tips | Everyone sees the same information | Player vs player: the edge exists only because others don't know |
| **3** | Staged events | Events reward luck, not play | Mixed: the game supplies the news, players fight over it |
| **4** | Stations spread apart | Everyone sees everyone | Player vs player: information depends on where you stand |

**B is a special case of 1**: one settlement, at the end of the round. The rest of this document
treats them as one feature. The final settlement at the end of the round always exists; 1 adds
more during the round.

## How a round plays

Here is a 5-minute round with all features on, from one player's point of view.

1. **Start.** Everyone spawns at the centre with $1000. The stations sit on a wide ring, so from
   the centre you see only part of it. The settlement schedule shows that lithium settles first,
   at 1:10, gold at 1:45, and so on. Cash starts losing value on the first tick.
2. **Getting into positions.** You can't sit in cash, but going to the obvious commodity with
   everyone else is how a crowd gets hurt at settlement. You pick silver, which settles later
   and is thinner, and walk there. On the way you see two players heading to lithium.
3. **Before the first settlement.** Lithium's price is high, because a crowd is in. A private tip
   tells you that one of the players you saw is long 30 units. You walk over and short
   lithium. You're betting the crowd leaves before settlement, which pushes the price down, and
   you cover in the collapse.
4. **Settlement.** At 1:10, every open lithium position closes in one combined order at one price.
   Those who left early got out near the top; those who stayed get the price that the crowd's
   own selling produces. Their cash starts decaying, so they have to re-enter somewhere.
5. **The middle of the round.** Cycles repeat, commodity by commodity. A public rumour says a
   shortage is coming to oil. Players who got the private version seconds earlier are already
   buying, and the price moves as they do. The event itself arrives after the players have
   mostly priced it in.
6. **The end.** Everything left open settles in the final auction. The final ranking is net worth
   after that auction.

What a player is thinking about the whole time: who's in what, when will they leave, what do I
know that they don't, and where should I stand to see it.

## Feature details

### A. Cash loses value

**Mechanism.** Every `decayInterval` (say one second), each player's cash balance shrinks by
`decayBasisPoints`. The amount lost is rounded up, so that cash stays whole cents and rounding
never favours the player, matching the pool's rule. The lost cash disappears. The ranking is by
net worth, so it doesn't matter where the money goes.

**What it does.**
- Idling now loses by a known amount, so any strategy that trades at a profit beats it.
- It makes every player a forced buyer. Everyone has to end up in some position, which is the
  "trader with a motive" pure speculation lacks. The motive comes from the players themselves,
  not from an outside trader.
- With 1, settlement turns positions back into cash, the cash decays, and players must re-enter,
  so the round becomes cycles instead of one rush.

**Knobs.**
- **Rate.** It has to beat the cost of a round trip in the deepest pool, or holding a position
  isn't better than cash. It also has to stay small enough that waiting ten seconds for better
  information is still reasonable, or the round's start becomes a reflex race (see "Skill vs
  randomness").
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

### B + 1. Settlement auctions

**Mechanism.** Every commodity has a settlement schedule. At a settlement tick, the server:

1. Sums every player's long units `L` and short units `S` in that commodity.
2. Trades the difference, `net = L − S`, with the pool as one order: a sell of `net` if it's
   positive, a buy of `−net` if it's negative. That order's per-unit price is `P`, from the same
   `sellPrice`/`buyPrice` the pool already uses, so rounding favours the pool as it does now.
3. Settles every position at `P`: longs receive `P` per unit, shorts pay `P` per unit.

Longs receive `L·P`, shorts pay `S·P`, and the pool pays `net·P`, so the cash balances exactly.
Longs and shorts are matched against each other at the pool's price, and only the difference
touches the pool. When `net` is 0, the pool doesn't trade, and `P` has to be defined another way;
the one-unit sell price is a reasonable choice. That case needs pinning down with a test.

After settlement the commodity reopens at whatever price the pool is left at.

**What it does.**
- A crowd pays for its own crowding. Everyone in the same commodity gets the same price, and the
  crowd as a whole roughly breaks even, since an AMM round trip loses only rounding. So entry
  order decides who wins: early buyers profit from the later buyers who lifted the price.
- Leaving before the others is the decision that matters, and it recurs at every settlement.
- Shorts are forced to cover at settlement, which creates squeezes on a known schedule.

**Knobs.**
- **Number of settlements per round.** Too many is chaos, too few brings back the dead middle.
- **Schedule.** A fixed calendar can be solved by a script. A random one, announced some time
  ahead, can't; how far ahead it is announced is itself a knob, and a candidate for private tips
  (2).
- **Blackout window.** No new positions in the last few seconds before a settlement, so it can't
  be dodged at the last tick with perfect reaction time. This makes reflexes matter less.
- **After settlement.** The pool keeps the price the auction left, or re-anchors towards the
  starting price. Re-anchoring makes every cycle comparable; keeping the price makes history
  matter.

**Alternative routes.**
- **Settle everything at once, periodically.** Simpler, but the whole map resets together, and
  there's never a "somewhere else is about to settle" to plan around.
- **Rolling over positions.** Positions carry past a settlement for a fee instead of being closed.
  Softer; it weakens the crowd penalty.
- **Settle holders one by one in random order.** Easier to implement, but where you land in the
  order is luck. Rejected.

**Scoring and the UI during the round.** Net worth shown mid-round should be what a settlement
right now would pay: each commodity's `net` traded as one order. Otherwise the display flatters
crowded positions. The same value is the bot's per-step reward (see "Training the trading bot").

**Protocol.** New `ServerMessage` members for a settlement being scheduled and its result (`P`,
`net`, the player's own proceeds), and each `PriceQuote` gains the time to its commodity's next
settlement.

### C. Short selling

**Mechanism.** Holdings become signed. Selling below zero opens a short: the sale proceeds are
credited as usual. A player's collateral (proceeds plus free cash) must always cover
`marginMultiple × |units| × buy price`. If a rising price breaks that, the server force-covers
the short at market: a buy into the pool, which pushes the price up further, which can break the
next short's margin. That cascade is the squeeze.

**Pool limit.** At settlement, net shorts become a buy from the pool, and the pool must keep at
least one unit, so `buyPrice` exists. A per-commodity cap on total open shorts, as a fraction of
`unitReserve`, keeps that buy fillable. With lithium's pool of 60 units, shorts in lithium stay
small, which makes it the commodity longs are safe in. That's worth knowing, not necessarily
worth fixing.

**What it does.**
- A lone holder is attackable: short into their pumped price, and at settlement the netting
  (their sell against your cover) brings the price back to about where they bought, and they
  lose what you gained.
- Every pumped price invites a bear; every crowded short invites a squeeze. Prices move both ways.

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

### 2. Private tips

**Mechanism.** Every so often, the server sends one player (or a few) a message only they see.
Tips are generated from the true game state, with deliberate noise when reliability is a knob.

| Tip | Example | How player vs player |
|---|---|---|
| **A position** | "Player X is long 30 lithium" | Fully: it's only about other players. You know who's crowded, so you can short them before settlement. |
| **The schedule** | "Gold settles 20 s earlier than announced" (if schedules can change) or "silver's next settlement is at 2:40" before it's public | Mostly: about the game's schedule, but the profit is in acting before the others work it out. |
| **An event** | "Shortage in oil in about 15 s" | Partly: the edge comes from the game; see 3. |

**What it does.** It creates the informed-trader game. The tipped player profits only if they
trade without revealing the tip, and everyone else profits by spotting who got it from how they
move and trade. Private information is what makes reading other players worth something at all.

**Knobs.**
- **Who gets tips.** At random, in rotation so everyone gets the same number, or for sale.
- **Price.** Buying a tip for cash (possibly at an information station on the map, which ties in
  with 4) is a decision rather than luck, and it's a cash sink that pairs with A.
- **Reliability.** Some tips wrong. Acting on a tip becomes a risk, and whoever is reading you
  can't be sure either.
- **Lifetime.** How long a tip stays true. A position tip goes stale as soon as the player trades.
- **Visibility of the fact.** Do others see that you got a tip, but not its content? That makes
  the recipient a target and a bluff possible.

**Alternative routes.**
- **Everyone gets the same number of tips, at different times.** Luck averages out over a round.
- **Tips as rewards** for a good trade, a settlement won, or scouting. Rich get richer; probably
  bad for newcomers.

**Fairness.** Humans see tips in the UI; the bot gets them as observation slots, with the same
timing. Neither gets them sooner.

**Protocol.** One new per-player `ServerMessage` member; its payload is a `oneof` over tip kinds.

### 3. Staged events

**Mechanism.** An event arrives in stages instead of all at once:

1. **Private tip** (2) to one or a few players, about 15 s before.
2. **Public rumour** about 7 s before. It could be vague on purpose: the commodity but not the
   direction, or the direction but not the commodity.
3. **The event** starts. `frontLoadBasisPoints` drops well below today's 70%, so the event's own
   move no longer lands at once.
4. **Drift** over the event's duration, as today.

**What it does.** Players price the event in before the game does: they buy during the warning,
and the latecomers pay them. The event becomes a race between players with a head start of
different sizes, instead of luck of already holding. It's still the game supplying the news, but
the money moves between players.

**Knobs.**
- **Stage timing and the front-load.**
- **False rumours.** A public rumour that doesn't come true punishes a crowd that trusts it
  blindly.
- **How vague each stage is.**

**The risk: a public warning alone becomes a reflex race.** Everyone sees it at the same moment, and
the fastest wins. Staging with a private stage first, and vague public stages, keeps it about
reading rather than reacting.

**For the bot.** Stage 2 training starts without events (see below). Events come back at this
feature.

### 4. Stations spread apart

**Mechanism.** Today the server's stations sit on a 25×16 ellipse around the centre
(`NewTradingStations`) and `DefaultFOVRadius` is 37.5. From one station you can't see the opposite
one, but from the centre you see almost everything. Moving the ring out to a radius of around
60–80 means that from any station you see only that station and its neighbours.

**What it does.**
- Seeing who is trading where becomes information you earn by being there. `MarketState` still
  goes to everyone, so trades are visible as anonymous price moves; the map hides who made them.
- Where you stand becomes a decision. With 1 you can't be at every settling commodity, and with C
  you can only attack where you are.
- It feeds 2: a private tip about a position is worth more when you can't just look.

**Costs.** At `MoveSpeed` 15.6, crossing a ring of radius 70 takes about 9 s, roughly 3% of a
5-minute round. Much further and the game starts being about walking.

**Knobs.**
- **Ring radius, or FOV radius.** A smaller FOV does the same thing without moving stations, but
  also changes how the game looks.
- **What's visible in FOV.** Players only, or also their trades (a flash at the station when
  someone trades).
- **`PriceQuote.dominant_whale_id`.** Already in the protocol and unused. Filling it with the
  largest holder makes positions public and undoes most of the point of this feature and of
  position tips. It should stay empty, or show only to players in FOV.

**Alternative routes.**
- **Hubs:** clusters of two or three stations with distance between clusters. You watch a whole
  hub at once, and choose which hub to be at.
- **Scouting tools:** a way to see a distant station briefly, for a price.

**For the bot.** Other players drop out of view more often, so the observation needs memory of
who was last seen where. Pathing (Stage 1b/1c) matters more on a wider map.

## Skill vs randomness

| Source | Luck or skill | What keeps it from being luck |
|---|---|---|
| Spawn and the start of the round | Luck plus reflexes | Low decay rate (A), several settlement cycles (1) so the first one is a small part of the round |
| Settlement timing | Skill | A schedule that is known or announced, a blackout window so it can't be dodged at the last tick |
| Crowding | Skill | Reading who's in, which 4 makes costly and 2 makes possible |
| Short squeezes | Skill with risk | Visible margin state in the UI, a margin multiple that isn't brutal |
| Who gets tips | Luck | Rotation or buying (2) |
| Events | Luck without 3, skill with it | Staging, a smaller front-load |
| Reaction time | Reflexes | Blackout windows, staging, latency parity for bots |

Two failure modes to watch for:
- **The start becomes a reflex race.** A decay rate that's too high forces everyone to buy on tick
  0. Keeping it low enough that waiting for information pays is the fix.
- **Luck dominates.** Measured, not argued: see "Validation".

## Player vs player, and player vs game

Where money comes from and goes, per feature:

| Flow | Direction | Feel |
|---|---|---|
| Pool spread and rounding | Players → pool | Player vs game, small and constant, as today |
| Cash decay (A) | Players → nowhere | Player vs game, but applied equally; it matters only relatively |
| Settlement (B + 1) | Late crowd → early crowd; longs ↔ shorts | Player vs player |
| Shorting and margin calls (C) | Losers of a squeeze → winners | Player vs player |
| Private tips (2) | Uninformed → informed | Player vs player |
| Events (3) | Pool ↔ players, then slow players → fast readers | Mixed |
| The map (4) | None directly | Player vs player, through information |

The game's own flows (spread, decay, events) set the pressure; almost every cent a player wins,
another player loses. That matches the aim of competing against each other rather than against
the game.

## Training the trading bot

These changes are what make Stage 2 learnable at all. Under the current rules, pure speculative
self-play has a stable outcome where nobody trades, and PPO would likely find it.

**What gets easier.**
- **No no-trade trap.** A costs reward every tick spent idle, so from the first episode the
  gradient pushes the bot into positions.
- **Self-play has something to learn.** With B + 1 and C, any position can be punished, so the
  best play depends on what the others do.
- **Training finds exploits.** PPO is good at finding degenerate strategies. Short runs are an
  automated playtest: if the bot converges on something dull, humans would have found it too.

**What gets harder.**
- **The payout is lumpy.** Settlement pays at specific ticks. The per-step reward should be the
  change in net worth marked at what a settlement right now would pay, not today's `netWorth`.
  That needs everyone's positions, which is allowed: the reward may use information the
  observation can't. Only the policy is bound by what a human would know.
- **High-variance rewards.** Margin cascades can wipe out a round. That needs reward normalisation,
  maybe clipping, and more samples per update.
- **Cycles become more likely.** Bulls, bears and squeezers have a rock-paper-scissors shape the
  long-only game didn't. The win-rate matrix and opponent sampling from `BOT_TRAINING.md` move
  from "measure first" to "probably needed".
- **Design churn.** Every rule change moves the optimum, and every observation change throws the
  policy away. No serious training until the rules settle.

**Observation additions.**
- Signed position per commodity, collateral and margin state.
- A short price history per commodity (an MLP sees only the present, and trends are the signal).
- Time to each commodity's next settlement, and time left in the round.
- Tip slots (2), empty when there's no live tip.
- Last-seen position and time for other players (4).
- Event stage information (3), once events return.

**Actions.**
- **Macro actions.** The policy picks a station and a signed position change; scripted steering
  walks there. That cuts the horizon from 6000 ticks to a few hundred decisions, which matters
  more than any other design choice here for credit assignment.
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

**Latency parity.** Margin calls and settlement exits turn on reaction time. Delay the bot's
actions by a few ticks, in the sim and in production alike, from the first run.

**Staging the curriculum.**
1. Pure speculation: A, B + 1, C and 4, no tips, no events. This is the question Stage 2 was
   asking: can the bot beat other traders by trading alone?
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
- **Lone holder:** claims an unclaimed commodity and holds it.
- **Early rotator:** gets into a commodity before others, leaves before settlement.
- **Trend follower.**
- **Random trader.**
- **Short attacker:** shorts lone holders and crowded pumps (once C exists).
- **Oracle:** cheats by knowing what the others will do next. It measures the skill ceiling.

**Metrics.**
- **Rank per strategy**, averaged over seeds, in 2-, 4- and 8+-player worlds.
- **Repeatability:** a strategy's advantage against its variance across seeds. If the variance
  is bigger, the game is mostly luck.
- **Skill ceiling:** how far the oracle beats the field. If even perfect knowledge of the others
  barely helps, reading other players is worthless.
- **When money is made:** profit and loss over the round. If it's all at tick 0 and the end, the
  middle is dead.

**Gates.**
- Idle, buy-and-hold and the lone holder rank last, in every world size.
- The early rotator and the short attacker win.
- The random trader loses, but not too hard, so new players aren't punished too much.
- The oracle clearly beats the field.
- Profit and loss spread over the round.

The same scripted strategies become the bot's baselines and first frozen opponents, so none of
this work is thrown away.

## Rollout

1. **The harness, on the current rules.** Measures the problem before anything changes it.
2. **A + B** (the final settlement only). Small: decay on balances, and the close in `phase.go`.
   Tune the decay rate with the harness. Expected failure: the lone holder, which C fixes.
3. **C.** Protocol, rules, UI and `PRICING.md`. The decisions under C come first.
4. **1**, settlements during the round.
5. **4**, the map. Unity camera and layout; retrain Stage 1c pathing if the station ring moves.
6. **2**, private tips.
7. **3**, staged events.
8. **Stage 2 bot training**, following the curriculum above, with short PPO runs from step 3 on
   as exploit probes.

## Open questions

- **Decay rate and settlement count.** Numbers for the harness to find.
- **Margin calls or capped losses.** Squeezes are the fun of shorting, but they're also where new
  players lose badly without understanding why.
- **Does short collateral decay?** It decides which way the market leans.
- **Tips: random, rotating or bought?**
- **How much others can see.** Positions private, shown in FOV, or public; this decides how much
  the reading-other-players game has to work with.
- **Does the pool re-anchor after settlement?** A fixed anchor makes cycles comparable but
  predictable.
- **Mean reversion as a fallback.** A pool that relaxes towards an anchor price between trades
  also kills lone holding, without shorting. It's the house moving money, so it's kept as a tuning
  tool if C and 1 leave a passive strategy standing. With C it needs care: a pump that fades on its
  own makes shorting it nearly free money.
