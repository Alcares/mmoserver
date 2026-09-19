package game

import pb "github.com/alcares/mmoserver/gen/go/game/v1"

const (
	// Cash is whole cents everywhere: balances, prices, order amounts and the pool's cash reserve.
	//
	// UnitScale is the fixed-point scale of commodity quantities: 1 unit is
	// 1,000,000 micro-units. Pools and inventories both hold micro-units, so a
	// trade can buy a fraction of a unit.
	UnitScale = 1_000_000

	// MinBuyUnits is the smallest order the pool fills (micro-units). Bigger orders
	// can be fractional (1.2, 14.55 units), but never less than one whole unit.
	MinBuyUnits = 1 * UnitScale

	initialUnits      = 100
	initialPriceCents = 10_00 // cash per whole unit, used only to seed cashReserve
)

// CommodityState is one commodity's constant-product AMM pool
type CommodityState struct {
	cashReserve uint64  // cents
	unitReserve uint64  // micro-units
	lastPrice   float64 // unitPrice as of the last tick's broadcast for calculating price delta
}

// ceilDiv rounds up. Every reserve is rounded up so that rounding error always
// stays in the pool: k = cashReserve*unitReserve can grow but never shrink,
// otherwise a trader could buy fractional units for free.
func ceilDiv(a, b uint64) uint64 {
	return (a + b - 1) / b
}

// unitPrice is the price of the commodity: the cash (cents) that buys exactly one
// whole unit right now, rounded up to the cent. This is the price shown to players and
// the order they send; sending exactly it fills the minimum order, a hair over one unit.
// Returns 0 if the pool holds no more than one unit and can't sell one.
func (c *CommodityState) unitPrice() uint64 {
	if c.unitReserve <= MinBuyUnits {
		return 0
	}
	k := c.cashReserve * c.unitReserve
	return ceilDiv(k, c.unitReserve-MinBuyUnits) - c.cashReserve
}

// buy spends cashIn against the pool and returns the micro-units bought, which
// is whatever the pool's price is at the moment the trade executes, not when it
// was requested. Returns 0 and leaves the pool untouched if cashIn wouldn't buy
// MinBuyUnits, e.g. because the price rose after the order was placed.
func (c *CommodityState) buy(cashIn uint64) uint64 {
	k := c.cashReserve * c.unitReserve

	// Round the pool's remaining units up, so units bought round down.
	newUnitReserve := ceilDiv(k, c.cashReserve+cashIn)
	unitsOut := c.unitReserve - newUnitReserve
	if unitsOut < MinBuyUnits {
		return 0
	}

	c.cashReserve += cashIn
	c.unitReserve = newUnitReserve
	return unitsOut
}

// sellPrice is the cash (cents) received for selling exactly one whole unit right now,
// rounded down to the cent. It is below unitPrice: the spread is what the pool keeps.
func (c *CommodityState) sellPrice() uint64 {
	k := c.cashReserve * c.unitReserve
	return c.cashReserve - ceilDiv(k, c.unitReserve+UnitScale)
}

// sell returns unitsIn micro-units to the pool and returns how much cash (cents) it pays,
// rounded down so selling can never pay out more than the pool gives up. Returns 0 and
// leaves the pool untouched if the units are worth less than a cent.
func (c *CommodityState) sell(unitsIn uint64) uint64 {
	k := c.cashReserve * c.unitReserve
	newUnitReserve := c.unitReserve + unitsIn
	newCashReserve := ceilDiv(k, newUnitReserve)
	cashOut := c.cashReserve - newCashReserve
	if cashOut == 0 {
		return 0
	}

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
		state := &CommodityState{
			unitReserve: initialUnits * UnitScale,
			cashReserve: initialUnits * initialPriceCents,
		}
		state.lastPrice = float64(state.unitPrice())
		commodities[v] = state
	}

	return commodities
}
