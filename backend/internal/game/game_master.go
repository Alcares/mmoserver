package game

import (
	"errors"
	"log/slog"
	"math/rand"
	"strings"
	"sync"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
	"github.com/google/uuid"
)

// ErrServerFull means this server already hosts maxGames games
var ErrServerFull = errors.New("server is hosting the maximum number of games")

const (
	codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	maxGames     = 5
)

type Result struct {
	GameID        string
	RatingChanges map[uuid.UUID]float64 // account ID to rating change
	// later: standings, end reason, ...
}

type Master struct {
	mu           sync.Mutex
	privateGames map[string]*World
	publicGames  map[string]*World
	maxGames     int
	rng          *rand.Rand
	onGameEnd    func(Result)
}

func NewMaster(rng *rand.Rand, onGameEnd func(Result)) *Master {
	return &Master{
		publicGames:  make(map[string]*World),
		privateGames: make(map[string]*World),
		maxGames:     maxGames,
		rng:          rng,
		onGameEnd:    onGameEnd,
	}
}

func (m *Master) newCode() string { // caller holds m.mu
	for {
		b := make([]byte, 6)
		for i := range b {
			b[i] = codeAlphabet[m.rng.Intn(len(codeAlphabet))]
		}
		code := string(b)
		_, private := m.privateGames[code]
		_, public := m.publicGames[code]
		if !private && !public {
			return code
		}
	}
}

// joinable reports whether Join would currently accept a player
func (w *World) joinable() (float64, bool) {
	w.Mu.Lock()
	defer w.Mu.Unlock()

	fill := float64(len(w.players)) / float64(w.config.MaxPlayers)
	return fill, (w.phase == pb.GamePhase_GAME_PHASE_WAITING || w.phase == pb.GamePhase_GAME_PHASE_COUNTDOWN) && fill < 1.0
}

func (m *Master) FindPublicGame(config WorldConfig) (*World, error) {
	config.IsPublic = true

	m.mu.Lock()
	defer m.mu.Unlock()

	var world *World
	mostFilled := -1.0

	for _, w := range m.publicGames {
		if fill, ok := w.joinable(); ok && fill > mostFilled {
			world, mostFilled = w, fill
		}
	}
	if world != nil {
		return world, nil
	}

	return m.create(config)
}

func (m *Master) Create(config WorldConfig) (*World, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.create(config)
}

// create builds and starts a world; caller holds m.mu
func (m *Master) create(config WorldConfig) (*World, error) {
	gameCount := len(m.publicGames) + len(m.privateGames)
	if gameCount >= m.maxGames {
		return nil, ErrServerFull
	}

	// Each world gets its own rng: they tick on separate goroutines, and *rand.Rand
	// is not safe for concurrent use
	config.Rng = rand.New(rand.NewSource(m.rng.Int63()))

	id := m.newCode()
	world := NewWorld(id, config)
	if world.config.IsPublic {
		m.publicGames[id] = world
	} else {
		m.privateGames[id] = world
	}
	gameCount++

	slog.Info("game started", "game", id, "active_games", gameCount)

	go func() {
		world.Run()
		if m.onGameEnd != nil {
			m.onGameEnd(world.GetResult())
		}

		m.mu.Lock()
		defer m.mu.Unlock()

		if world.config.IsPublic {
			delete(m.publicGames, id)
		} else {
			delete(m.privateGames, id)
		}
		slog.Info("game ended", "game", id, "active_games", len(m.publicGames)+len(m.privateGames))
	}()

	return world, nil
}

func (m *Master) GetPrivate(id string) (*World, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	game, exists := m.privateGames[strings.ToUpper(strings.TrimSpace(id))]
	return game, exists
}
