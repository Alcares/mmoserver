package bot

import (
	"math"
	"testing"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
)

func initial(stations ...*pb.TradingStation) *pb.ServerMessage {
	return &pb.ServerMessage{Msg: &pb.ServerMessage_InitialState{
		InitialState: &pb.InitialGameState{StationLayout: stations},
	}}
}

// snapshot builds a WorldSnapshot with the bot itself first, as the server sends it
func snapshot(x, y float32) *pb.ServerMessage {
	return &pb.ServerMessage{Msg: &pb.ServerMessage_WorldSnapshot{
		WorldSnapshot: &pb.WorldSnapshot{Players: []*pb.PlayerState{{Id: 1, X: x, Y: y}}},
	}}
}

func gold(x, y float32) *pb.TradingStation {
	return &pb.TradingStation{Commodity: pb.CommodityType_COMMODITY_GOLD, X: x, Y: y}
}

func TestEncodeTowardsGoal(t *testing.T) {
	var o Observer
	o.Consume(initial(gold(300, 250)))
	o.Consume(snapshot(250, 250))

	obs, err := o.Encode(pb.CommodityType_COMMODITY_GOLD)
	if err != nil {
		t.Fatal(err)
	}
	obsVec := obs.Vectorise()

	want := []float32{0.5, 0.5, 0.1, 0, 0.1}
	for i := range want {
		if math.Abs(float64(obsVec[i]-want[i])) > 1e-6 {
			t.Errorf("obs[%d] = %v, want %v", i, obsVec[i], want[i])
		}
	}
}

func TestEncodeUsesLatestSnapshot(t *testing.T) {
	var o Observer
	o.Consume(initial(gold(300, 250)))
	o.Consume(snapshot(250, 250))
	o.Consume(snapshot(300, 200))

	obs, err := o.Encode(pb.CommodityType_COMMODITY_GOLD)
	if err != nil {
		t.Fatal(err)
	}
	obsVec := obs.Vectorise()
	// Goal is straight down (+y) from the bot, 50 units away
	if obsVec[2] != 0 || math.Abs(float64(obsVec[3]-0.1)) > 1e-6 {
		t.Errorf("dx, dy = %v, %v, want 0, 0.1", obsVec[2], obsVec[3])
	}
}

func TestEncodeNotReady(t *testing.T) {
	var o Observer
	if _, err := o.Encode(pb.CommodityType_COMMODITY_GOLD); err == nil {
		t.Error("Encode on an empty Observer: want error")
	}

	o.Consume(initial(gold(300, 250)))
	if _, err := o.Encode(pb.CommodityType_COMMODITY_GOLD); err == nil {
		t.Error("Encode before any WorldSnapshot: want error")
	}
}

func TestEncodeMissingStation(t *testing.T) {
	var o Observer
	o.Consume(initial(gold(300, 250)))
	o.Consume(snapshot(250, 250))

	for _, goal := range []pb.CommodityType{pb.CommodityType_COMMODITY_WHEAT, pb.CommodityType_COMMODITY_UNSPECIFIED} {
		if _, err := o.Encode(goal); err == nil {
			t.Errorf("Encode(%v) with only GOLD stations: want error", goal)
		}
	}
}
