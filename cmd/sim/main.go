package main

import (
	"math/rand"
	"time"

	"github.com/alcares/mmoserver/internal/game"
)

func main() {
	world := game.NewWorld(rand.New(rand.NewSource(time.Now().Unix())))
	grid := game.NewSpatialGrid()

	for {
		world.Tick(grid)
	}
}
