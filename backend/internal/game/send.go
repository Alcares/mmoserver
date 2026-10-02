package game

import (
	"log"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
	"google.golang.org/protobuf/proto"
)

// SendBufferSize is how many outgoing messages a client can have queued before new ones are dropped
const SendBufferSize = 32

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

	client.Enqueue(payload)
}

func (w *World) sendToAll(msg *pb.ServerMessage) {
	payload, err := proto.Marshal(msg)
	if err != nil {
		log.Printf("Marshal error: %v", err)
		return
	}

	for _, c := range w.clients {
		c.Enqueue(payload)
	}
}
