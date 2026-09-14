package game

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	pb "github.com/alcares/mmoserver/gen/go/game/v1"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

type Vec2f struct {
	X float64
	Y float64
}

func CompactCoordinates(x, y int32) uint64 {
	return (uint64(uint32(x)) << 32) | uint64(uint32(y))
}

const (
	CellSize         = 10.0 // 10x10 world units per cell
	MoveSpeed        = 8.0  // World units per second
	PlayerRadius     = 0.5  // Collision boundary size
	DefaultFOVRadius = 15.0 // Float-based vision circle
	WorldMinX        = 0.0
	WorldMaxX        = 500.0
	WorldMinY        = 0.0
	WorldMaxY        = 500.0
	TickDuration     = 0.05 // 50ms = 20Hz
)

type SpatialGrid struct {
	cellToPlayers map[uint64][]uint32
}

func NewSpatialGrid() *SpatialGrid {
	return &SpatialGrid{
		cellToPlayers: make(map[uint64][]uint32),
	}
}

func (sg *SpatialGrid) Clear() {
	clear(sg.cellToPlayers)
}

func (sg *SpatialGrid) Insert(player *Player) {
	cx := int32(math.Floor(player.Pos.X / CellSize))
	cy := int32(math.Floor(player.Pos.Y / CellSize))
	key := CompactCoordinates(cx, cy)

	sg.cellToPlayers[key] = append(sg.cellToPlayers[key], player.ID)
}

func (sg *SpatialGrid) QueryRadius(pos Vec2f, radius float64) []uint32 {
	minX := int32(math.Floor((pos.X - radius) / CellSize))
	maxX := int32(math.Floor((pos.X + radius) / CellSize))
	minY := int32(math.Floor((pos.Y - radius) / CellSize))
	maxY := int32(math.Floor((pos.Y + radius) / CellSize))

	var candidates []uint32

	for cx := minX; cx <= maxX; cx++ {
		for cy := minY; cy <= maxY; cy++ {
			key := CompactCoordinates(cx, cy)
			candidates = append(candidates, sg.cellToPlayers[key]...)
		}
	}

	return candidates
}

type Player struct {
	ID        uint32
	Name      string
	Pos       Vec2f
	TargetDir Vec2f // Current intended movement heading (-1 to 1)
	Speed     float64
}

// ToProto maps internal domain state to wire DTO
func (p *Player) ToProto() *pb.PlayerState {
	return &pb.PlayerState{
		Id:   p.ID,
		Name: p.Name,
		X:    float32(p.Pos.X),
		Y:    float32(p.Pos.Y),
	}
}

// PlayerInput bundles the command with who sent it
type PlayerInput struct {
	PlayerID uint32
	Vx       float64
	Vy       float64
}

// Client represents an active WebSocket connection
type Client struct {
	ID   uint32
	Conn *websocket.Conn
	Send chan []byte // Outgoing message buffer (prevents blocking the main game loop)
}

func (c *Client) WritePump() {
	defer c.Conn.Close()

	for msg := range c.Send {
		if err := c.Conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
			break
		}
	}
}

func (c *Client) ReadPump(w *World) {
	defer func() {
		w.Mu.Lock()
		w.removePlayer(c.ID)
		w.Mu.Unlock()
		close(c.Send)
		c.Conn.Close()
	}()

	for {
		messageType, payload, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		if messageType != websocket.BinaryMessage {
			continue
		}

		var cmd pb.InputCommand
		if err := proto.Unmarshal(payload, &cmd); err != nil {
			continue
		}

		select {
		case w.inputQueue <- PlayerInput{
			PlayerID: c.ID,
			Vx:       float64(cmd.GetVx()),
			Vy:       float64(cmd.GetVy()),
		}:
		default:
			// Buffer full (client spamming inputs faster than server ticks)
			// Drop input to maintain server stability
		}
	}
}

// World represents the central authoritative game state
type World struct {
	Mu sync.RWMutex

	// Game state
	tick    uint64
	players map[uint32]*Player
	clients map[uint32]*Client
	// Communication channels
	register   chan *Client
	unregister chan *Client
	inputQueue chan PlayerInput
	// ID generator counter
	availableSpawns []Vec2f
	nextPlayerID    uint32
}

func NewWorld() *World {
	// Predefined fixed spawn positions (e.g. within an 800x600 canvas)
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
		register:        make(chan *Client),
		unregister:      make(chan *Client),
		inputQueue:      make(chan PlayerInput, 1024), // Buffered to handle bursts
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

	player := &Player{
		ID:    playerID,
		Name:  fmt.Sprintf("Player %d", playerID),
		Pos:   spawnPos,
		Speed: MoveSpeed,
	}

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
			case input := <-w.inputQueue:
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
			grid.Insert(player)
		}

		// 4. Per-client Area of Interest replication
		for _, c := range w.clients {
			player, exists := w.players[c.ID]
			if !exists {
				continue
			}

			candidates := grid.QueryRadius(player.Pos, DefaultFOVRadius)

			// Add self first
			protoPlayers := []*pb.PlayerState{player.ToProto()}

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
					protoPlayers = append(protoPlayers, other.ToProto())
				}
			}

			snapshot := &pb.WorldSnapshot{
				Tick:    w.tick,
				Players: protoPlayers,
			}

			payload, err := proto.Marshal(snapshot)
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
