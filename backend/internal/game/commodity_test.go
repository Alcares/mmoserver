package game

import (
	"math/rand"
	"testing"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
)

func TestWorkedExample(t *testing.T) {
	// docs/PRICING.md "Worked example": 100 units at $10.00
	c := newCommodityState(100, 10_00)

	if got := c.buyPrice(1); got != 1011 {
		t.Fatalf("buyPrice(1) = %d, want 1011", got)
	}
	if got := c.sellPrice(1); got != 990 {
		t.Fatalf("sellPrice(1) = %d, want 990", got)
	}
	if got := c.buyPrice(10); got != 1112 {
		t.Fatalf("buyPrice(10) = %d, want 1112", got)
	}

	if got := c.buy(10); got != 11120 {
		t.Fatalf("buy(10) = %d, want 11120", got)
	}
	if c.unitReserve != 90 || c.cashReserve != 111120 {
		t.Fatalf("reserves after buy = %d units, %d cents; want 90, 111120", c.unitReserve, c.cashReserve)
	}

	if got := c.sell(10); got != 11110 {
		t.Fatalf("sell(10) = %d, want 11110", got)
	}
}

func TestBuyNeverEmptiesPool(t *testing.T) {
	c := newCommodityState(10, 10_00)
	if got := c.buyPrice(10); got != 0 {
		t.Fatalf("buyPrice(10) on a 10-unit pool = %d, want 0", got)
	}
	if got := c.buy(10); got != 0 || c.unitReserve != 10 {
		t.Fatalf("buy(10) = %d leaving %d units; want 0 and pool untouched", got, c.unitReserve)
	}
	if got := c.buyPrice(9); got == 0 {
		t.Fatal("buyPrice(9) on a 10-unit pool = 0, want a price")
	}
}

func TestBulkPricing(t *testing.T) {
	c := newCommodityState(500, 10_00)
	for i := 1; i < len(OrderSizes); i++ {
		small, big := OrderSizes[i-1], OrderSizes[i]
		if c.buyPrice(big) < c.buyPrice(small) {
			t.Errorf("buying %d costs less per unit than %d", big, small)
		}
		if c.sellPrice(big) > c.sellPrice(small) {
			t.Errorf("selling %d pays more per unit than %d", big, small)
		}
	}
	if c.sellPrice(1) >= c.buyPrice(1) {
		t.Errorf("sellPrice(1) = %d not below buyPrice(1) = %d", c.sellPrice(1), c.buyPrice(1))
	}
}

// Rounding must always favour the pool: k never shrinks, and no sequence of buys and
// sells ends with the trader holding their starting units and more cash than they began with.
func TestRandomTradesNeverProfit(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for round := 0; round < 2000; round++ {
		c := newCommodityState(uint64(20+rng.Intn(3000)), uint64(1+rng.Intn(100_00)))

		var spent, received, held uint64
		for step := 0; step < 20; step++ {
			k := c.cashReserve * c.unitReserve
			units := uint64(1 + rng.Intn(15))
			if rng.Intn(2) == 0 {
				total := c.buy(units)
				if total > 0 {
					spent += total
					held += units
				}
			} else if held > 0 {
				units = min(units, held)
				if total := c.sell(units); total > 0 {
					received += total
					held -= units
				}
			}
			if c.cashReserve*c.unitReserve < k {
				t.Fatalf("round %d: k shrank from %d to %d", round, k, c.cashReserve*c.unitReserve)
			}
		}
		for held > 0 {
			total := c.sell(held)
			if total == 0 {
				break // worth under a cent: the trader just loses them
			}
			received += total
			held = 0
		}
		if received > spent {
			t.Fatalf("round %d: spent %d, got back %d", round, spent, received)
		}
	}
}

func TestEveryCommodityHasAPool(t *testing.T) {
	for _, cType := range GetCommodityTypes() {
		if _, ok := poolUnits[cType]; !ok {
			t.Errorf("%s has no poolUnits entry", cType)
		}
	}
	commodities := NewCommodities()
	if oil, lithium := commodities[pb.CommodityType_COMMODITY_OIL], commodities[pb.CommodityType_COMMODITY_LITHIUM]; oil.unitReserve <= lithium.unitReserve {
		t.Errorf("oil pool (%d) should be deeper than lithium (%d)", oil.unitReserve, lithium.unitReserve)
	}
}
