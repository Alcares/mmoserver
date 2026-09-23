package bot

import (
	"math"
	"testing"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
)

// testWorldSize is what the server sends; the expected vectors below are scaled by it
const testWorldSize = 500.0

// initial is the server's join message. The Observer takes only the world size from it: the
// goal is the task its caller sets, not something the station layout decides.
func initial(worldSize float32) *pb.ServerMessage {
	return &pb.ServerMessage{Msg: &pb.ServerMessage_InitialState{
		InitialState: &pb.InitialGameState{WorldSize: worldSize},
	}}
}

// snapshot builds a WorldSnapshot with the bot itself first, as the server sends it
func snapshot(x, y float32) *pb.ServerMessage {
	return &pb.ServerMessage{Msg: &pb.ServerMessage_WorldSnapshot{
		WorldSnapshot: &pb.WorldSnapshot{Players: []*pb.PlayerState{{Id: 1, X: x, Y: y}}},
	}}
}

func TestEncodeTowardsGoal(t *testing.T) {
	var o Observer
	o.Consume(initial(testWorldSize))
	o.Consume(snapshot(250, 250))

	obs, err := o.Encode(Goal{X: 300, Y: 250})
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
	o.Consume(initial(testWorldSize))
	o.Consume(snapshot(250, 250))
	o.Consume(snapshot(300, 200))

	obs, err := o.Encode(Goal{X: 300, Y: 250})
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
	if _, err := o.Encode(Goal{X: 300, Y: 250}); err == nil {
		t.Error("Encode on an empty Observer: want error")
	}

	// Still no snapshot: Encode must report it, not dereference a nil World
	o.Consume(initial(testWorldSize))
	if _, err := o.Encode(Goal{X: 300, Y: 250}); err == nil {
		t.Error("Encode before any WorldSnapshot: want error")
	}
}

// A server that never sent a world size would make Vectorise divide by zero
func TestEncodeWithoutWorldSize(t *testing.T) {
	var o Observer
	o.Consume(initial(0))
	o.Consume(snapshot(250, 250))

	if _, err := o.Encode(Goal{X: 300, Y: 250}); err == nil {
		t.Error("Encode with world size 0: want error")
	}
}
