package bot

import (
	"math"
	"testing"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
)

// What the server sends; the expected vectors below are scaled by testWorldSize
const (
	testWorldSize  = 500.0
	testTradeRange = 4.0
)

// initial is the server's join message. The Observer takes the world size and trade range from
// it: the goal is the task its caller sets, not something the station layout decides.
func initial(worldSize float32) *pb.ServerMessage {
	return &pb.ServerMessage{Msg: &pb.ServerMessage_InitialState{
		InitialState: &pb.InitialGameState{WorldSize: worldSize, TradeRange: testTradeRange},
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

	want := []float32{1, 0, 0.1}
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
	if obsVec[0] != 0 || obsVec[1] != 1 {
		t.Errorf("direction = %v, %v, want 0, 1", obsVec[0], obsVec[1])
	}
}

// The direction must be as readable a few units from the goal as far away: that is where a
// world-scaled offset shrank to ~0.01 and the policy lost its heading
func TestEncodeDirectionNearGoal(t *testing.T) {
	var o Observer
	o.Consume(initial(testWorldSize))
	o.Consume(snapshot(250, 250))

	obs, err := o.Encode(Goal{X: 253, Y: 254}) // 3-4-5 triangle
	if err != nil {
		t.Fatal(err)
	}
	obsVec := obs.Vectorise()
	if math.Abs(float64(obsVec[0]-0.6)) > 1e-6 || math.Abs(float64(obsVec[1]-0.8)) > 1e-6 {
		t.Errorf("direction = %v, %v, want 0.6, 0.8", obsVec[0], obsVec[1])
	}
}

// Standing on the goal has no direction; it must come out as zeros, not NaN
func TestEncodeOnGoal(t *testing.T) {
	var o Observer
	o.Consume(initial(testWorldSize))
	o.Consume(snapshot(250, 250))

	obs, err := o.Encode(Goal{X: 250, Y: 250})
	if err != nil {
		t.Fatal(err)
	}
	obsVec := obs.Vectorise()
	if obsVec[0] != 0 || obsVec[1] != 0 || obsVec[2] != 0 {
		t.Errorf("direction, dist = %v, %v, %v, want 0, 0, 0", obsVec[0], obsVec[1], obsVec[2])
	}
}

// rays encodes a bot at (x, y) in a world with the given stations, with the server's radii
// (player 1, station 2, so a centre stops 3 from a station's), and returns the 8 ray values
func rays(t *testing.T, x, y float32, stations ...*pb.TradingStation) []float32 {
	t.Helper()
	var o Observer
	o.Consume(&pb.ServerMessage{Msg: &pb.ServerMessage_InitialState{
		InitialState: &pb.InitialGameState{
			WorldSize: testWorldSize, TradeRange: testTradeRange,
			PlayerRadius: 1, StationRadius: 2, StationLayout: stations,
		},
	}})
	o.Consume(snapshot(x, y))
	obs, err := o.Encode(Goal{X: 300, Y: 250})
	if err != nil {
		t.Fatal(err)
	}
	v := obs.Vectorise()
	return v[3:]
}

func checkRays(t *testing.T, got []float32, want map[Action]float64) {
	t.Helper()
	for a := ActionN; a <= ActionNW; a++ {
		w, ok := want[a]
		if !ok {
			w = 1
		}
		if math.Abs(float64(got[a])-w) > 1e-6 {
			t.Errorf("ray %d = %v, want %v", a, got[a], w)
		}
	}
}

func TestRaysClearInOpenSpace(t *testing.T) {
	checkRays(t, rays(t, 250, 250), nil)
}

// The station's centre is 5 east; the bot's centre stops 3 short of it, so 2 units of travel
func TestRaysSeeStation(t *testing.T) {
	got := rays(t, 250, 250, &pb.TradingStation{X: 255, Y: 250})
	// The diagonals pass 5/√2 ≈ 3.54 from the centre, beyond reach, so only east is blocked
	checkRays(t, got, map[Action]float64{ActionE: 2 / RaySize})
}

// A station whose centre is beyond RaySize can still have its edge within range: centre
// RaySize+2 east, so the bot's centre stops at RaySize-1
func TestRaysSeeStationEdgeBeyondRangeCentre(t *testing.T) {
	got := rays(t, 250, 250, &pb.TradingStation{X: 250 + RaySize + 2, Y: 250})
	checkRays(t, got, map[Action]float64{ActionE: (RaySize - 1) / RaySize})
}

// Touching a station after a collision: heading in is blocked, sliding along or leaving is not
func TestRaysWhenTouchingStation(t *testing.T) {
	got := rays(t, 250, 250, &pb.TradingStation{X: 253, Y: 250})
	want := map[Action]float64{ActionE: 0, ActionNE: 0, ActionSE: 0}
	checkRays(t, got, want)
}

func TestRaysSeeMapEdge(t *testing.T) {
	got := rays(t, 2, 250)
	checkRays(t, got, map[Action]float64{ActionW: 2 / RaySize, ActionNW: 2 * math.Sqrt2 / RaySize, ActionSW: 2 * math.Sqrt2 / RaySize})
}

func TestRaysKeepNearestHit(t *testing.T) {
	got := rays(t, 250, 250, &pb.TradingStation{X: 258, Y: 250}, &pb.TradingStation{X: 255, Y: 250})
	checkRays(t, got, map[Action]float64{ActionE: 2 / RaySize})
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

// Without a trade range a policy could never tell it had arrived, and would walk through the goal
func TestEncodeWithoutTradeRange(t *testing.T) {
	var o Observer
	o.Consume(&pb.ServerMessage{Msg: &pb.ServerMessage_InitialState{
		InitialState: &pb.InitialGameState{WorldSize: testWorldSize},
	}})
	o.Consume(snapshot(250, 250))

	if _, err := o.Encode(Goal{X: 300, Y: 250}); err == nil {
		t.Error("Encode with trade range 0: want error")
	}
}

func TestEncodeCarriesTradeRange(t *testing.T) {
	var o Observer
	o.Consume(initial(testWorldSize))
	o.Consume(snapshot(250, 250))

	obs, err := o.Encode(Goal{X: 300, Y: 250})
	if err != nil {
		t.Fatal(err)
	}
	if obs.tradeRange != testTradeRange {
		t.Errorf("observation trade range = %v, want the %v the server sent", obs.tradeRange, testTradeRange)
	}
}
