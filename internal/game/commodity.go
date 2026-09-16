package game

import pb "github.com/alcares/mmoserver/gen/go/game/v1"

const (
	CommodityPrice = 10
)

type CommodityState struct {
	price  uint64
	amount uint32
}

func GetCommodityTypes() []pb.CommodityType {
	commodityTypes := make([]pb.CommodityType, 0, len(pb.CommodityType_value))

	for v := range pb.CommodityType_name {
		if v == int32(pb.CommodityType_COMMODITY_UNSPECIFIED) {
			continue
		}
		commodityTypes = append(commodityTypes, pb.CommodityType(v))
	}

	return commodityTypes
}

func NewCommodities() map[pb.CommodityType]*CommodityState {
	commodityTypes := GetCommodityTypes()

	commodities := make(map[pb.CommodityType]*CommodityState, len(commodityTypes))
	for _, v := range commodityTypes {
		commodities[v] = &CommodityState{
			amount: uint32(v),
			price:  CommodityPrice,
		}
	}

	return commodities
}
