package transport

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Sessions struct {
	mu     sync.Mutex
	active map[uuid.UUID]*WebsocketClient // key: string(accountID)
}

func NewSessions() *Sessions {
	return &Sessions{
		active: make(map[uuid.UUID]*WebsocketClient),
	}
}

// replace makes c the account's session and disconnects the one it replaces
func (s *Sessions) replace(id uuid.UUID, c *WebsocketClient) {
	s.mu.Lock()
	old := s.active[id]
	s.active[id] = c
	s.mu.Unlock()

	if old != nil {
		old.Conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(4001, "logged in elsewhere"),
			time.Now().Add(time.Second))
		old.Conn.Close()
	}
}

// remove forgets c's session unless a newer login has already replaced it
func (s *Sessions) remove(id uuid.UUID, c *WebsocketClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active[id] == c {
		delete(s.active, id)
	}
}
