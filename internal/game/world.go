package game

import (
	"errors"
	"log"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	pb "github.com/alcares/mmoserver/gen/go/game/v1"
	"google.golang.org/protobuf/proto"
)

const (
	MoveSpeed        = 12.0 // World units per second
	DefaultFOVRadius = 25.0 // Float-based vision circle
	WorldMinX        = 0.0
	WorldMaxX        = 500.0
	WorldMinY        = 0.0
	WorldMaxY        = 500.0
	TickDuration     = 0.05 // 50ms = 20Hz
	TradeRange       = 5.0  // Max distance to a station a player can trade from
	MaxPlayers       = 50
)

// Bounds every WorldConfig is clamped to, so client-supplied settings can't create
// a round that never ends or a lobby that holds a game slot forever
const (
	MinRoundDuration  = 1 * time.Minute
	MaxRoundDuration  = 30 * time.Minute
	MaxStartCountdown = 1 * time.Minute
	DefaultLobbyTTL   = 10 * time.Minute
)

var (
	ErrGameInProgress = errors.New("game already in progress")
	ErrGameFull       = errors.New("game is full")
)

// SpawnPos is where every player joins: the centre of the map
var SpawnPos = Vec2f{X: (WorldMinX + WorldMaxX) / 2, Y: (WorldMinY + WorldMaxY) / 2}

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

// WorldConfig holds one game's settings
type WorldConfig struct {
	Duration       time.Duration // How long a round lasts once it starts
	StartCountdown time.Duration // Delay between reaching MinPlayers and the round starting
	LobbyTTL       time.Duration // How long a game waits for MinPlayers before giving up
	MinPlayers     int           // Players needed to start the countdown; 0 starts immediately
	// Rng seeds the station layout. Master mints one per game and overwrites whatever is
	// passed, so only direct NewWorld callers (tests, the sim) set it.
	Rng *rand.Rand
}

// sanitize forces cfg into the supported range, filling in defaults for unset fields.
// Every world goes through it, so no caller can skip the clamps.
func (cfg *WorldConfig) sanitize() {
	cfg.Duration = clampDuration(cfg.Duration, MinRoundDuration, MaxRoundDuration)
	cfg.StartCountdown = clampDuration(cfg.StartCountdown, 0, MaxStartCountdown)

	if cfg.LobbyTTL <= 0 {
		cfg.LobbyTTL = DefaultLobbyTTL
	}
	if cfg.MinPlayers < 0 {
		cfg.MinPlayers = 0
	}
	if cfg.MinPlayers > MaxPlayers {
		cfg.MinPlayers = MaxPlayers
	}
	if cfg.Rng == nil {
		cfg.Rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
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

func (w *World) addPlayer(client *Client) (*Player, error) {
	if len(w.players) >= MaxPlayers {
		return nil, ErrGameFull
	}

	playerID := w.nextPlayerID
	w.nextPlayerID++
	player := NewPlayer(playerID, SpawnPos)

	client.ID = playerID
	w.players[playerID] = player
	w.clients[playerID] = client

	w.statusDirty = true

	return player, nil
}

// Join adds a player for c and queues its InitialGameState and PlayerInventory ahead of any WorldSnapshot
func (w *World) Join(c *Client) (*Player, error) {
	w.Mu.Lock()
	defer w.Mu.Unlock()

	// Allow late joins
	if w.phase != pb.GamePhase_GAME_PHASE_WAITING && w.phase != pb.GamePhase_GAME_PHASE_COUNTDOWN {
		return nil, ErrGameInProgress
	}

	player, err := w.addPlayer(c)
	if err != nil {
		return nil, err
	}

	w.sendTo(player.ID, &pb.ServerMessage{
		Msg: &pb.ServerMessage_InitialState{InitialState: w.initialState()},
	})
	w.sendTo(player.ID, &pb.ServerMessage{
		Msg: &pb.ServerMessage_PlayerInventory{PlayerInventory: player.ToProtoInventory()},
	})

	return player, nil
}

// initialState describes the current station layout; built per join so it never goes stale
func (w *World) initialState() *pb.InitialGameState {
	stations := make([]*pb.TradingStation, 0, len(w.Stations))
	for _, s := range w.Stations {
		stations = append(stations, s.ToProto())
	}
	return &pb.InitialGameState{StationLayout: stations, GameId: w.gameID}
}

func (w *World) removePlayer(playerID uint32) {
	delete(w.players, playerID)
	delete(w.clients, playerID)

	w.statusDirty = true
}

func (w *World) toProtoGameStatus() *pb.GameStatus {
	var remainingMs int32
	if w.phaseEndTick > w.tick {
		remainingMs = int32(float64(w.phaseEndTick-w.tick) * TickDuration * 1000)
	}

	return &pb.GameStatus{
		Phase:       w.phase,
		PlayerCount: int32(len(w.players)),
		MinPlayers:  int32(w.config.MinPlayers),
		RemainingMs: remainingMs,
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

func (w *World) Tick(grid *SpatialGrid) {
	// 1. Drain input and store latest target direction vector
	w.tick++
	w.advancePhase()

drainInputs:
	for {
		select {
		case input := <-w.movementQueue:
			player, exists := w.players[input.PlayerID]
			if !exists {
				continue
			}

			dirX := clampFloat(input.Vx, -1.0, 1.0)
			dirY := clampFloat(input.Vy, -1.0, 1.0)

			// Normalize diagonal movement to prevent moving faster diagonally
			lenSq := dirX*dirX + dirY*dirY
			if lenSq > 1.0 {
				invLen := 1.0 / math.Sqrt(lenSq)
				dirX *= invLen
				dirY *= invLen
			}

			player.TargetDir.X = dirX
			player.TargetDir.Y = dirY

		case order := <-w.tradeQueue:
			player, exists := w.players[order.PlayerID]
			if !exists {
				continue
			}

			receipt := w.executeTrade(player, order)
			w.sendTo(order.PlayerID, &pb.ServerMessage{Msg: &pb.ServerMessage_Trade{Trade: receipt}})
			if receipt.Success {
				w.sendTo(order.PlayerID, &pb.ServerMessage{
					Msg: &pb.ServerMessage_PlayerInventory{PlayerInventory: player.ToProtoInventory()},
				})
			}

		default:
			break drainInputs
		}
	}

	// 2. Continuous Physics Update: Pos += Velocity * dt
	for _, player := range w.players {
		player.Pos.X += player.TargetDir.X * player.Speed * TickDuration
		player.Pos.Y += player.TargetDir.Y * player.Speed * TickDuration

		// Keep within map boundaries
		if player.Pos.X < WorldMinX {
			player.Pos.X = WorldMinX
		}
		if player.Pos.X > WorldMaxX {
			player.Pos.X = WorldMaxX
		}
		if player.Pos.Y < WorldMinY {
			player.Pos.Y = WorldMinY
		}
		if player.Pos.Y > WorldMaxY {
			player.Pos.Y = WorldMaxY
		}
	}

	// 3. Re-index positions into spatial partitions
	grid.Clear()
	for _, player := range w.players {
		grid.Insert(player.ID, player.Pos)
	}

	// 4. Per-client Area of Interest replication`
	for _, c := range w.clients {
		player, exists := w.players[c.ID]
		if !exists {
			continue
		}

		candidates := grid.QueryRadius(player.Pos, DefaultFOVRadius)

		// Add self first
		protoPlayers := []*pb.PlayerState{player.ToProtoState()}

		// Fine-grained narrow phase: Euclidean distance filter
		maxDistSq := DefaultFOVRadius * DefaultFOVRadius
		for _, id := range candidates {
			if id == player.ID {
				continue // Skip self (already added)
			}
			other, ok := w.players[id]
			if !ok {
				continue
			}

			dx := other.Pos.X - player.Pos.X
			dy := other.Pos.Y - player.Pos.Y
			if (dx*dx + dy*dy) <= maxDistSq {
				protoPlayers = append(protoPlayers, other.ToProtoState())
			}
		}

		msg := &pb.ServerMessage{
			Msg: &pb.ServerMessage_WorldSnapshot{
				WorldSnapshot: &pb.WorldSnapshot{
					Tick:    w.tick,
					Players: protoPlayers,
				},
			},
		}

		w.sendTo(c.ID, msg)
	}

	// 5. Broadcast market state: identical for every client, so marshal once
	quotes := make([]*pb.PriceQuote, 0, len(w.Commodities))
	for cType, state := range w.Commodities {
		price := state.buyPrice(1)

		var deltaBasisPoints int32
		if state.lastPrice > 0 {
			deltaBasisPoints = int32((int64(price) - int64(state.lastPrice)) * 10000 / int64(state.lastPrice))
		}
		state.lastPrice = price

		quotes = append(quotes, state.toProtoQuote(cType, deltaBasisPoints))
	}

	marketMsg := &pb.ServerMessage{
		Msg: &pb.ServerMessage_MarketState{
			MarketState: &pb.MarketState{
				Tick:   w.tick,
				Quotes: quotes,
			},
		},
	}

	w.sendToAll(marketMsg)

	if w.statusDirty {
		w.statusDirty = false
		w.sendToAll(&pb.ServerMessage{
			Msg: &pb.ServerMessage_GameStatus{GameStatus: w.toProtoGameStatus()},
		})
	}
}

// setPhase moves to next and marks the status for broadcast; d of 0 means open-ended
func (w *World) setPhase(next pb.GamePhase, d time.Duration) {
	w.phase = next
	if d > 0 {
		w.phaseEndTick = w.tick + ticks(d)
	} else {
		w.phaseEndTick = 0
	}
	w.statusDirty = true
}

// advancePhase runs the round's state machine; caller holds w.Mu
func (w *World) advancePhase() {
	switch w.phase {
	case pb.GamePhase_GAME_PHASE_WAITING:
		if len(w.players) >= w.config.MinPlayers {
			w.setPhase(pb.GamePhase_GAME_PHASE_COUNTDOWN, w.config.StartCountdown)
		} else if w.tick > ticks(w.config.LobbyTTL) {
			w.finish() // nobody ever joined
		}
	case pb.GamePhase_GAME_PHASE_COUNTDOWN:
		if w.tick >= w.phaseEndTick {
			w.setPhase(pb.GamePhase_GAME_PHASE_RUNNING, w.config.Duration)
		}
	case pb.GamePhase_GAME_PHASE_RUNNING:
		if w.tick >= w.phaseEndTick {
			w.finish()
		}
	}
}

func (w *World) netWorth(p *Player) uint64 {
	total := p.balance
	for cType, units := range p.commodities {
		if units == 0 {
			continue
		}
		total += units * w.Commodities[cType].sellPrice(units)
	}
	return total
}

func (w *World) finish() {
	w.setPhase(pb.GamePhase_GAME_PHASE_FINISHED, 0)

	standings := make([]*pb.PlayerFinalStanding, 0, len(w.players))
	for _, player := range w.players {
		standings = append(standings, &pb.PlayerFinalStanding{
			Id:       player.ID,
			Name:     player.Name,
			NetWorth: w.netWorth(player),
		})
	}
	sort.Slice(standings, func(i, j int) bool {
		return standings[i].NetWorth > standings[j].NetWorth
	})

	w.statusDirty = false
	w.sendToAll(&pb.ServerMessage{
		Msg: &pb.ServerMessage_GameStatus{GameStatus: w.toProtoGameStatus()},
	})
	w.sendToAll(&pb.ServerMessage{
		Msg: &pb.ServerMessage_GameOver{GameOver: &pb.GameOver{Standings: standings}},
	})
}

// sendTo queues msg for one player's client without blocking; dropped if its buffer is full
func (w *World) sendTo(playerID uint32, msg *pb.ServerMessage) {
	client, exists := w.clients[playerID]
	if !exists {
		return
	}

	payload, err := proto.Marshal(msg)
	if err != nil {
		log.Printf("Marshal error: %v", err)
		return
	}

	select {
	case client.Send <- payload:
	default:
		// Client buffer is full; drop this frame to keep tick rate steady
	}
}

func (w *World) sendToAll(msg *pb.ServerMessage) {
	payload, err := proto.Marshal(msg)
	if err != nil {
		log.Printf("Marshal error: %v", err)
		return
	}

	for _, c := range w.clients {
		select {
		case c.Send <- payload:
		default:
			// Client buffer is full; drop this frame to keep tick rate steady
		}
	}
}

func clampFloat(val, min, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func ticks(d time.Duration) uint64 {
	return uint64(d.Seconds() / TickDuration)
}

func clampDuration(val, min, max time.Duration) time.Duration {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}
