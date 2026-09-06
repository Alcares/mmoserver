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

type Vec2 struct {
	X int32
	Y int32
}

func CompactCoordinates(x, y int32) uint64 {
	return (uint64(uint32(x)) << 32) | uint64(uint32(y))
}

const CellSize = 10 // Each cell is 10x10 tiles

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
	cx := int32(player.Pos.X) / CellSize
	cy := int32(player.Pos.Y) / CellSize
	key := CompactCoordinates(cx, cy)

	sg.cellToPlayers[key] = append(sg.cellToPlayers[key], player.ID)
}

func (sg *SpatialGrid) QueryRadius(pos Vec2, radius int32) []uint32 {
	minX := (pos.X - radius) / CellSize
	maxX := (pos.X + radius) / CellSize
	minY := (pos.Y - radius) / CellSize
	maxY := (pos.Y + radius) / CellSize

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
	ID   uint32
	Name string
	Pos  Vec2
}

// ToProto maps internal domain state to wire DTO
func (p *Player) ToProto() *pb.PlayerState {
	return &pb.PlayerState{
		Id:   p.ID,
		Name: p.Name,
		X:    int32(p.Pos.X),
		Y:    int32(p.Pos.Y),
	}
}

// PlayerInput bundles the command with who sent it
type PlayerInput struct {
	PlayerID uint32
	Dx       int32
	Dy       int32
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
			Dx:       cmd.GetDx(),
			Dy:       cmd.GetDy(),
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
	availableSpawns []Vec2
	nextPlayerID    uint32
}

func NewWorld() *World {
	// Predefined fixed spawn positions (e.g. within an 800x600 canvas)
	// Generates 50 spawns scattered in a radius around the central town (250, 250)
	initialSpawns := make([]Vec2, 0, 50)
	for i := 0; i < 50; i++ {
		// Rings of 10, 20, 30 tiles out
		ring := float64((i%5 + 1) * 5)
		angle := float64(i) * 0.7
		initialSpawns = append(initialSpawns, Vec2{
			X: int32(math.Floor(250 + ring*math.Cos(angle))),
			Y: int32(math.Floor(250 + ring*math.Sin(angle))),
		})
	}

	return &World{
		tick:            0,
		players:         make(map[uint32]*Player),
		clients:         make(map[uint32]*Client),
		register:        make(chan *Client),
		unregister:      make(chan *Client),
		inputQueue:      make(chan PlayerInput, 1024), // Buffered to hanlde bursts
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
		ID:   playerID,
		Name: fmt.Sprintf("Player %d", playerID),
		Pos: Vec2{
			X: spawnPos.X,
			Y: spawnPos.Y,
		},
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
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	grid := NewSpatialGrid()

	for range ticker.C {
		w.Mu.Lock()
		w.tick++

	drainInputs:
		for {
			select {
			case input := <-w.inputQueue:
				player, exists := w.players[input.PlayerID]
				if !exists {
					continue
				}

				dx := clamp(input.Dx, -1, 1)
				dy := clamp(input.Dy, -1, 1)

				player.Pos.X += dx
				player.Pos.Y += dy

				if player.Pos.X < 0 {
					player.Pos.X = 0
				}
				if player.Pos.X > 499 {
					player.Pos.X = 499
				}
				if player.Pos.Y < 0 {
					player.Pos.Y = 0
				}
				if player.Pos.Y > 499 {
					player.Pos.Y = 499
				}

			default:
				break drainInputs
			}
		}

		grid.Clear()
		for _, player := range w.players {
			grid.Insert(player)
		}

		for _, c := range w.clients {
			player, exists := w.players[c.ID]
			if !exists {
				continue
			}

			candidates := grid.QueryRadius(player.Pos, 6)

			protoPlayers := make([]*pb.PlayerState, 0, len(candidates))
			// Add self first so client always recognizes its identity
			protoPlayers = append(protoPlayers, player.ToProto())
			// Add others visible withing fog of war
			for _, id := range candidates {
				if other, ok := w.players[id]; ok {
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

func clamp(val, min, max int32) int32 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}
