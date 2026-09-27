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
