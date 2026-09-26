package game

import (
	"math"
	"math/rand"
	"strings"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
)

type TradingStation struct {
	Label     string
	Commodity pb.CommodityType
	Pos       Vec2f
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

	positions := ellipsePoints(WorldMaxX/2.0, WorldMaxY/2.0, 25, 16, len(types))
	rand.Shuffle(len(positions), func(i, j int) {
		positions[i], positions[j] = positions[j], positions[i]
	})

	return stationsAt(types, positions)
}

const (
	// Random stations land within this distance of spawn, the area the bot always crosses first.
	randomLayoutRadius = 40.0
	// Room between spawn and a station, so no player joins touching one.
	spawnClearance = PlayerRadius + StationRadius + 2
	// Centre-to-centre distance that leaves a player room to pass between two stations.
	stationSpacing = 2*(PlayerRadius+StationRadius) + 1
)

// RandomTradingStations places one station per commodity at random around spawn, apart from each
// other and from spawn itself. The same rng state always gives the same layout.
func RandomTradingStations(rng *rand.Rand) []*TradingStation {
	types := GetCommodityTypes()
	positions := make([]Vec2f, 0, len(types))
	for len(positions) < len(types) {
		// sqrt keeps the density uniform over the disc instead of bunching at the centre
		r := randomLayoutRadius * math.Sqrt(rng.Float64())
		theta := 2 * math.Pi * rng.Float64()
		p := Vec2f{X: SpawnPos.X + r*math.Cos(theta), Y: SpawnPos.Y + r*math.Sin(theta)}

		if EuclideanDistance(p, SpawnPos) < spawnClearance {
			continue
		}
		clear := true
		for _, q := range positions {
			if EuclideanDistance(p, q) < stationSpacing {
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

// stationsAt pairs each commodity with the position at the same index.
func stationsAt(types []pb.CommodityType, positions []Vec2f) []*TradingStation {
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
