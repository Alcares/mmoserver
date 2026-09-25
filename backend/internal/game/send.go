package game

import (
	"log"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
	"google.golang.org/protobuf/proto"
)

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
