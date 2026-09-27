package game

import (
	"time"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
)

const RandomEventDuration = time.Second * 15
const DelayBetweenEvents = time.Second * 15

type RandomEvent struct {
	commodity pb.CommodityType
	eventType pb.RandomCommodityEventType
	endTick   uint64
}

// chooseEvent picks a commodity that has a station, and any event type but UNSPECIFIED
func (w *World) chooseEvent() *RandomEvent {
	rng := w.config.Rng
	station := w.Stations[rng.Intn(len(w.Stations))]
	eventType := pb.RandomCommodityEventType(1 + rng.Intn(len(pb.RandomCommodityEventType_name)-1))

	return &RandomEvent{
		endTick:   w.tick + ticks(RandomEventDuration),
		commodity: station.Commodity,
		eventType: eventType,
	}
}

func (w *World) startEvent() {
	e := w.chooseEvent()

	w.activeEvent = e
	remainingMs := int32(float64(e.endTick-w.tick) * TickDuration * 1000)

	w.sendToAll(&pb.ServerMessage{
		Msg: &pb.ServerMessage_RandomEffectOccurred{
			RandomEffectOccurred: &pb.RandomEventOccurred{
				EventType:   e.eventType,
				Commodity:   e.commodity,
				RemainingMs: remainingMs,
			},
		},
	})
}

func (w *World) endEvent() {
	e := w.activeEvent
	w.activeEvent = nil
	w.nextEventTick = w.tick + ticks(DelayBetweenEvents)
	w.sendToAll(&pb.ServerMessage{
		Msg: &pb.ServerMessage_RandomEventEnded{
			RandomEventEnded: &pb.RandomEventEnded{
				EventType: e.eventType,
				Commodity: e.commodity,
			},
		},
	})
}

// eventOn reports whether an event of type t is running on commodity c
func (w *World) eventOn(c pb.CommodityType, t pb.RandomCommodityEventType) bool {
	e := w.activeEvent
	return e != nil && e.commodity == c && e.eventType == t
}
