package main

import (
	"math/rand"
	"time"

	"github.com/alcares/mmoserver/internal/game"
)

func main() {
	world := game.NewWorld("sim", game.WorldConfig{
		MinPlayers:     0,
		StartCountdown: 0 * time.Second,
		Duration:       5 * time.Minute,
		Rng:            rand.New(rand.NewSource(time.Now().UnixNano())),
	})
	grid := game.NewSpatialGrid()

	for {
		world.Tick(grid)
	}
}
