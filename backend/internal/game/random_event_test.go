package game

import (
	"math/rand"
	"slices"
	"testing"
	"time"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
)

func TestEventTimesBunchUpTowardsTheEnd(t *testing.T) {
	const round = 5 * time.Minute
	times := slices.Collect(EventTimes(round))

	if len(times) != eventCurve-1 {
		t.Fatalf("got %d events, want %d", len(times), eventCurve-1)
	}
	if times[len(times)-1] >= round {
		t.Fatalf("last event at %v, not inside the %v round", times[len(times)-1], round)
	}
	for i := 1; i < len(times); i++ {
		if times[i] <= times[i-1] {
			t.Fatalf("event %d at %v, not after %v", i, times[i], times[i-1])
		}
		if i > 1 && times[i]-times[i-1] >= times[i-1]-times[i-2] {
			t.Fatalf("gap before event %d is %v, not shorter than the %v before it", i, times[i]-times[i-1], times[i-1]-times[i-2])
		}
	}
}

// Events only ever start at scheduled ticks: one still running when the next comes due never
// holds that one back to start the moment it ends.
func TestEventsStartOnlyOnSchedule(t *testing.T) {
	const round = 5 * time.Minute
	w := NewWorld("test", WorldConfig{Rng: rand.New(rand.NewSource(1)), RandomEvents: true, Duration: round})
	w.phase = pb.GamePhase_GAME_PHASE_RUNNING
	w.scheduleEvents()

	scheduled := map[uint64]bool{}
	for at := range EventTimes(round) {
		scheduled[w.tick+ticks(at)] = true
	}
	starts := 0
	for end := w.tick + ticks(round); w.tick < end; w.tick++ {
		before := w.nextEventID
		w.randomEvents()
		if w.nextEventID != before {
			starts++
			if !scheduled[w.tick] {
				t.Fatalf("an event started at tick %d, which is not a scheduled time", w.tick)
			}
		}
	}
	if starts == 0 {
		t.Fatal("no event started")
	}
}

// Once every free slot conflicts with a running event, chooseEvent gives up instead of
// retrying forever and hanging the tick.
func TestChooseEventReturnsNilWhenOnlyConflictsRemain(t *testing.T) {
	w := testWorld(t)
	w.Stations = w.Stations[:1]
	c := w.Stations[0].Commodity
	w.activeEvents[1] = &RandomEvent{commodity: c, eventType: pb.RandomCommodityEventType_RANDOM_COMMODITY_EVENT_CLOSED}
	w.activeEvents[2] = &RandomEvent{commodity: c, eventType: pb.RandomCommodityEventType_RANDOM_COMMODITY_EVENT_SUPPLY_SHORTAGE}

	if e := w.chooseEvent(); e != nil {
		t.Fatalf("chose %v, want nil: SANCTIONED conflicts with CLOSED and SUPPLY_FLOOD with SUPPLY_SHORTAGE", e.eventType)
	}
}

// Conflicts hold in both orders: whichever of a pair runs first keeps the other from starting.
func TestCanStartConflictsBothWays(t *testing.T) {
	for _, pair := range conflicting {
		for _, order := range [][2]pb.RandomCommodityEventType{{pair[0], pair[1]}, {pair[1], pair[0]}} {
			w := testWorld(t)
			c := w.Stations[0].Commodity
			w.activeEvents[1] = &RandomEvent{commodity: c, eventType: order[0]}

			if w.canStart(c, order[1]) {
				t.Errorf("%v started while %v runs on the same commodity", order[1], order[0])
			}
		}
	}
}

// poolEventTypes are the events that trade with the pool
var poolEventTypes = []pb.RandomCommodityEventType{
	pb.RandomCommodityEventType_RANDOM_COMMODITY_EVENT_SUPPLY_FLOOD,
	pb.RandomCommodityEventType_RANDOM_COMMODITY_EVENT_SUPPLY_SHORTAGE,
}

// priceMove is how far now is from start, in basis points of start, either way
func priceMove(start, now uint64) uint64 {
	ratio := now * wholeBasisPoints / start
	if ratio < wholeBasisPoints {
		return wholeBasisPoints - ratio
	}
	return ratio - wholeBasisPoints
}

// Each flood or shortage rolls a cap inside its commodity's range, gets close to it by the time
// the event ends, and never past it.
func TestPoolTradeStopsAtItsCap(t *testing.T) {
	for _, eventType := range poolEventTypes {
		for _, c := range GetCommodityTypes() {
			w := testWorld(t)
			w.tick = 7
			pool := w.Commodities[c]
			start := pool.buyPrice(1)
			e := newRandomEvent(c, eventType, w.tick, w.config.Rng)
			if caps := capsFor(c); e.capBasisPoints < caps.min || e.capBasisPoints > caps.max {
				t.Errorf("%v on %v: rolled a %d bp cap, outside %d-%d", eventType, c, e.capBasisPoints, caps.min, caps.max)
			}
			w.activeEvents[1] = e

			// From the tick the event starts on, which need not be a trade tick, until it ends:
			// the ticks Tick runs poolTrade on while the event is active
			for ; w.tick < e.endTick; w.tick++ {
				w.poolTrade()
			}

			moved := priceMove(start, pool.buyPrice(1))
			if moved > e.capBasisPoints {
				t.Errorf("%v on %v: price moved %d bp, past its %d bp cap", eventType, c, moved, e.capBasisPoints)
			}
			if moved < e.capBasisPoints*9/10 {
				t.Errorf("%v on %v: price moved %d bp, short of its %d bp cap", eventType, c, moved, e.capBasisPoints)
			}
		}
	}
}

// A flood or shortage makes most of its move the tick it starts, even off the drift schedule,
// and about its front-loaded share of the cap.
func TestPoolTradeFrontLoadsTheMove(t *testing.T) {
	for _, eventType := range poolEventTypes {
		for _, c := range GetCommodityTypes() {
			w := testWorld(t)
			w.tick = 7 // not a drift tick
			pool := w.Commodities[c]
			start := pool.buyPrice(1)
			e := newRandomEvent(c, eventType, w.tick, w.config.Rng)
			w.activeEvents[1] = e

			w.poolTrade()

			moved := priceMove(start, pool.buyPrice(1))
			jump := e.capBasisPoints * frontLoadBasisPoints / wholeBasisPoints
			// Quotes are whole cents and a unit's price rounds up, so on a ~$10 price the measured
			// move can land a cent or two (10-20 bp) off the target the units were worked out for
			if moved > jump+20 || moved < jump*85/100 {
				t.Errorf("%v on %v: jumped %d bp, want close to %d bp and not past it", eventType, c, moved, jump)
			}
		}
	}
}
