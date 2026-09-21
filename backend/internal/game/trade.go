package game

import (
	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
)

// TradeOrder created out of proto TradeRequest
type TradeOrder struct {
	PlayerID   uint32
	SequenceID uint32
	Intent     pb.OrderIntent
	Units      uint64 // whole units
	PriceCents uint64 // per-unit price the client saw: the worst it accepts
}

// nearestStation is the closest station within TradeRange of pos, or nil
func (w *World) nearestStation(pos Vec2f) *TradingStation {
	var target *TradingStation
	nearest := TradeRange
	for _, station := range w.Stations {
		if d := EuclideanDistance(pos, station.Pos); d <= nearest {
			nearest = d
			target = station
		}
	}
	return target
}

// executeTrade fills the whole order against the pool of the station the player stands at,
// or rejects it and changes nothing. Every unit trades at the same per-unit price, so the
// order moves exactly units*price cents. Must only be called from World.Run.
func (w *World) executeTrade(player *Player, o TradeOrder) *pb.TradeReceipt {
	receipt := &pb.TradeReceipt{
		SequenceId: o.SequenceID,
		Intent:     o.Intent,
	}
	// set once the station is known, so a rejection before that still logs, without pool state
	var pool *CommodityState
	reject := func(reason pb.TradeRejection) *pb.TradeReceipt {
		receipt.Rejection = reason
		receipt.NewCashBalanceCents = player.balance
		receipt.NewHoldingUnits = player.commodities[receipt.Commodity]
		w.logTrade(player, o, receipt, pool)
		return receipt
	}

	if w.phase != pb.GamePhase_GAME_PHASE_RUNNING {
		return reject(pb.TradeRejection_TRADE_REJECTION_GAME_NOT_RUNNING)
	}

	station := w.nearestStation(player.Pos)
	if station == nil {
		return reject(pb.TradeRejection_TRADE_REJECTION_NOT_AT_STATION)
	}
	receipt.Commodity = station.Commodity
	pool = w.Commodities[station.Commodity]

	if o.Units == 0 {
		return reject(pb.TradeRejection_TRADE_REJECTION_INVALID_ORDER)
	}

	var price, total uint64
	switch o.Intent {
	case pb.OrderIntent_INTENT_BUY:
		price = pool.buyPrice(o.Units)
		if price == 0 {
			return reject(pb.TradeRejection_TRADE_REJECTION_POOL_EXHAUSTED)
		}
		if price > o.PriceCents {
			return reject(pb.TradeRejection_TRADE_REJECTION_PRICE_MOVED)
		}
		if player.balance < o.Units*price {
			return reject(pb.TradeRejection_TRADE_REJECTION_INSUFFICIENT_CASH)
		}
		total = pool.buy(o.Units)
		player.balance -= total
		player.tradeVolume += total
		player.commodities[station.Commodity] += o.Units
		player.unitsTraded += o.Units
	case pb.OrderIntent_INTENT_SELL:
		if player.commodities[station.Commodity] < o.Units {
			return reject(pb.TradeRejection_TRADE_REJECTION_INSUFFICIENT_UNITS)
		}
		price = pool.sellPrice(o.Units)
		if price == 0 {
			return reject(pb.TradeRejection_TRADE_REJECTION_POOL_EXHAUSTED)
		}
		if price < o.PriceCents {
			return reject(pb.TradeRejection_TRADE_REJECTION_PRICE_MOVED)
		}
		total = pool.sell(o.Units)
		player.balance += total
		player.tradeVolume += total
		player.commodities[station.Commodity] -= o.Units
		player.unitsTraded += o.Units
	default:
		return reject(pb.TradeRejection_TRADE_REJECTION_INVALID_ORDER)
	}

	receipt.Success = true
	receipt.PriceCents = price
	receipt.UnitsTransacted = o.Units
	receipt.TotalBalanceChange = total
	receipt.NewCashBalanceCents = player.balance
	receipt.NewHoldingUnits = player.commodities[station.Commodity]
	w.logTrade(player, o, receipt, pool)
	return receipt
}
