package game

import (
	"testing"

	pb "github.com/alcares/mmoserver/gen/go/game/v1"
)

// newTradeWorld returns a world with one player standing on the first station
func newTradeWorld() (*World, *Player, *TradingStation) {
	w := NewWorld()
	station := w.Stations[0]
	player := NewPlayer(1, station.Pos)
	w.players[player.ID] = player
	return w, player, station
}

func TestBuyThenSell(t *testing.T) {
	w, player, station := newTradeWorld()
	pool := w.Commodities[station.Commodity]
	price := pool.buyPrice(5)

	r := w.executeTrade(player, TradeOrder{Intent: pb.OrderIntent_INTENT_BUY, Units: 5, PriceCents: price})
	if !r.Success {
		t.Fatalf("buy rejected: %s", r.Rejection)
	}
	if r.PriceCents != price || r.TotalBalanceChange != 5*price || r.UnitsTransacted != 5 {
		t.Fatalf("receipt = %d units at %d, total %d; want 5 at %d, total %d", r.UnitsTransacted, r.PriceCents, r.TotalBalanceChange, price, 5*price)
	}
	if player.balance != StartingBalance-5*price || player.commodities[station.Commodity] != 5 {
		t.Fatalf("player has %d cents, %d units", player.balance, player.commodities[station.Commodity])
	}

	sellPrice := pool.sellPrice(5)
	r = w.executeTrade(player, TradeOrder{Intent: pb.OrderIntent_INTENT_SELL, Units: 5, PriceCents: sellPrice})
	if !r.Success {
		t.Fatalf("sell rejected: %s", r.Rejection)
	}
	if player.balance != StartingBalance-5*price+5*sellPrice || player.commodities[station.Commodity] != 0 {
		t.Fatalf("player has %d cents, %d units", player.balance, player.commodities[station.Commodity])
	}
}

func TestTradeRejections(t *testing.T) {
	tests := []struct {
		name  string
		setup func(p *Player, station *TradingStation, pool *CommodityState)
		order func(pool *CommodityState) TradeOrder
		want  pb.TradeRejection
	}{
		{
			name:  "not at a station",
			setup: func(p *Player, _ *TradingStation, _ *CommodityState) { p.Pos = Vec2f{X: 0, Y: 0} },
			order: func(pool *CommodityState) TradeOrder {
				return TradeOrder{Intent: pb.OrderIntent_INTENT_BUY, Units: 1, PriceCents: pool.buyPrice(1)}
			},
			want: pb.TradeRejection_TRADE_REJECTION_NOT_AT_STATION,
		},
		{
			name: "zero units",
			order: func(pool *CommodityState) TradeOrder {
				return TradeOrder{Intent: pb.OrderIntent_INTENT_BUY, Units: 0, PriceCents: pool.buyPrice(1)}
			},
			want: pb.TradeRejection_TRADE_REJECTION_INVALID_ORDER,
		},
		{
			name:  "sell price fell",
			setup: func(p *Player, station *TradingStation, _ *CommodityState) { p.commodities[station.Commodity] = 5 },
			order: func(pool *CommodityState) TradeOrder {
				return TradeOrder{Intent: pb.OrderIntent_INTENT_SELL, Units: 5, PriceCents: pool.sellPrice(5) + 1}
			},
			want: pb.TradeRejection_TRADE_REJECTION_PRICE_MOVED,
		},
		{
			name: "buy price rose",
			order: func(pool *CommodityState) TradeOrder {
				return TradeOrder{Intent: pb.OrderIntent_INTENT_BUY, Units: 2, PriceCents: pool.buyPrice(2) - 1}
			},
			want: pb.TradeRejection_TRADE_REJECTION_PRICE_MOVED,
		},
		{
			name:  "can't afford the whole order",
			setup: func(p *Player, _ *TradingStation, pool *CommodityState) { p.balance = 10*pool.buyPrice(10) - 1 },
			order: func(pool *CommodityState) TradeOrder {
				return TradeOrder{Intent: pb.OrderIntent_INTENT_BUY, Units: 10, PriceCents: pool.buyPrice(10)}
			},
			want: pb.TradeRejection_TRADE_REJECTION_INSUFFICIENT_CASH,
		},
		{
			name: "buying the pool's last unit",
			order: func(pool *CommodityState) TradeOrder {
				return TradeOrder{Intent: pb.OrderIntent_INTENT_BUY, Units: pool.unitReserve, PriceCents: 1 << 62}
			},
			want: pb.TradeRejection_TRADE_REJECTION_POOL_EXHAUSTED,
		},
		{
			name:  "selling more than held",
			setup: func(p *Player, station *TradingStation, _ *CommodityState) { p.commodities[station.Commodity] = 4 },
			order: func(pool *CommodityState) TradeOrder {
				return TradeOrder{Intent: pb.OrderIntent_INTENT_SELL, Units: 5, PriceCents: 0}
			},
			want: pb.TradeRejection_TRADE_REJECTION_INSUFFICIENT_UNITS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, player, station := newTradeWorld()
			pool := w.Commodities[station.Commodity]
			if tt.setup != nil {
				tt.setup(player, station, pool)
			}
			balance, units, reserve := player.balance, player.commodities[station.Commodity], pool.unitReserve

			r := w.executeTrade(player, tt.order(pool))
			if r.Success || r.Rejection != tt.want {
				t.Fatalf("got success=2%v rejection=%s, want %s", r.Success, r.Rejection, tt.want)
			}
			if player.balance != balance || player.commodities[station.Commodity] != units || pool.unitReserve != reserve {
				t.Fatal("rejected trade changed state")
			}
		})
	}
}
