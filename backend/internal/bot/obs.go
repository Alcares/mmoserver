package bot

import (
	"fmt"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
)

// ObsSize is the length of Encode's output, and the policy network's input width
const ObsSize = 5

// Observer is the contract between the game and the bot, and everything else (sim, training, spectator) depends on it.
type Observer struct {
	WorldSize float32
	World     *pb.WorldSnapshot
	// for later
	//inventory *pb.PlayerInventory
	//market    *pb.MarketState
	//receipt   *pb.TradeReceipt
}

type Goal struct {
	X float32
	Y float32
}

type Observation struct {
	botX      float32
	botY      float32
	goalDX    float32 // Signed offset from the bot to the goal station
	goalDY    float32
	GoalDist  float32
	worldSize float32 // The divisor Vectorise scales by, copied from the Observer
}

// Vectorise returns [x, y, dx, dy, dist], all ÷ the world size. Changing the order or length
// invalidates every trained policy.
func (o *Observation) Vectorise() [ObsSize]float32 {
	return [ObsSize]float32{
		normalize(o.botX, o.worldSize),
		normalize(o.botY, o.worldSize),
		normalize(o.goalDX, o.worldSize),
		normalize(o.goalDY, o.worldSize),
		normalize(o.GoalDist, o.worldSize),
	}
}

func (o *Observer) Consume(msg *pb.ServerMessage) { // fed from Client.Send, nothing else
	switch msg.Msg.(type) {
	case *pb.ServerMessage_InitialState:
		o.WorldSize = msg.GetInitialState().WorldSize
	case *pb.ServerMessage_WorldSnapshot:
		o.World = msg.GetWorldSnapshot()
	}
}

func (o *Observer) Encode(goal Goal) (*Observation, error) {
	if o.WorldSize <= 0 {
		return nil, fmt.Errorf("observer: world size %v - cannot encode goal", o.WorldSize)
	}
	// GetPlayers is nil-safe: Encode can run before the first WorldSnapshot arrives
	if len(o.World.GetPlayers()) == 0 {
		return nil, fmt.Errorf("observer: no snapshot with players yet - cannot encode goal")
	}

	observation := &Observation{worldSize: o.WorldSize}
	observation.botX = o.World.Players[0].X
	observation.botY = o.World.Players[0].Y

	observation.goalDX = goal.X - observation.botX
	observation.goalDY = goal.Y - observation.botY

	observation.GoalDist = distance(observation.goalDX, observation.goalDY)

	return observation, nil
}
