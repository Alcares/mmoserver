package game

import (
	"fmt"
	"math/rand"
	"slices"
	"testing"
	"time"
)

var queueStart = time.Unix(1_000_000, 0)

// queued makes a ticket for a player rated rating who has waited waited by now
func queued(rating float64, waited, now time.Duration) *ticket {
	return &ticket{account: Account{Rating: rating}, queuedAt: queueStart.Add(now - waited)}
}

func at(d time.Duration) time.Time { return queueStart.Add(d) }

func TestFullGroupFormsAtOnce(t *testing.T) {
	now := time.Second
	queue := []*ticket{queued(1000, now, now), queued(1010, now, now), queued(1020, now, now)}

	groups := formGroups(queue, at(now), 2, 3)
	if len(groups) != 1 || len(groups[0]) != 3 {
		t.Fatalf("want one group of 3, got %v", groups)
	}
}

func TestShortGroupWaitsForFill(t *testing.T) {
	now := fillWait - time.Second
	queue := []*ticket{queued(1000, now, now), queued(1010, now, now)}
	if groups := formGroups(queue, at(now), 2, 3); len(groups) != 0 {
		t.Fatalf("a short group formed before fillWait: %v", groups)
	}

	now = fillWait
	queue = []*ticket{queued(1000, now, now), queued(1010, now, now)}
	if groups := formGroups(queue, at(now), 2, 3); len(groups) != 1 || len(groups[0]) != 2 {
		t.Fatalf("want one group of 2 after fillWait, got %v", groups)
	}
}

func TestNewcomerIsNotPutWithFarPlayers(t *testing.T) {
	now := 5 * time.Minute
	// Both veterans accept 250 by now, but the newcomer, there for a moment, accepts only 25
	queue := []*ticket{queued(1000, now, now), queued(1240, now, now), queued(1200, 0, now)}

	groups := formGroups(queue, at(now), 2, 3)
	if len(groups) != 1 || len(groups[0]) != 2 {
		t.Fatalf("want the veterans alone, got %v", groups)
	}
	for _, tk := range groups[0] {
		if tk.account.Rating == 1200 {
			t.Fatal("newcomer matched across a range they never waited for")
		}
	}
}

func TestRangeNeverExceedsTolerance(t *testing.T) {
	now := fillWait
	// Any two neighbours fit, but all three span nearly twice the tolerance
	gap := matchTolerance(now) * 0.9
	queue := []*ticket{queued(1000, now, now), queued(1000+gap, now, now), queued(1000-gap, now, now)}

	groups := formGroups(queue, at(now), 2, 3)
	if len(groups) != 1 || len(groups[0]) != 2 {
		t.Fatalf("want a pair, got %v", groups)
	}
	lo, hi := groups[0][0].account.Rating, groups[0][1].account.Rating
	if d := max(lo, hi) - min(lo, hi); d > matchTolerance(now) {
		t.Fatalf("group spans %v, over the tolerance %v", d, matchTolerance(now))
	}
}

func TestEveryoneIsInAtMostOneGroup(t *testing.T) {
	now := time.Minute
	var queue []*ticket
	for i := range 7 {
		queue = append(queue, queued(1000+float64(i), now, now))
	}

	seen := make(map[*ticket]bool)
	for _, g := range formGroups(queue, at(now), 2, 3) {
		for _, tk := range g {
			if seen[tk] {
				t.Fatal("ticket placed in two groups")
			}
			seen[tk] = true
		}
	}
	if len(seen) != 6 {
		t.Fatalf("want two groups of 3, with the 7th left queued alone, placed %d", len(seen))
	}
}

// BenchmarkFormGroups runs one matchmaking pass over queues of ratings spread like a real ladder,
// with everyone somewhere within the first minute of waiting
func BenchmarkFormGroups(b *testing.B) {
	for _, n := range []int{100, 1000, 5000} {
		rng := rand.New(rand.NewSource(1))
		now := time.Minute
		queue := make([]*ticket, n)
		for i := range queue {
			waited := time.Duration(rng.Int63n(int64(now)))
			queue[i] = queued(1000+rng.NormFloat64()*200, waited, now)
		}
		slices.SortFunc(queue, func(a, b *ticket) int { return a.queuedAt.Compare(b.queuedAt) }) // join order, as Enqueue keeps it

		b.Run(fmt.Sprint(n), func(b *testing.B) {
			for b.Loop() {
				formGroups(queue, at(now), 2, 3)
			}
		})
	}
}
