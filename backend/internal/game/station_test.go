package game

import (
	"math/rand"
	"testing"
)

func TestRandomTradingStationsKeepApart(t *testing.T) {
	commodities := len(GetCommodityTypes())
	for seed := range int64(500) {
		stations := RandomTradingStations(rand.New(rand.NewSource(seed)))

		if len(stations) != commodities {
			t.Fatalf("seed %d: %d stations, want one per commodity (%d)", seed, len(stations), commodities)
		}
		seen := make(map[string]bool)
		for i, s := range stations {
			if seen[s.Label] {
				t.Errorf("seed %d: two %s stations", seed, s.Label)
			}
			seen[s.Label] = true

			d := EuclideanDistance(s.Pos, SpawnPos)
			if d > randomLayoutRadius || d < spawnClearance {
				t.Errorf("seed %d: %s is %.2f from spawn, want between %v and %v", seed, s.Label, d, spawnClearance, randomLayoutRadius)
			}
			for _, other := range stations[i+1:] {
				if gap := EuclideanDistance(s.Pos, other.Pos); gap < stationSpacing {
					t.Errorf("seed %d: %s and %s are %.2f apart, want at least %v", seed, s.Label, other.Label, gap, stationSpacing)
				}
			}
		}
	}
}

// The point of a random layout is that no position can be learned: seeds must differ, and one
// seed must always give the same layout so an episode can be replayed.
func TestRandomTradingStationsPerSeed(t *testing.T) {
	a := RandomTradingStations(rand.New(rand.NewSource(1)))
	again := RandomTradingStations(rand.New(rand.NewSource(1)))
	b := RandomTradingStations(rand.New(rand.NewSource(2)))

	for i := range a {
		if a[i].Pos != again[i].Pos {
			t.Errorf("seed 1 placed %s at %v, then at %v", a[i].Label, a[i].Pos, again[i].Pos)
		}
	}
	// Compared as sets: shuffling commodities between the same spots would still be learnable
	spots := make(map[Vec2f]bool)
	for _, s := range a {
		spots[s.Pos] = true
	}
	for _, s := range b {
		if !spots[s.Pos] {
			return
		}
	}
	t.Error("seeds 1 and 2 used the same spots")
}
