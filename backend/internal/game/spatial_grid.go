package game

import (
	"math"

	"github.com/alcares/mmoserver/backend/internal/geometry"
)

const (
	CellSize = 10.0 // 10x10 world units per cell
)

type SpatialGrid struct {
	cellToPlayers map[uint64][]uint32
}

func NewSpatialGrid() *SpatialGrid {
	return &SpatialGrid{
		cellToPlayers: make(map[uint64][]uint32),
	}
}

func (sg *SpatialGrid) Clear() {
	clear(sg.cellToPlayers)
}

func (sg *SpatialGrid) Insert(id uint32, pos geometry.Vec2f) {
	cx := int32(math.Floor(pos.X / CellSize))
	cy := int32(math.Floor(pos.Y / CellSize))
	key := geometry.CompactCoordinates(cx, cy)

	sg.cellToPlayers[key] = append(sg.cellToPlayers[key], id)
}

func (sg *SpatialGrid) QueryRadius(pos geometry.Vec2f, radius float64) []uint32 {
	minX := int32(math.Floor((pos.X - radius) / CellSize))
	maxX := int32(math.Floor((pos.X + radius) / CellSize))
	minY := int32(math.Floor((pos.Y - radius) / CellSize))
	maxY := int32(math.Floor((pos.Y + radius) / CellSize))

	var candidates []uint32

	for cx := minX; cx <= maxX; cx++ {
		for cy := minY; cy <= maxY; cy++ {
			key := geometry.CompactCoordinates(cx, cy)
			candidates = append(candidates, sg.cellToPlayers[key]...)
		}
	}

	return candidates
}
