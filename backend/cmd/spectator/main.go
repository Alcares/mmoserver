// Command spectator replays a training run's snapshots in sim.Env at 20Hz. Clients receive
// the bot's own stream, so the bot is players[0]; their input is discarded.
// Restart it after starting a new training run: the new run overwrites the old snapshots in
// place, and the spectator only moves on to snapshots newer than the one it is showing.
package main

import (
	"log/slog"
	"math"
	"net/http"
	"os"
	"sync"
	"time"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
	"github.com/alcares/mmoserver/backend/internal/bot"
	"github.com/alcares/mmoserver/backend/internal/game"
	"github.com/alcares/mmoserver/backend/internal/logging"
	"github.com/alcares/mmoserver/backend/internal/sim"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

// logPath is relative to the repo root the binary runs from.
const logPath = "backend/spectator.log"

var upgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

// hub fans the bot's stream out to spectators and replays the episode's InitialGameState and
// GoalMarker to late joiners. A spectator is a game.WebsocketClient that never joins a world.
type hub struct {
	mu      sync.Mutex
	clients map[*game.WebsocketClient]struct{}
	initial []byte
	goal    []byte

	// onFirstJoin runs once, after the first spectator is added, so no episode plays unwatched
	// before anyone connects. It keeps playing after everyone leaves.
	onFirstJoin func()
	firstJoin   sync.Once
}

func newHub(onFirstJoin func()) *hub {
	return &hub{clients: make(map[*game.WebsocketClient]struct{}), onFirstJoin: onFirstJoin}
}

func (h *hub) add(c *game.WebsocketClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.initial != nil {
		c.Enqueue(h.initial)
	}
	if h.goal != nil {
		c.Enqueue(h.goal)
	}
	h.clients[c] = struct{}{}
}

// remove closes Send under broadcast's lock, so nothing enqueues after it.
func (h *hub) remove(c *game.WebsocketClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
	close(c.Send)
}

func (h *hub) broadcast(payload []byte, initial bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if initial {
		h.initial = payload
		h.goal = nil // a new world; its goal follows
	}
	for c := range h.clients {
		c.Enqueue(payload)
	}
}

// showGoal marks the episode's goal, which is not part of the game stream.
func (h *hub) showGoal(g bot.Goal) {
	payload, err := proto.Marshal(&pb.ServerMessage{
		Msg: &pb.ServerMessage_GoalMarker{GoalMarker: &pb.GoalMarker{X: g.X, Y: g.Y}},
	})
	if err != nil {
		slog.Error("marshal goal", "err", err)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.goal = payload
	for c := range h.clients {
		c.Enqueue(payload)
	}
}

func (h *hub) serve(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("upgrade failed", "err", err)
		return
	}
	c := game.NewWebsocketClient(conn)
	h.add(c)
	defer h.remove(c)
	go c.WritePump()
	h.firstJoin.Do(h.onFirstJoin)

	// Input is ignored; reading is how we notice the socket closing.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func isInitialState(payload []byte) bool {
	var msg pb.ServerMessage
	if proto.Unmarshal(payload, &msg) != nil {
		return false
	}
	return msg.GetInitialState() != nil
}

// watch plays the same seeds with every snapshot, forever, so the weights are the only
// thing that changes between them.
func watch(env *sim.Env, h *hub, p *bot.SnapshotPolicy, seeds []int64, maxTicks int) {
	ticker := time.NewTicker(time.Duration(game.TickDuration * float64(time.Second)))
	defer ticker.Stop()

	for {
		for _, seed := range seeds {
			result, ticks := episode(env, h, p, ticker, seed, maxTicks)

			ending := "arrived"
			switch {
			case result.Truncated:
				ending = "timed out"
			case !result.Terminated:
				ending = "gave up" // hit maxTicks
			}
			slog.Info("episode",
				"policy", p.String(),
				"seed", seed,
				"ending", ending,
				"ticks", ticks,
				"goal_dist", math.Round(float64(result.Obs.GoalDist)*100)/100)
		}
		p.Reload()
	}
}

// episode plays one seed until it ends or hits maxTicks, and returns the last step and tick count.
func episode(env *sim.Env, h *hub, p *bot.SnapshotPolicy, ticker *time.Ticker, seed int64, maxTicks int) (sim.StepResult, int) {
	obs, err := env.Reset(seed)
	if err != nil {
		slog.Error("reset", "seed", seed, "err", err)
		os.Exit(1)
	}
	h.showGoal(env.Goal())

	var result sim.StepResult
	for ticks := 1; ; ticks++ {
		<-ticker.C

		if result, err = env.Step(p.Act(obs)); err != nil {
			slog.Error("step", "seed", seed, "err", err)
			os.Exit(1)
		}
		obs = result.Obs

		if result.Terminated || result.Truncated || ticks >= maxTicks {
			return result, ticks
		}
	}
}

const (
	DefaultAddr      = ":8080"                 // beside the game server's :8080
	DefaultSnapshots = "rl-training/snapshots" // where train.py exports
	DefaultSeed      = 0
	DefaultEpisodes  = 3
	DefaultMaxTicks  = 700 // 35s; the furthest goal takes ~580 ticks
)

type Config struct {
	Addr      string
	Snapshots string
	Seed      int64 // first seed; each snapshot plays Seed..Seed+Episodes-1
	Episodes  int
	MaxTicks  int // 0 means sim.MaxSteps
}

func NewConfig() Config {
	return Config{
		Addr:      DefaultAddr,
		Snapshots: DefaultSnapshots,
		Seed:      DefaultSeed,
		Episodes:  DefaultEpisodes,
		MaxTicks:  DefaultMaxTicks,
	}
}

func (c *Config) sanitize() {
	if c.Addr == "" {
		c.Addr = DefaultAddr
	}
	if c.Snapshots == "" {
		c.Snapshots = DefaultSnapshots
	}
	if c.Episodes < 1 {
		c.Episodes = DefaultEpisodes
	}
	if c.MaxTicks <= 0 {
		c.MaxTicks = sim.MaxSteps
	}
}

func (c *Config) seeds() []int64 {
	seeds := make([]int64, c.Episodes)
	for i := range seeds {
		seeds[i] = c.Seed + int64(i)
	}
	return seeds
}

func main() {
	cfg := NewConfig()
	cfg.sanitize()

	logger, logFile, err := logging.New(logPath)
	if err != nil {
		slog.Error("open log", "path", logPath, "err", err)
		os.Exit(1)
	}
	defer logFile.Close()
	slog.SetDefault(logger)

	p := bot.NewSnapshotPolicy(cfg.Snapshots, bot.ScriptedPolicy{})
	slog.Info("playing snapshots in order", "dir", cfg.Snapshots)

	env := sim.NewEnv()
	var h *hub
	h = newHub(func() { go watch(env, h, p, cfg.seeds(), cfg.MaxTicks) })
	// Set before the first Reset so its InitialGameState is forwarded.
	env.OnMessage = func(payload []byte) {
		h.broadcast(payload, isInitialState(payload))
	}

	http.HandleFunc("/ws", h.serve)
	slog.Info("spectating", "url", "ws://localhost"+cfg.Addr+"/ws", "log", logPath)
	if err := http.ListenAndServe(cfg.Addr, nil); err != nil {
		slog.Error("listen", "addr", cfg.Addr, "err", err)
		os.Exit(1)
	}
}
