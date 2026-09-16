package game

import (
	"math/rand"
	"strings"

	pb "github.com/alcares/mmoserver/gen/go/game/v1"
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

func NewTradingStations() []*TradingStation {
	types := make([]int32, 0, len(pb.CommodityType_name))
	for v := range pb.CommodityType_name {
		if v == int32(pb.CommodityType_COMMODITY_UNSPECIFIED) {
			continue
		}
		types = append(types, v)
	}

	positions := ellipsePoints(WorldMaxX/2.0, WorldMaxY/2.0, 25, 16, len(types))
	rand.Shuffle(len(positions), func(i, j int) {
		positions[i], positions[j] = positions[j], positions[i]
	})

	stations := make([]*TradingStation, len(types))
	for i, label := range types {
		stations[i] = &TradingStation{
			Label:     strings.ToUpper(strings.Split(pb.CommodityType_name[label], "_")[1]),
			Commodity: pb.CommodityType(label),
			Pos:       positions[i],
		}
	}
	return stations
}
