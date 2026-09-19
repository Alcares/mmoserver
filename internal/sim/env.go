package sim

import (
	"errors"
	"math/rand"

	pb "github.com/alcares/mmoserver/gen/go/game/v1"
	"github.com/alcares/mmoserver/internal/bot"
	"github.com/alcares/mmoserver/internal/game"
)

// MaxSteps ends an episode as truncated: 600 ticks = 30 s of game time
const MaxSteps = 600

// Reward shaping; tune these, but keep them together so they're easy to find
const (
	progressScale = 1.0 / (game.MoveSpeed * game.TickDuration) // One tick of full-speed progress = +1
	stepPenalty   = 0.01
	arrivalBonus  = 10.0
)

// sendBuffer matches the server's per-client buffer, so the bot sees the same drops a player would
const sendBuffer = 32

// StepResult is what one Step returns; a struct rather than 5 return values so a batched
// env can later return []StepResult
type StepResult struct {
	Obs        *bot.Observation
	Reward     float32
	Terminated bool // Reached the goal: a real ending
	Truncated  bool // Ran out of steps: stopped watching, not a failure
}

// Env is one headless world with one bot, stepped as fast as the CPU allows.
// It only sees the world through the bot's Client.Send, like a real client.
// Not safe for concurrent use; run one Env per goroutine.
type Env struct {
	rng      *rand.Rand
	world    *game.World
	grid     *game.SpatialGrid
	client   *game.Client
	observer bot.Observer
	goal     pb.CommodityType
	steps    int
	prevDist float32
	done     bool
}

// NewEnv returns an Env that must be Reset before the first Step
func NewEnv() *Env {
	// TODO: create the grid; mark the Env as done so Step before Reset fails
	return &Env{}
}

// Reset starts a new episode from seed: new world (reshuffled stations), bot joined through
// World.Join, random goal, and one Tick so the first Encode succeeds.
// The same seed always gives the same first observation.
func (e *Env) Reset(seed int64) (*bot.Observation, error) {
	// TODO:
	//   - seeded rng, new World, fresh Client with a sendBuffer-sized Send, fresh Observer
	//     (why must the Observer be replaced, not reused?)
	//   - Join, Tick once, drain
	//   - pickGoal, Encode, remember prevDist
	return nil, errors.New("sim: Reset not implemented")
}

// Step applies one movement input for one tick. Calling Step before Reset or after the
// episode ended returns an error.
func (e *Env) Step(vx, vy float64) (StepResult, error) {
	// TODO: EnqueueMovement → Tick → drain → Encode → reward, terminated, truncated.
	// Which should win if the bot arrives on exactly step MaxSteps?
	return StepResult{}, errors.New("sim: Step not implemented")
}

// Goal is the commodity whose station the bot must reach this episode
func (e *Env) Goal() pb.CommodityType {
	return e.goal
}

// drain feeds every queued server message to the observer, without blocking
func (e *Env) drain() error {
	// TODO: select { case payload := <-e.client.Send: Unmarshal, Consume; default: return }
	return errors.New("sim: drain not implemented")
}

// pickGoal chooses a random station from the layout the observer received, the same way a
// player would: from what the server told it, not from World.Stations
func (e *Env) pickGoal() (pb.CommodityType, error) {
	// TODO: pick with e.rng so the same seed gives the same goal
	return pb.CommodityType_COMMODITY_UNSPECIFIED, errors.New("sim: pickGoal not implemented")
}

// reward scores one step: progress towards the goal, a small cost per step,
// and a bonus on arrival
func reward(prevDist, dist float32, arrived bool) float32 {
	// TODO: use progressScale, stepPenalty and arrivalBonus
	return 0
}
