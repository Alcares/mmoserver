package game

import (
	pb "github.com/alcares/mmoserver/gen/go/game/v1"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

// Client represents an active WebSocket connection
type Client struct {
	ID   uint32
	Conn *websocket.Conn
	Send chan []byte // Outgoing message buffer (prevents blocking the main game loop)
}

func (c *Client) WritePump() {
	defer func(Conn *websocket.Conn) {
		if err := Conn.Close(); err != nil {
		}
	}(c.Conn)

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

		var msg pb.ClientMessage
		if err := proto.Unmarshal(payload, &msg); err != nil {
			continue
		}

		switch cmd := msg.Cmd.(type) {
		case *pb.ClientMessage_Input:
			select {
			case w.movementQueue <- PlayerMovementInput{
				PlayerID: c.ID,
				Vx:       float64(cmd.Input.GetVx()),
				Vy:       float64(cmd.Input.GetVy()),
			}:
			default:
				// Buffer full
			}
		case *pb.ClientMessage_Trade:
			o := TradeOrder{
				PlayerID:   c.ID,
				SequenceID: cmd.Trade.GetSequenceId(),
				Intent:     cmd.Trade.GetIntent(),
				Units:      uint64(cmd.Trade.GetUnits()),
				PriceCents: cmd.Trade.GetPriceCents(),
			}

			select {
			case w.tradeQueue <- o:
			default:
				// Buffer full
			}
		}

	}
}
