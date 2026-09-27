package bot

import (
	"fmt"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
	"github.com/alcares/mmoserver/backend/internal/geometry"
)

const (
	// ObsSize is the length of Encode's output, and the policy network's input width
	ObsSize  = 11
	RaySize  = 15.0
	RayCount = ActionCount
)

// Observer is the contract between the game and the bot, and everything else (sim, training, spectator) depends on it.
type Observer struct {
	WorldSize     float32
	TradeRange    float32
	World         *pb.WorldSnapshot
	stationLayout []*pb.TradingStation
	playerRadius  float32
	stationRadius float32
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
	rays      [RayCount]float32 // One per movement direction, capped at RaySize
	worldSize float32           // The divisor Vectorise scales by, copied from the Observer
	// How close counts as arrived, copied from the Observer. Not part of Vectorise.
	tradeRange float32
}

// Arrived reports whether the goal is within the trade range the server sent. No action stops
// the bot, so a caller checks this before asking a policy for the next move.
func (o *Observation) Arrived() bool {
	return o.GoalDist <= o.tradeRange
}

// Vectorise returns the unit direction to the goal, the distance ÷ the world size, then the
// rays ÷ RaySize. Changing the order or length invalidates every trained policy.
func (o *Observation) Vectorise() [ObsSize]float32 {
	v := [ObsSize]float32{2: normalize(o.GoalDist, o.worldSize)}
	// A unit vector reads the same 5 units from the goal as 300 away. On the goal itself there
	// is no direction, so it stays (0, 0).
	if o.GoalDist > 0 {
		v[0] = normalize(o.goalDX, o.GoalDist)
		v[1] = normalize(o.goalDY, o.GoalDist)
	}
	for i, ray := range o.rays {
		v[3+i] = normalize(ray, RaySize)
	}
	return v
}

// castRays gives, for each movement direction, how far the bot's center can travel before it
func (o *Observer) castRays(botX, botY float32) [RayCount]float32 {
	var rays [RayCount]float32
	botPos := geometry.Vec2f{X: float64(botX), Y: float64(botY)}
	// The bot's center stops this far from a station's center, where the two edges touch
	reach := float64(o.playerRadius + o.stationRadius)

	for i := range rays {
		vector := actionVectors[i]
		direction := geometry.Normalize(geometry.Vec2f{X: vector[0], Y: vector[1]})

		// The map clamps the bot's center to the world square, so its edge stops the ray too
		nearest := min(RaySize, geometry.EdgeDistance(botPos, direction, float64(o.WorldSize)))
		for _, station := range o.stationLayout {
			stationPos := geometry.Vec2f{X: float64(station.X), Y: float64(station.Y)}
			if dist, hit := geometry.RayCircle(botPos, direction, stationPos, reach); hit {
				nearest = min(nearest, dist)
			}
		}
		rays[i] = float32(nearest)
	}
	return rays
}

func (o *Observer) Consume(msg *pb.ServerMessage) { // fed from Client.Send, nothing else
	switch msg.Msg.(type) {
	case *pb.ServerMessage_InitialState:
		initialState := msg.GetInitialState()
		o.WorldSize = initialState.WorldSize
		o.TradeRange = initialState.TradeRange
		o.stationLayout = initialState.StationLayout
		o.playerRadius = initialState.PlayerRadius
		o.stationRadius = initialState.StationRadius
	case *pb.ServerMessage_WorldSnapshot:
		o.World = msg.GetWorldSnapshot()
	}
}

func (o *Observer) Encode(goal Goal) (*Observation, error) {
	if o.WorldSize <= 0 {
		return nil, fmt.Errorf("observer: world size %v - cannot encode goal", o.WorldSize)
	}
	if o.TradeRange <= 0 {
		return nil, fmt.Errorf("observer: trade range %v - cannot tell when the goal is reached", o.TradeRange)
	}
	// GetPlayers is nil-safe: Encode can run before the first WorldSnapshot arrives
	if len(o.World.GetPlayers()) == 0 {
		return nil, fmt.Errorf("observer: no snapshot with players yet - cannot encode goal")
	}

	observation := &Observation{worldSize: o.WorldSize, tradeRange: o.TradeRange}
	observation.botX = o.World.Players[0].X
	observation.botY = o.World.Players[0].Y

	observation.goalDX = goal.X - observation.botX
	observation.goalDY = goal.Y - observation.botY

	observation.GoalDist = distance(observation.goalDX, observation.goalDY)
	observation.rays = o.castRays(observation.botX, observation.botY)

	return observation, nil
}
