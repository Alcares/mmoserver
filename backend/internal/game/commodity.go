package game

import pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"

// Cash is whole cents everywhere: balances, prices and the pool's cash reserve.
// Commodity quantities are whole units everywhere: pools, inventories and orders.
const (
	initialPriceCents = 10_00 // cash per whole unit, used only to seed cashReserve
	defaultPoolUnits  = 200   // pool depth for a commodity missing from poolUnits
)

// OrderSizes are the order sizes (whole units) every MarketState quotes, smallest first.
// Clients offer exactly these as their multipliers; the server fills any size.
var OrderSizes = []uint64{1, 2, 5, 10}

// poolUnits is each commodity's starting pool depth in whole units, the one knob for how
// hard its price is to move: deep pools barely move per trade, shallow ones swing hard.
var poolUnits = map[pb.CommodityType]uint64{
	pb.CommodityType_COMMODITY_OIL:     2000,
	pb.CommodityType_COMMODITY_WHEAT:   1000,
	pb.CommodityType_COMMODITY_COFFEE:  500,
	pb.CommodityType_COMMODITY_GOLD:    400,
	pb.CommodityType_COMMODITY_SILVER:  250,
	pb.CommodityType_COMMODITY_LITHIUM: 60,
}

// CommodityState is one commodity's constant-product AMM pool
type CommodityState struct {
	cashReserve uint64 // cents
	unitReserve uint64 // whole units
	lastPrice   uint64 // one-unit buy price as of the last tick's broadcast, for the price delta
}

func newCommodityState(units, priceCents uint64) *CommodityState {
	c := &CommodityState{unitReserve: units, cashReserve: units * priceCents}
	c.lastPrice = c.buyPrice(1)
	return c
}

// ceilDiv rounds up. Every reserve is rounded up so that rounding error always
// stays in the pool: k = cashReserve*unitReserve can grow but never shrink.
func ceilDiv(a, b uint64) uint64 {
	return (a + b - 1) / b
}

// buyPrice is the per-unit price (cents) of buying units whole units right now: the pool's
// cost for the whole order spread evenly over it and rounded up to the cent, so every unit
// costs the same, the order costs exactly units*price, and the pool keeps the rounding.
// Returns 0 if the pool can't sell that many; it always keeps at least one unit.
func (c *CommodityState) buyPrice(units uint64) uint64 {
	if units == 0 || units >= c.unitReserve {
		return 0
	}
	k := c.cashReserve * c.unitReserve
	cost := ceilDiv(k, c.unitReserve-units) - c.cashReserve
	return ceilDiv(cost, units)
}

// sellPrice is the per-unit price (cents) received for selling units whole units right now:
// the pool's payout spread evenly over the order and rounded down to the cent. It is below
// buyPrice for the same size; the spread is what the pool keeps. Returns 0 if each unit is
// worth under a cent.
func (c *CommodityState) sellPrice(units uint64) uint64 {
	if units == 0 {
		return 0
	}
	k := c.cashReserve * c.unitReserve
	payout := c.cashReserve - ceilDiv(k, c.unitReserve+units)
	return payout / units
}

// buy takes units out of the pool at buyPrice(units) each and returns the total cost in
// cents, which is whatever the pool's price is when the trade executes. Returns 0 and leaves
// the pool untouched if the pool can't sell that many.
func (c *CommodityState) buy(units uint64) uint64 {
	price := c.buyPrice(units)
	if price == 0 {
		return 0
	}
	total := units * price
	c.cashReserve += total
	c.unitReserve -= units
	return total
}

// sell returns units to the pool at sellPrice(units) each and returns the total payout in
// cents. Returns 0 and leaves the pool untouched if the units are worth under a cent each.
func (c *CommodityState) sell(units uint64) uint64 {
	price := c.sellPrice(units)
	if price == 0 {
		return 0
	}
	total := units * price
	c.cashReserve -= total
	c.unitReserve += units
	return total
}

// toProtoQuote prices every OrderSizes order against the pool's current reserves
func (c *CommodityState) toProtoQuote(cType pb.CommodityType, deltaBasisPoints int32) *pb.PriceQuote {
	orders := make([]*pb.OrderQuote, len(OrderSizes))
	for i, units := range OrderSizes {
		orders[i] = &pb.OrderQuote{
			Units:          uint32(units),
			BuyPriceCents:  c.buyPrice(units),
			SellPriceCents: c.sellPrice(units),
		}
	}
	return &pb.PriceQuote{
		Commodity:          cType,
		DeltaBasisPoints:   deltaBasisPoints,
		AvailablePoolUnits: uint32(c.unitReserve),
		Orders:             orders,
	}
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
		units, ok := poolUnits[v]
		if !ok {
			units = defaultPoolUnits
		}
		commodities[v] = newCommodityState(units, initialPriceCents)
	}

	return commodities
}
