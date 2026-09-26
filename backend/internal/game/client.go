package game

import (
	"errors"
	"log"
	"time"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

// SendBufferSize is how many outgoing messages a client can have queued before new ones are dropped
const SendBufferSize = 32
const LobbyHandshakeTimeout = 30 * time.Second

// Client is all the world needs from a participant: somewhere to put outgoing frames.
type Client interface {
	Enqueue(payload []byte)
}

type SendQueue struct {
	Send chan []byte
}

func NewSendQueue() *SendQueue {
	return &SendQueue{Send: make(chan []byte, SendBufferSize)}
}

func (q *SendQueue) Enqueue(payload []byte) {
	select {
	case q.Send <- payload:
	default:
		// Client buffer is full; drop this frame to keep tick rate steady
	}
}

// WebsocketClient represents an active WebSocket connection
type WebsocketClient struct {
	*SendQueue
	ID    uint32
	Conn  *websocket.Conn
	World *World
}

// NewWebsocketClient takes no ID: the world assigns one on join, and JoinWorld records it.
func NewWebsocketClient(conn *websocket.Conn) *WebsocketClient {
	return &WebsocketClient{SendQueue: NewSendQueue(), Conn: conn}
}

func (c *WebsocketClient) WritePump() {
	defer func(Conn *websocket.Conn) {
		if err := Conn.Close(); err != nil {
		}
	}(c.Conn)

	for msg := range c.Send {
		if err := c.Conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
			return
		}
	}
	// cmd/spectator closes Send on a healthy socket when the recap ends; a close frame lets the
	// client see a clean close instead of an error. The game server only closes Send after the
	// socket has died, where this write just fails.
	_ = c.Conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
}

// TODO: I dont like that readpump has this dual identity, we should split it
func (c *WebsocketClient) ReadPump(m *Master, cfg WorldConfig) {
	defer func() {
		if c.World != nil {
			c.World.Mu.Lock()
			c.World.removePlayer(c.ID)
			c.World.Mu.Unlock()
		}
		close(c.Send)
		c.Conn.Close()
	}()

	err := c.Conn.SetReadDeadline(time.Now().Add(LobbyHandshakeTimeout))
	if err != nil {
		return
	}

	for {
		messageType, payload, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		if messageType != websocket.BinaryMessage {
			continue
		}

		var msg pb.ClientMessage
		if err := proto.Unmarshal(payload, &msg); err != nil {
			continue
		}

		if c.World == nil {
			switch cmd := msg.Cmd.(type) {
			case *pb.ClientMessage_CreateGame:
				world, err := m.Create(createConfig(cfg, cmd.CreateGame))
				if err != nil {
					c.rejectJoin(joinRejection(err))
					continue
				}
				if err := c.joinWorld(world); err != nil {
					c.rejectJoin(joinRejection(err))
				}

			case *pb.ClientMessage_JoinGame:
				world, exists := m.Get(cmd.JoinGame.Id)
				if !exists {
					c.rejectJoin(pb.JoinRejection_JOIN_REJECTION_GAME_NOT_FOUND)
					continue
				}
				if err := c.joinWorld(world); err != nil {
					c.rejectJoin(joinRejection(err))
				}
			}
			continue // to avoid falling back the next switch statement
		}

		switch cmd := msg.Cmd.(type) {
		case *pb.ClientMessage_Input:
			c.World.EnqueueMovement(PlayerMovementInput{
				PlayerID: c.ID,
				Vx:       float64(cmd.Input.GetVx()),
				Vy:       float64(cmd.Input.GetVy()),
			})
		case *pb.ClientMessage_Trade:
			o := TradeOrder{
				PlayerID:   c.ID,
				SequenceID: cmd.Trade.GetSequenceId(),
				Intent:     cmd.Trade.GetIntent(),
				Units:      uint64(cmd.Trade.GetUnits()),
				PriceCents: cmd.Trade.GetPriceCents(),
			}

			select {
			case c.World.tradeQueue <- o:
			default:
				// Buffer full
			}
		case *pb.ClientMessage_SpawnBot:
			if _, err := c.World.SpawnBot(); err != nil {
				log.Printf("Spawn bot: %v", err)
				//c.rejectJoin(joinRejection(err))
			}
		}

	}
}

// joinRejection maps a create or join failure onto the reason sent to the client
func joinRejection(err error) pb.JoinRejection {
	switch {
	case errors.Is(err, ErrGameFull):
		return pb.JoinRejection_JOIN_REJECTION_GAME_FULL
	case errors.Is(err, ErrGameInProgress):
		return pb.JoinRejection_JOIN_REJECTION_GAME_IN_PROGRESS
	case errors.Is(err, ErrServerFull):
		return pb.JoinRejection_JOIN_REJECTION_SERVER_FULL
	default:
		return pb.JoinRejection_JOIN_REJECTION_UNSPECIFIED
	}
}

// rejectJoin tells the client why it isn't in a game; the connection stays open in the
// lobby so it can retry, for instance after a mistyped code
func (c *WebsocketClient) rejectJoin(reason pb.JoinRejection) {
	payload, err := proto.Marshal(&pb.ServerMessage{
		Msg: &pb.ServerMessage_JoinRejected{JoinRejected: &pb.JoinRejected{Reason: reason}},
	})
	if err != nil {
		log.Printf("Marshal error: %v", err)
		return
	}

	select {
	case c.Send <- payload:
	default:
		// Buffer full
	}
}

// createConfig overlays the settings a client asked for onto the server's defaults.
// CreateGame carries none yet; when it does, an unset field keeps the default and
// NewWorld's sanitize clamps whatever the client sent.
func createConfig(defaults WorldConfig, req *pb.CreateGame) WorldConfig {
	cfg := defaults
	_ = req
	return cfg
}

func (c *WebsocketClient) joinWorld(world *World) error {
	player, err := world.Join(c)
	if err != nil {
		log.Printf("Rejected client: %v", err)
		return err
	}

	err = c.Conn.SetReadDeadline(time.Time{})
	if err != nil {
		return err
	}

	c.ID = player.ID
	c.World = world
	log.Printf("Player %d (%s) joined at (%.1f, %.1f)", player.ID, player.Name, player.Pos.X, player.Pos.Y)

	return nil
}
