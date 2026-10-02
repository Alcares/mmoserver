package game

import (
	"math"

	"github.com/google/uuid"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
)

const kFactor = 32 // industry standard, determines strength of ELO variance

// updateElo requires standings to be sorted
func (w *World) updateElo(standings []*pb.PlayerFinalStanding) {
	byID := make(map[uint32]*Player)
	for p := range w.everyone() {
		byID[p.ID] = p
	}

	// Humans in finishing order, best first
	var ranked []*Player
	var netWorth []uint64
	for _, s := range standings {
		if p := byID[s.Id]; !p.IsBot {
			ranked = append(ranked, p)
			netWorth = append(netWorth, s.NetWorth)
		}
	}

	n := len(ranked)
	if n < 2 {
		return
	}

	delta := make([]float64, n)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			expected := 1 / (1 + math.Pow(10, (ranked[j].Rating-ranked[i].Rating)/400))
			score := 1.0 // player "i" finished ahead of "j"
			if netWorth[i] == netWorth[j] {
				score = 0.5
			}
			change := kFactor * (score - expected)
			delta[i] += change
			delta[j] -= change
		}
	}

	w.ratingChanges = make(map[uuid.UUID]float64, n)
	for i, p := range ranked {
		w.ratingChanges[p.AccountID] += delta[i] / float64(n-1)
	}
}
