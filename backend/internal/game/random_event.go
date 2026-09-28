package game

import (
	"iter"
	"math"
	"math/rand"
	"time"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
)

var EventDuration = map[pb.RandomCommodityEventType]time.Duration{
	pb.RandomCommodityEventType_RANDOM_COMMODITY_EVENT_SANCTIONED:      time.Second * 30,
	pb.RandomCommodityEventType_RANDOM_COMMODITY_EVENT_CLOSED:          time.Second * 15,
	pb.RandomCommodityEventType_RANDOM_COMMODITY_EVENT_SUPPLY_FLOOD:    time.Second * 20,
	pb.RandomCommodityEventType_RANDOM_COMMODITY_EVENT_SUPPLY_SHORTAGE: time.Second * 20,
}

// capRange bounds how far a flood or shortage may move a commodity's price by itself, either
// way; each event rolls its cap in between, inclusive. Both must stay under 10_000.
type capRange struct {
	min, max uint64 // basis points: 1_500 lets the price fall to 85% of where the event found it, or rise to 115%
}

// eventCaps is each commodity's cap range; a commodity missing here gets defaultEventCaps
var eventCaps = map[pb.CommodityType]capRange{
	pb.CommodityType_COMMODITY_OIL:     {1_000, 2_000},
	pb.CommodityType_COMMODITY_WHEAT:   {1_500, 2_500},
	pb.CommodityType_COMMODITY_COFFEE:  {2_000, 3_000},
	pb.CommodityType_COMMODITY_GOLD:    {2_000, 3_000},
	pb.CommodityType_COMMODITY_SILVER:  {2_500, 3_500},
	pb.CommodityType_COMMODITY_LITHIUM: {3_000, 4_000},
}

var defaultEventCaps = capRange{2_000, 3_000}

// capsFor is commodity c's cap range
func capsFor(c pb.CommodityType) capRange {
	if caps, ok := eventCaps[c]; ok {
		return caps
	}
	return defaultEventCaps
}

// roll picks a cap between min and max, inclusive
func (r capRange) roll(rng *rand.Rand) uint64 {
	return r.min + uint64(rng.Int63n(int64(r.max-r.min+1)))
}

const poolTradeTickFrequency = 10

// frontLoadBasisPoints is the share of its cap a flood or shortage moves the price by the moment
// it starts, as the news lands; the rest drifts in over the event. Buying on the announcement
// already pays most of the move.
const frontLoadBasisPoints = 7_000

// wholeBasisPoints is 100%
const wholeBasisPoints = 10_000

// eventCurve shapes the schedule: a round gets eventCurve-1 events, and a higher value spreads them closer together
const eventCurve = 14

// EventTimes yields when each event of a round starts, measured from the round's start. The
// gaps begin near round/7 and shrink steadily, so events bunch up towards the end.
func EventTimes(round time.Duration) iter.Seq[time.Duration] {
	return func(yield func(time.Duration) bool) {
		for n := 1; n < eventCurve; n++ {
			x := 1 - float64(n)/eventCurve
			if !yield(time.Duration(float64(round) * (1 - x*x))) {
				return
			}
		}
	}
}

const ExchangeFeeBasisPoints = 1000 // 10%

// exchangeFee is the exchange's cut of total, rounded up so it never undercharges
func exchangeFee(total uint64) uint64 {
	return (total*ExchangeFeeBasisPoints + 9_999) / 10_000
}

type RandomEvent struct {
	commodity        pb.CommodityType
	eventType        pb.RandomCommodityEventType
	startTick        uint64
	endTick          uint64
	movedBasisPoints uint64 // the event's own price move so far: wholeBasisPoints = none, half that = halved
	capBasisPoints   uint64 // how far movedBasisPoints may get from wholeBasisPoints, either way
}

// conflicting are the event pairs that never run on the same commodity at once, in either order
var conflicting = [][2]pb.RandomCommodityEventType{
	{pb.RandomCommodityEventType_RANDOM_COMMODITY_EVENT_SUPPLY_FLOOD, pb.RandomCommodityEventType_RANDOM_COMMODITY_EVENT_SUPPLY_SHORTAGE},
	{pb.RandomCommodityEventType_RANDOM_COMMODITY_EVENT_SANCTIONED, pb.RandomCommodityEventType_RANDOM_COMMODITY_EVENT_CLOSED},
}

// canStart reports whether an event of type t may start on commodity c: not already running
// there, and not in conflict with one that is
func (w *World) canStart(c pb.CommodityType, t pb.RandomCommodityEventType) bool {
	if w.eventOn(c, t) {
		return false
	}
	for _, pair := range conflicting {
		if (pair[0] == t && w.eventOn(c, pair[1])) || (pair[1] == t && w.eventOn(c, pair[0])) {
			return false
		}
	}
	return true
}

// chooseEvent picks a commodity that has a station, and any event type but UNSPECIFIED that
// can start on it; nil if none can
func (w *World) chooseEvent() *RandomEvent {
	type candidate struct {
		commodity pb.CommodityType
		eventType pb.RandomCommodityEventType
	}

	// The types after UNSPECIFIED, all of which have an effect
	types := len(pb.RandomCommodityEventType_name) - 1
	var candidates []candidate
	for _, station := range w.Stations {
		for i := 1; i <= types; i++ {
			t := pb.RandomCommodityEventType(i)
			if w.canStart(station.Commodity, t) {
				candidates = append(candidates, candidate{station.Commodity, t})
			}
		}
	}
	if len(candidates) == 0 {
		return nil
	}

	c := candidates[w.config.Rng.Intn(len(candidates))]
	return newRandomEvent(c.commodity, c.eventType, w.tick, w.config.Rng)
}

// newRandomEvent starts an event of type t on commodity c at tick, rolling its cap from c's range
func newRandomEvent(c pb.CommodityType, t pb.RandomCommodityEventType, tick uint64, rng *rand.Rand) *RandomEvent {
	return &RandomEvent{
		startTick:        tick,
		endTick:          tick + ticks(EventDuration[t]),
		commodity:        c,
		eventType:        t,
		movedBasisPoints: wholeBasisPoints,
		capBasisPoints:   capsFor(c).roll(rng),
	}
}

// priceFactor is the price multiple a flood or shortage stands at after moving share (basis
// points) of its cap: below 1 for a flood, above 1 for a shortage
func (e *RandomEvent) priceFactor(share uint64) float64 {
	move := float64(e.capBasisPoints) * float64(share) / (wholeBasisPoints * wholeBasisPoints)
	if e.eventType == pb.RandomCommodityEventType_RANDOM_COMMODITY_EVENT_SUPPLY_FLOOD {
		return 1 - move
	}
	return 1 + move
}

// target is the price multiple a flood or shortage aims to have made by tick: its front-loaded
// share of the cap at once, then the rest in equal steps until it ends
func (e *RandomEvent) target(tick uint64) float64 {
	jump := e.priceFactor(frontLoadBasisPoints)
	progress := float64(tick-e.startTick) / float64(e.endTick-e.startTick)
	return jump * math.Pow(e.priceFactor(wholeBasisPoints)/jump, progress)
}

// unitsToward is how many units to trade with a pool of units to scale its price by
// target/moved. Price goes as 1/units², so the pool must end at units·√(moved/target); rounded
// down, so it never goes past.
func unitsToward(units uint64, moved, target float64) uint64 {
	return uint64(math.Abs(float64(units)*math.Sqrt(moved/target) - float64(units)))
}

func (w *World) startNextEvent() {
	e := w.chooseEvent()
	if e == nil {
		return // no more room for events
	}

	w.nextEventID++
	w.activeEvents[w.nextEventID] = e
	remainingMs := int32(float64(e.endTick-w.tick) * TickDuration * 1000)

	w.sendToAll(&pb.ServerMessage{
		Msg: &pb.ServerMessage_RandomEventOccurred{
			RandomEventOccurred: &pb.RandomEventOccurred{
				EventType:   e.eventType,
				Commodity:   e.commodity,
				RemainingMs: remainingMs,
			},
		},
	})
	w.logEvent("event_start", w.nextEventID, e)
}

// scheduleEvents lists the ticks this round's events start at, from now
func (w *World) scheduleEvents() {
	w.eventTicks = w.eventTicks[:0] // empty slice but keeps storage
	for at := range EventTimes(w.config.Duration) {
		w.eventTicks = append(w.eventTicks, w.tick+ticks(at))
	}
}

func (w *World) endEvent(eventID uint64) {
	e, exists := w.activeEvents[eventID]
	if exists {
		w.sendToAll(&pb.ServerMessage{
			Msg: &pb.ServerMessage_RandomEventEnded{
				RandomEventEnded: &pb.RandomEventEnded{
					EventType: e.eventType,
					Commodity: e.commodity,
				},
			},
		})
		w.logEvent("event_end", eventID, e)
		delete(w.activeEvents, eventID)
	}
}

func (w *World) endAllEvents() {
	for id, _ := range w.activeEvents {
		w.endEvent(id)
	}
}

// eventOn reports whether an event of type t is running on commodity c
func (w *World) eventOn(c pb.CommodityType, t pb.RandomCommodityEventType) bool {
	for _, e := range w.activeEvents {
		if e.commodity == c && e.eventType == t {
			return true
		}
	}
	return false
}
