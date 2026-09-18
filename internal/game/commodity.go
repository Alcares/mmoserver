package game

import pb "github.com/alcares/mmoserver/gen/go/game/v1"

const (
	initialUnitReserve = 100
	initialPrice       = 10 // cash per unit, used only to seed cashReserve
)

// CommodityState is one commodity's constant-product AMM pool
type CommodityState struct {
	cashReserve uint64
	unitReserve uint64
	lastPrice   float64 // price as of the last tick's broadcast for calculating price delta
}

func (c *CommodityState) spotPrice() float64 {
	return float64(c.cashReserve) / float64(c.unitReserve)
}

// buy spends cashIn against the pool and returns how many whole units it
// buys. Returns 0 and leaves the pool untouched if cashIn is too little
func (c *CommodityState) buy(cashIn uint64) uint64 {
	k := c.cashReserve * c.unitReserve
	newCashReserve := c.cashReserve + cashIn
	newUnitReserve := k / newCashReserve
	unitsOut := c.unitReserve - newUnitReserve
	if unitsOut == 0 {
		return 0
	}

	c.cashReserve = newCashReserve
	c.unitReserve = newUnitReserve
	return unitsOut
}

// sell returns unitsIn to the pool and returns how much cash it's worth.
func (c *CommodityState) sell(unitsIn uint64) uint64 {
	k := c.cashReserve * c.unitReserve
	newUnitReserve := c.unitReserve + unitsIn
	newCashReserve := k / newUnitReserve
	cashOut := c.cashReserve - newCashReserve

	c.cashReserve = newCashReserve
	c.unitReserve = newUnitReserve
	return cashOut
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
			unitReserve: initialUnitReserve,
			cashReserve: initialUnitReserve * initialPrice,
			lastPrice:   initialPrice,
		}
	}

	return commodities
}
