package game

import (
	"math/rand"
	"testing"

	"github.com/alcares/mmoserver/backend/internal/geometry"
)

func TestRandomTradingStationsKeepApart(t *testing.T) {
	commodities := GetCommodityTypes()
	// One per commodity, and the many the sim uses
	for _, count := range []int{len(commodities), 20} {
		radius := randomLayoutRadius(count)
		for seed := range int64(500) {
			stations := RandomTradingStations(rand.New(rand.NewSource(seed)), count)

			if len(stations) != count {
				t.Fatalf("seed %d: %d stations, want %d", seed, len(stations), count)
			}
			for i, s := range stations {
				if want := commodities[i%len(commodities)]; s.Commodity != want {
					t.Errorf("count %d seed %d: station %d sells %v, want %v: commodities should cycle", count, seed, i, s.Commodity, want)
				}

				d := geometry.EuclideanDistance(s.Pos, SpawnPos)
				if d > radius || d < spawnClearance {
					t.Errorf("count %d seed %d: %s is %.2f from spawn, want between %v and %v", count, seed, s.Label, d, spawnClearance, radius)
				}
				for _, other := range stations[i+1:] {
					if gap := geometry.EuclideanDistance(s.Pos, other.Pos); gap < stationSpacing {
						t.Errorf("count %d seed %d: %s and %s are %.2f apart, want at least %v", count, seed, s.Label, other.Label, gap, stationSpacing)
					}
				}
			}
		}
	}
}

// The point of a random layout is that no position can be learned: seeds must differ, and one
// seed must always give the same layout so an episode can be replayed.
func TestRandomTradingStationsPerSeed(t *testing.T) {
	a := RandomTradingStations(rand.New(rand.NewSource(1)), 20)
	again := RandomTradingStations(rand.New(rand.NewSource(1)), 20)
	b := RandomTradingStations(rand.New(rand.NewSource(2)), 20)

	for i := range a {
		if a[i].Pos != again[i].Pos {
			t.Errorf("seed 1 placed %s at %v, then at %v", a[i].Label, a[i].Pos, again[i].Pos)
		}
	}
	// Compared as sets: shuffling commodities between the same spots would still be learnable
	spots := make(map[geometry.Vec2f]bool)
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
