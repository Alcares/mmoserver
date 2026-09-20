package game

import (
	"errors"
	"math/rand"
	"strings"
	"sync"
)

// ErrServerFull means this server already hosts maxGames games
var ErrServerFull = errors.New("server is hosting the maximum number of games")

const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
const maxGames = 5

type Master struct {
	mu       sync.Mutex
	games    map[string]*World
	maxGames int
	rng      *rand.Rand
}

func NewMaster(rng *rand.Rand) *Master {
	return &Master{
		games:    make(map[string]*World),
		maxGames: maxGames,
		rng:      rng,
	}
}

func (m *Master) newCode() string { // caller holds m.mu
	for {
		b := make([]byte, 6)
		for i := range b {
			b[i] = codeAlphabet[m.rng.Intn(len(codeAlphabet))]
		}
		if _, taken := m.games[string(b)]; !taken {
			return string(b)
		}
	}
}

func (m *Master) Create(config WorldConfig) (*World, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.games) >= m.maxGames {
		return nil, ErrServerFull
	}

	// Each world gets its own rng: they tick on separate goroutines, and *rand.Rand
	// is not safe for concurrent use
	config.Rng = rand.New(rand.NewSource(m.rng.Int63()))

	id := m.newCode()
	world := NewWorld(id, config)
	m.games[id] = world

	go func() {
		world.Run()
		m.mu.Lock()
		defer m.mu.Unlock()
		delete(m.games, id)
	}()

	return world, nil
}

func (m *Master) Get(id string) (*World, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	game, exists := m.games[strings.ToUpper(strings.TrimSpace(id))]
	return game, exists
}
