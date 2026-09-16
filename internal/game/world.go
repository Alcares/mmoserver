package game

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	pb "github.com/alcares/mmoserver/gen/go/game/v1"
	"google.golang.org/protobuf/proto"
)

const (
	MoveSpeed        = 8.0  // World units per second
	PlayerRadius     = 0.5  // Collision boundary size
	DefaultFOVRadius = 15.0 // Float-based vision circle
	WorldMinX        = 0.0
	WorldMaxX        = 500.0
	WorldMinY        = 0.0
	WorldMaxY        = 500.0
	TickDuration     = 0.05 // 50ms = 20Hz
)

// World represents the central authoritative game state
type World struct {
	Mu sync.RWMutex

	// Game state
	tick        uint64
	players     map[uint32]*Player
	clients     map[uint32]*Client
	Stations    []*TradingStation
	Commodities map[pb.CommodityType]*CommodityState

	// Communication channels
	movementQueue chan PlayerMovementInput
	tradeQueue    chan TradeOrder

	// ID generator counter
	availableSpawns []Vec2f
	nextPlayerID    uint32
}

func NewWorld() *World {
	// Predefined fixed spawn positions
	// Generates 50 spawns scattered in a radius around the central town (250, 250)
	initialSpawns := make([]Vec2f, 0, 50)
	for i := 0; i < 50; i++ {
		ring := float64((i%5 + 1) * 8)
		angle := float64(i) * 0.7
		initialSpawns = append(initialSpawns, Vec2f{
			X: 250.0 + ring*math.Cos(angle),
			Y: 250.0 + ring*math.Sin(angle),
		})
	}

	return &World{
		tick:            0,
		players:         make(map[uint32]*Player),
		clients:         make(map[uint32]*Client),
		Stations:        NewTradingStations(),
		Commodities:     NewCommodities(),
		movementQueue:   make(chan PlayerMovementInput, 1024), // Buffered to handle bursts
		tradeQueue:      make(chan TradeOrder, 64),
		availableSpawns: initialSpawns,
		nextPlayerID:    1,
	}
}

func (w *World) AddPlayer(client *Client) (*Player, error) {
	if len(w.availableSpawns) == 0 {
		return nil, fmt.Errorf("server full: no spawn points available")
	}

	spawnPos := w.availableSpawns[0]
	w.availableSpawns = w.availableSpawns[1:]

	playerID := w.nextPlayerID
	w.nextPlayerID++
	player := NewPlayer(playerID, spawnPos)

	client.ID = playerID
	w.players[playerID] = player
	w.clients[playerID] = client

	return player, nil
}

func (w *World) removePlayer(playerID uint32) {
	player, exists := w.players[playerID]
	if !exists {
		return
	}

	w.availableSpawns = append(w.availableSpawns, player.Pos)
	delete(w.players, playerID)
	delete(w.clients, playerID)
}

func (w *World) Run() {
	ticker := time.NewTicker(time.Duration(TickDuration * float64(time.Second)))
	defer ticker.Stop()

	grid := NewSpatialGrid()

	for range ticker.C {
		w.Mu.Lock()
		w.tick++

		// 1. Drain input and store latest target direction vector
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

			// TODO: execute trade
			case trade := <-w.tradeQueue:
				player, exists := w.players[trade.PlayerID]
				if !exists {
					continue
				}

				var targetStation *TradingStation

				// 1. Find out if player is near a trading station
				for _, station := range w.Stations {
					if EuclideanDistance(player.Pos, station.Pos) <= 5 {
						targetStation = station
					}
				}

				if targetStation == nil {
					// TODO: return an invalid trade to the client
					continue
				}

				// 2. Find out if player has enough cash for the transaction
				switch trade.Intent {
				case pb.OrderIntent_INTENT_ALLOCATE_FIXED:
					if player.balance < trade.CashAmount {
						// TODO: return an invalid trade to the client
						continue
					}
					if w.Commodities[targetStation.Commodity].amount == 0 {
						continue
					}
					player.balance -= trade.CashAmount
					player.commodities[targetStation.Commodity]++
					w.Commodities[targetStation.Commodity].amount--
					// TODO: updateCommodityPrice()
				case pb.OrderIntent_INTENT_ALLOCATE_RATIO:
					continue
				case pb.OrderIntent_INTENT_SELL_RATIO:
					continue
				case pb.OrderIntent_INTENT_DUMP_ALL:
					continue
				case pb.OrderIntent_INTENT_UNSPECIFIED:
					continue
				default:
					continue
				}

				client, exists := w.clients[trade.PlayerID]
				if !exists {
					continue
				}

				msg := &pb.ServerMessage{
					Msg: &pb.ServerMessage_PlayerInventory{
						PlayerInventory: player.ToProtoInventory(),
					},
				}

				payload, err := proto.Marshal(msg)
				if err != nil {
					log.Printf("Marshal error: %v", err)
					continue
				}

				select {
				case client.Send <- payload:
				default:
					// Client buffer is full; drop this frame to keep tick rate steady
				}

			// 3. Commit transaction
			// 4. Update balances
			// 5. Broadcast changes
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

		// 4. Per-client Area of Interest replication
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

			payload, err := proto.Marshal(msg)
			if err != nil {
				log.Printf("Marshal error: %v", err)
				continue
			}

			select {
			case c.Send <- payload:
			default:
				// Client buffer is full; drop this frame to keep tick rate steady
			}
		}

		// 5. Broadcast market state: identical for every client, so marshal once
		quotes := make([]*pb.PriceQuote, 0, len(w.Commodities))
		for cType, state := range w.Commodities {
			quotes = append(quotes, &pb.PriceQuote{
				Commodity:          cType,
				SpotPriceCents:     uint32(state.price),
				AvailablePoolUnits: state.amount,
			})
		}

		marketPayload, err := proto.Marshal(&pb.ServerMessage{
			Msg: &pb.ServerMessage_MarketState{
				MarketState: &pb.MarketState{
					Tick:   w.tick,
					Quotes: quotes,
				},
			},
		})
		if err != nil {
			log.Printf("Marshal error: %v", err)
		} else {
			for _, c := range w.clients {
				select {
				case c.Send <- marketPayload:
				default:
					// Client buffer is full; drop this frame to keep tick rate steady
				}
			}
		}

		w.Mu.Unlock()
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
