package game

import (
	"iter"
	"time"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
)

var EventDuration = map[pb.RandomCommodityEventType]time.Duration{
	pb.RandomCommodityEventType_RANDOM_COMMODITY_EVENT_SANCTIONED: time.Second * 30,
	pb.RandomCommodityEventType_RANDOM_COMMODITY_EVENT_CLOSED:     time.Second * 15,
}

// eventCurve shapes the schedule: a round gets eventCurve-1 events, and a higher value spreads
// them closer together
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
	commodity pb.CommodityType
	eventType pb.RandomCommodityEventType
	endTick   uint64
}

// chooseEvent picks a commodity that has a station, and any event type but UNSPECIFIED
func (w *World) chooseEvent() *RandomEvent {
	rng := w.config.Rng

	// The types after UNSPECIFIED that have an effect; the last two are not implemented yet
	types := len(pb.RandomCommodityEventType_name) - 1 - 2
	if len(w.activeEvents) >= len(w.Stations)*types {
		return nil
	}

	for {
		station := w.Stations[rng.Intn(len(w.Stations))]
		eventType := pb.RandomCommodityEventType(1 + rng.Intn(types))
		if w.eventOn(station.Commodity, eventType) {
			continue
		}

		return &RandomEvent{
			endTick:   w.tick + ticks(EventDuration[eventType]),
			commodity: station.Commodity,
			eventType: eventType,
		}
	}
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
