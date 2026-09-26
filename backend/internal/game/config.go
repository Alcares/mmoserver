package game

import (
	"errors"
	"log/slog"
	"math/rand"
	"time"

	"github.com/alcares/mmoserver/backend/internal/bot"
)

const (
	MoveSpeed        = 12.0 // World units per second
	DefaultFOVRadius = 25.0 // Float-based vision circle
	WorldMinX        = 0.0
	WorldMaxX        = 500.0
	WorldMinY        = 0.0
	WorldMaxY        = 500.0
	TickDuration     = 0.05 // 50ms = 20Hz
	TradeRange       = 4.0  // Max distance to a station a player can trade from. Have to be kept in sync with its mirrors
	PlayerRadius     = 1.0
	StationRadius    = 2.0
	MaxPlayers       = 50
)

// Bounds every WorldConfig is clamped to, so client-supplied settings can't create
// a round that never ends or a lobby that holds a game slot forever
const (
	MinRoundDuration  = 1 * time.Minute
	MaxRoundDuration  = 30 * time.Minute
	MaxStartCountdown = 1 * time.Minute
	DefaultLobbyTTL   = 10 * time.Minute
)

var (
	ErrGameInProgress = errors.New("game already in progress")
	ErrGameFull       = errors.New("game is full")
)

// SpawnPos is where every player joins: the centre of the map
var SpawnPos = Vec2f{X: (WorldMinX + WorldMaxX) / 2, Y: (WorldMinY + WorldMaxY) / 2}

// WorldConfig holds one game's settings
type WorldConfig struct {
	Duration       time.Duration // How long a round lasts once it starts
	StartCountdown time.Duration // Delay between reaching MinPlayers and the round starting
	LobbyTTL       time.Duration // How long a game waits for MinPlayers before giving up
	MinPlayers     int           // Players needed to start the countdown; 0 starts immediately
	// Rng seeds the station layout. Master mints one per game and overwrites whatever is
	// passed, so only direct NewWorld callers (tests, the sim) set it.
	Rng *rand.Rand
	// Logger receives game events. sanitize defaults it to a discarding logger, so the
	// training sim and the tests stay silent without every call site checking.
	Logger *slog.Logger
	// Layout places the stations. sanitize defaults it to NewTradingStations, the ellipse every
	// real game uses; the sim swaps it to control what stands in a bot's way.
	Layout func(*rand.Rand) []*TradingStation

	BotPolicy bot.Policy
}

// sanitize forces cfg into the supported range, filling in defaults for unset fields.
// Every world goes through it, so no caller can skip the clamps.
func (cfg *WorldConfig) sanitize() {
	cfg.Duration = clampDuration(cfg.Duration, MinRoundDuration, MaxRoundDuration)
	cfg.StartCountdown = clampDuration(cfg.StartCountdown, 0, MaxStartCountdown)

	if cfg.LobbyTTL <= 0 {
		cfg.LobbyTTL = DefaultLobbyTTL
	}

	if cfg.MinPlayers < 0 {
		cfg.MinPlayers = 0
	}
	if cfg.MinPlayers > MaxPlayers {
		cfg.MinPlayers = MaxPlayers
	}
	if cfg.Rng == nil {
		cfg.Rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	if cfg.BotPolicy == nil {
		cfg.BotPolicy = bot.ScriptedPolicy{}
	}
	if cfg.Layout == nil {
		cfg.Layout = NewTradingStations
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

func ticks(d time.Duration) uint64 {
	return uint64(d.Seconds() / TickDuration)
}

func clampDuration(val, min, max time.Duration) time.Duration {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}
