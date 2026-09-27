package game

import (
	"math"
	"math/rand"
	"strings"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
	"github.com/alcares/mmoserver/backend/internal/geometry"
)

type TradingStation struct {
	Label     string
	Commodity pb.CommodityType
	Pos       geometry.Vec2f
}

// ToProto maps internal domain state to wire DTO
func (t *TradingStation) ToProto() *pb.TradingStation {
	return &pb.TradingStation{
		Label:     t.Label,
		Commodity: t.Commodity,
		X:         float32(t.Pos.X),
		Y:         float32(t.Pos.Y),
	}
}

func NewTradingStations(rand *rand.Rand) []*TradingStation {
	types := GetCommodityTypes()

	positions := geometry.EllipsePoints(WorldMaxX/2.0, WorldMaxY/2.0, 25, 16, len(types))
	rand.Shuffle(len(positions), func(i, j int) {
		positions[i], positions[j] = positions[j], positions[i]
	})

	return stationsAt(types, positions)
}

const (
	// Random stations land within this distance of spawn, the area the bot always crosses first.
	baseLayoutRadius = 40.0
	// Room between spawn and a station, so no player joins touching one.
	spawnClearance = PlayerRadius + StationRadius + 2
	// Centre-to-centre distance that leaves a player room to pass between two stations.
	stationSpacing = 2*(PlayerRadius+StationRadius) + 1
)

// RandomTradingStations places count stations at random around spawn, apart from each other and
// from spawn itself, cycling through the commodities. The same rng state always gives the same layout.
func RandomTradingStations(rng *rand.Rand, count int) []*TradingStation {
	commodities := GetCommodityTypes()
	types := make([]pb.CommodityType, count)
	for i := range types {
		types[i] = commodities[i%len(commodities)]
	}

	radius := randomLayoutRadius(count)
	positions := make([]geometry.Vec2f, 0, count)
	for len(positions) < count {
		p := geometry.RandomPointInDisc(rng, SpawnPos, radius)

		if geometry.EuclideanDistance(p, SpawnPos) < spawnClearance {
			continue
		}
		clear := true
		for _, q := range positions {
			if geometry.EuclideanDistance(p, q) < stationSpacing {
				clear = false
				break
			}
		}
		if clear {
			positions = append(positions, p)
		}
	}
	return stationsAt(types, positions)
}

// randomLayoutRadius grows the disc with the station count, keeping the density of one station
// per commodity within baseLayoutRadius.
func randomLayoutRadius(count int) float64 {
	return baseLayoutRadius * math.Sqrt(float64(count)/float64(len(GetCommodityTypes())))
}

// stationsAt pairs each commodity with the position at the same index.
func stationsAt(types []pb.CommodityType, positions []geometry.Vec2f) []*TradingStation {
	stations := make([]*TradingStation, len(types))
	for i, cType := range types {
		stations[i] = &TradingStation{
			Label:     strings.ToUpper(strings.Split(pb.CommodityType_name[int32(cType)], "_")[1]),
			Commodity: cType,
			Pos:       positions[i],
		}
	}
	return stations
}
