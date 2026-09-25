package game

import (
	"log"
	"math/rand"
	"time"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
	"github.com/alcares/mmoserver/backend/internal/bot"
	"google.golang.org/protobuf/proto"
)

// BotClient is a Client driven by a policy instead of a socket.
type BotClient struct {
	*SendQueue
	ID       uint32
	world    *World
	observer bot.Observer
	goal     bot.Goal
	policy   bot.Policy

	rng      *rand.Rand
	stations []*pb.TradingStation
	goalIdx  int // -1 until the first pick
}

// SpawnBot joins a policy-driven client to w and starts the goroutine that drives it. The
// goroutine ends when the bot's queue is closed.
func (w *World) SpawnBot() (*BotClient, error) {
	c := &BotClient{
		SendQueue: NewSendQueue(),
		world:     w,
		policy:    w.config.BotPolicy,
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
		goalIdx:   -1,
	}

	player, err := w.Join(c)
	if err != nil {
		return nil, err
	}
	c.ID = player.ID

	go c.run()
	return c, nil
}

func (c *BotClient) run() {
	defer func() {
		c.world.Mu.Lock()
		c.world.removePlayer(c.ID)
		c.world.Mu.Unlock()
	}()

	for payload := range c.Send {
		var msg pb.ServerMessage
		if err := proto.Unmarshal(payload, &msg); err != nil {
			log.Printf("Bot %d: %v", c.ID, err)
			continue
		}
		c.observer.Consume(&msg)

		switch {
		case msg.GetInitialState() != nil:
			c.stations = msg.GetInitialState().GetStationLayout()
			c.pickGoal()
		case msg.GetGameOver() != nil:
			return
		case msg.GetWorldSnapshot() != nil:
			c.step()
		}
	}
}

func (c *BotClient) step() {
	obs, err := c.observer.Encode(c.goal)
	if err != nil {
		return // no snapshot with players yet
	}

	if obs.GoalDist <= TradeRange {
		c.pickGoal()
		if obs, err = c.observer.Encode(c.goal); err != nil {
			return
		}
	}

	vx, vy := c.policy.Act(obs).Vector()
	c.world.EnqueueMovement(PlayerMovementInput{PlayerID: c.ID, Vx: vx, Vy: vy})
}

func (c *BotClient) pickGoal() {
	n := len(c.stations)
	if n == 0 {
		return
	}
	next := c.rng.Intn(n)
	if n > 1 && next == c.goalIdx {
		next = (next + 1) % n
	}
	c.goalIdx = next
	c.goal = bot.Goal{X: c.stations[next].GetX(), Y: c.stations[next].GetY()}
}
