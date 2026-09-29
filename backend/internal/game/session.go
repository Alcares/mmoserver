package game

import (
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func (w *World) addPlayer(client Client, name string) (*Player, error) {
	if len(w.players) >= w.config.MaxPlayers {
		return nil, ErrGameFull
	}

	playerID := w.nextPlayerID
	w.nextPlayerID++
	player := NewPlayer(playerID, SpawnPos, sanitizePlayerName(name, playerID))
	// Which client drives it is the only thing that makes a player a bot, and it is decided here.
	_, player.IsBot = client.(*BotClient)

	w.players[playerID] = player
	w.clients[playerID] = client

	w.statusDirty = true

	return player, nil
}

// Join adds a player for c and queues its InitialGameState and PlayerInventory ahead of any WorldSnapshot
func (w *World) Join(c Client, name string) (*Player, error) {
	w.Mu.Lock()
	defer w.Mu.Unlock()

	// Allow late joins
	if w.phase != pb.GamePhase_GAME_PHASE_WAITING && w.phase != pb.GamePhase_GAME_PHASE_COUNTDOWN {
		w.logPlayerJoin(nil, ErrGameInProgress)
		return nil, ErrGameInProgress
	}

	player, err := w.addPlayer(c, name)
	if err != nil {
		w.logPlayerJoin(nil, err)
		return nil, err
	}

	w.sendTo(player.ID, &pb.ServerMessage{
		Msg: &pb.ServerMessage_InitialState{InitialState: w.initialState()},
	})
	w.sendTo(player.ID, &pb.ServerMessage{
		Msg: &pb.ServerMessage_PlayerInventory{PlayerInventory: player.ToProtoInventory()},
	})

	w.logPlayerJoin(player, nil)
	return player, nil
}

// initialState describes the current station layout; built per join so it never goes stale
func (w *World) initialState() *pb.InitialGameState {
	stations := make([]*pb.TradingStation, 0, len(w.Stations))
	for _, s := range w.Stations {
		stations = append(stations, s.ToProto())
	}
	return &pb.InitialGameState{
		StationLayout: stations,
		GameId:        w.gameID,
		WorldSize:     WorldMaxX,
		TradeRange:    TradeRange,
		PlayerRadius:  PlayerRadius,
		StationRadius: StationRadius,
	}
}

func (w *World) removePlayer(playerID uint32) {
	player := w.players[playerID]
	w.logPlayerLeave(player)

	delete(w.players, playerID)
	delete(w.clients, playerID)

	w.statusDirty = true
}

const maxNameRunes = 16
const minNameRunes = 3

// sanitizePlayerName guards against adversarial input. Proto wire guarantees that name will be a valid UTF-8 string.
func sanitizePlayerName(name string, playerID uint32) string {
	r := []rune(strings.TrimSpace(name))
	if len(r) > maxNameRunes {
		r = r[:maxNameRunes]
	}
	sanitized := make([]rune, 0, len(r))
	for _, c := range r {
		if !unicode.IsControl(c) && !unicode.Is(unicode.Cf, c) {
			sanitized = append(sanitized, c)
		}
	}
	if len(sanitized) < minNameRunes {
		return fmt.Sprintf("Player %d", playerID)
	}
	return string(sanitized)
}

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
