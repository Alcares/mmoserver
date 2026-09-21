package game

import (
	"context"
	"log/slog"
	"sync"
	"time"

	pb "github.com/alcares/mmoserver/gen/go/game/v1"
)

// World represents the central authoritative game state
type World struct {
	Mu     sync.RWMutex
	gameID string

	// Game state
	tick         uint64
	phase        pb.GamePhase
	phaseEndTick uint64 // tick at which the current phase ends; 0 = open-ended
	statusDirty  bool   // broadcast GameStatus on the next tick
	players      map[uint32]*Player
	clients      map[uint32]*Client
	Stations     []*TradingStation
	Commodities  map[pb.CommodityType]*CommodityState

	// Communication channels
	movementQueue chan PlayerMovementInput
	tradeQueue    chan TradeOrder
	// TODO: add a joinQueue and exitQueue to remove the Mu entirely

	// ID generator counter
	nextPlayerID uint32

	// Game configuration
	config WorldConfig
}

func NewWorld(gameID string, cfg WorldConfig) *World {
	cfg.sanitize()
	cfg.Logger = cfg.Logger.With("game", gameID)

	return &World{
		gameID:        gameID,
		tick:          0,
		phase:         pb.GamePhase_GAME_PHASE_WAITING,
		players:       make(map[uint32]*Player),
		clients:       make(map[uint32]*Client),
		Stations:      NewTradingStations(cfg.Rng),
		Commodities:   NewCommodities(),
		movementQueue: make(chan PlayerMovementInput, 1024), // Buffered to handle bursts
		tradeQueue:    make(chan TradeOrder, 64),
		nextPlayerID:  1,
		config:        cfg,
	}
}

func (w *World) EnqueueMovement(mov PlayerMovementInput) {
	select {
	case w.movementQueue <- mov:
	default:
		// Buffer full
	}
}

func (w *World) Run() {
	ticker := time.NewTicker(time.Duration(TickDuration * float64(time.Second)))
	defer ticker.Stop()

	grid := NewSpatialGrid()

	for range ticker.C {
		w.Mu.Lock()
		w.Tick(grid)
		done := w.phase == pb.GamePhase_GAME_PHASE_FINISHED
		w.Mu.Unlock()

		if done {
			return
		}
	}
}

// logTrade records one order's outcome, filled or rejected. The pool's reserves are part of
// the record so the price of any order size at that moment can be recomputed offline; pool is
// nil when the order was rejected before a station was found.
func (w *World) logTrade(player *Player, o TradeOrder, receipt *pb.TradeReceipt, pool *CommodityState) {
	// The sim and the tests discard, so skip building the record for them
	if !w.config.Logger.Enabled(context.Background(), slog.LevelInfo) {
		return
	}

	attrs := []slog.Attr{
		slog.Uint64("tick", w.tick),
		slog.Uint64("player", uint64(player.ID)),
		slog.String("intent", o.Intent.String()),
		slog.String("commodity", receipt.Commodity.String()),
		slog.Uint64("units", o.Units),
		slog.Bool("filled", receipt.Success),
		slog.Uint64("price_cents", receipt.PriceCents),
		slog.Uint64("total_cents", receipt.TotalBalanceChange),
		slog.Uint64("balance_cents", receipt.NewCashBalanceCents),
		slog.Uint64("holding_units", receipt.NewHoldingUnits),
	}
	if !receipt.Success {
		attrs = append(attrs, slog.String("rejection", receipt.Rejection.String()))
	}
	if pool != nil {
		attrs = append(attrs,
			slog.Uint64("pool_cash_cents", pool.cashReserve),
			slog.Uint64("pool_units", pool.unitReserve),
		)
	}

	w.config.Logger.LogAttrs(context.Background(), slog.LevelInfo, "trade", attrs...)
}
