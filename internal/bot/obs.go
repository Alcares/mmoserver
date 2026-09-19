package bot

import (
	"fmt"

	pb "github.com/alcares/mmoserver/gen/go/game/v1"
)

// worldSize mirrors game.WorldMaxX; bot must not import game, so it only knows what a player knows
const worldSize float32 = 500.0

// ObsSize is the length of Encode's output, and the policy network's input width
const ObsSize = 5

// Observer is the contract between the game and the bot, and everything else (sim, training, spectator) depends on it.
type Observer struct {
	stations []*pb.TradingStation
	world    *pb.WorldSnapshot
	// for later
	//inventory *pb.PlayerInventory
	//market    *pb.MarketState
	//receipt   *pb.TradeReceipt
}

type Observation struct {
	botX     float32
	botY     float32
	goalDX   float32 // Signed offset from the bot to the goal station
	goalDY   float32
	goalDist float32
}

// Vectorise returns [x, y, dx, dy, dist], all ÷ worldSize. Changing the order or length
// invalidates every trained policy.
func (o *Observation) Vectorise() [ObsSize]float32 {
	return [ObsSize]float32{
		normalize(o.botX, worldSize),
		normalize(o.botY, worldSize),
		normalize(o.goalDX, worldSize),
		normalize(o.goalDY, worldSize),
		normalize(o.goalDist, worldSize),
	}
}

func (o *Observer) Consume(msg *pb.ServerMessage) { // fed from Client.Send, nothing else
	switch msg.Msg.(type) {
	case *pb.ServerMessage_InitialState:
		o.stations = msg.GetInitialState().StationLayout
	case *pb.ServerMessage_WorldSnapshot:
		o.world = msg.GetWorldSnapshot()
	}
}

func (o *Observer) Encode(goal pb.CommodityType) (*Observation, error) {
	if o.stations == nil || o.world == nil {
		return nil, fmt.Errorf("observer: no stations or world found - cannot encode goal")
	}
	if len(o.world.Players) == 0 {
		return nil, fmt.Errorf("observer: no players found - cannot encode goal")
	}

	observation := &Observation{}
	observation.botX = o.world.Players[0].X
	observation.botY = o.world.Players[0].Y

	found := false
	for _, station := range o.stations {
		if station.Commodity == goal {
			observation.goalDX = station.X - observation.botX
			observation.goalDY = station.Y - observation.botY
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("observer: no station trades %v - cannot encode goal", goal)
	}

	observation.goalDist = distance(observation.goalDX, observation.goalDY)

	return observation, nil
}
