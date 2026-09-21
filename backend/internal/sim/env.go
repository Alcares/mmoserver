package sim

import (
	"errors"
	"math/rand"
	"time"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
	"github.com/alcares/mmoserver/backend/internal/bot"
	"github.com/alcares/mmoserver/backend/internal/game"
	"google.golang.org/protobuf/proto"
)

// MaxSteps ends an episode as truncated: 1200 ticks = 60s of game time
const MaxSteps = 1200

// Reward shaping
const (
	progressScale = 1.0 / (game.MoveSpeed * game.TickDuration) // One tick of full-speed progress = +1
	stepPenalty   = 0.01
	arrivalBonus  = 10.0
)

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
	return &Env{grid: game.NewSpatialGrid(), done: true}
}

// Reset starts a new episode from seed: new world (reshuffled stations), bot joined through
// World.Join, random goal, and one Tick so the first Encode succeeds.
// The same seed always gives the same first observation.
func (e *Env) Reset(seed int64) (*bot.Observation, error) {
	e.done = true

	e.rng = rand.New(rand.NewSource(seed))
	e.world = game.NewWorld("sim", game.WorldConfig{
		MinPlayers:     0,
		StartCountdown: 0 * time.Second,
		Duration:       5 * time.Minute,
		Rng:            e.rng,
	})
	e.client = &game.Client{
		Send: make(chan []byte, game.SendBufferSize),
	}
	e.observer = bot.Observer{}

	_, err := e.world.Join(e.client)
	if err != nil {
		return nil, err
	}

	e.world.Tick(e.grid)

	err = e.drain()
	if err != nil {
		return nil, err
	}
	e.goal, err = e.pickGoal()
	if err != nil {
		return nil, err
	}

	obs, err := e.observer.Encode(e.goal)
	if err != nil {
		return nil, err
	}

	e.prevDist = obs.GoalDist
	e.steps = 0
	e.done = false

	return obs, nil
}

// Step applies one movement input for one tick. Calling Step before Reset or after the
// episode ended returns an error.
func (e *Env) Step(vx, vy float64) (StepResult, error) {
	if e.done {
		return StepResult{}, errors.New("sim: Env already done")
	}

	e.world.EnqueueMovement(game.PlayerMovementInput{
		PlayerID: e.client.ID,
		Vx:       vx,
		Vy:       vy,
	})
	e.world.Tick(e.grid)
	if err := e.drain(); err != nil {
		return StepResult{}, err
	}

	obs, err := e.observer.Encode(e.goal)
	if err != nil {
		return StepResult{}, err
	}

	arrived := obs.GoalDist <= game.TradeRange
	e.steps++
	truncated := !arrived && e.steps >= MaxSteps
	stepResult := StepResult{
		Obs:        obs,
		Reward:     reward(e.prevDist, obs.GoalDist, arrived),
		Terminated: arrived,
		Truncated:  truncated,
	}

	e.prevDist = obs.GoalDist
	e.done = truncated || arrived

	return stepResult, nil
}

// drain feeds every queued server message to the observer, without blocking
func (e *Env) drain() error {
	for {
		select {
		case payload := <-e.client.Send:
			var msg pb.ServerMessage
			err := proto.Unmarshal(payload, &msg)
			if err != nil {
				return err
			}
			e.observer.Consume(&msg)
		default:
			return nil
		}
	}
}

// pickGoal chooses a random station from the layout the observer received, the same way a
// player would: from what the server told it, not from World.Stations
func (e *Env) pickGoal() (pb.CommodityType, error) {
	stations := e.observer.Stations
	if len(stations) == 0 {
		return pb.CommodityType_COMMODITY_UNSPECIFIED, errors.New("sim: no stations received")
	}
	return stations[e.rng.Intn(len(stations))].Commodity, nil
}

// reward scores one step: progress towards the goal, a small cost per step, and a bonus on arrival
func reward(prevDist, dist float32, arrived bool) float32 {
	r := (prevDist-dist)*progressScale - stepPenalty
	if arrived {
		r += arrivalBonus
	}
	return r
}
