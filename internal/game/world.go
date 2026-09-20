package game

import (
	"sync"
	"time"

	pb "github.com/alcares/mmoserver/gen/go/game/v1"
)

// World represents the central authoritative game state
type World struct {
	Mu sync.RWMutex

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
