package bot

import (
	"math"
	"testing"
)

var _ Policy = ScriptedPolicy{}

// goalAt builds an observation with the goal at a signed offset, the way Encode fills one
func goalAt(dx, dy float32) *Observation {
	return &Observation{goalDX: dx, goalDY: dy, GoalDist: distance(dx, dy)}
}

// bearing puts the goal deg clockwise from north, dist units away
func bearing(deg, dist float64) *Observation {
	rad := deg * math.Pi / 180
	return goalAt(float32(math.Sin(rad)*dist), float32(-math.Cos(rad)*dist))
}

func TestScriptedPolicyDirections(t *testing.T) {
	cases := []struct {
		name   string
		dx, dy float32
		want   Action
	}{
		{"north", 0, -100, ActionN},
		{"north-east", 100, -100, ActionNE},
		{"east", 100, 0, ActionE},
		{"south-east", 100, 100, ActionSE},
		{"south", 0, 100, ActionS},
		{"south-west", -100, 100, ActionSW},
		{"west", -100, 0, ActionW},
		{"north-west", -100, -100, ActionNW},
	}
	for _, c := range cases {
		if got := (ScriptedPolicy{}).Act(goalAt(c.dx, c.dy)); got != c.want {
			t.Errorf("%s: Act(%v, %v) = %v, want %v", c.name, c.dx, c.dy, got, c.want)
		}
	}
}

// Sectors are 45° wide, so the direction flips 22.5° either side of each one
func TestScriptedPolicySectorBoundaries(t *testing.T) {
	cases := []struct {
		deg  float64
		want Action
	}{
		{20, ActionN},
		{25, ActionNE},
		{200, ActionS},
		{205, ActionSW},
		{335, ActionNW},
		{350, ActionN},
	}
	for _, c := range cases {
		if got := (ScriptedPolicy{}).Act(bearing(c.deg, 100)); got != c.want {
			t.Errorf("bearing %v°: Act = %v, want %v", c.deg, got, c.want)
		}
	}
}

func TestScriptedPolicyStopsInRange(t *testing.T) {
	for _, obs := range []*Observation{
		goalAt(0, 0),         // standing on the station
		goalAt(stopRange, 0), // exactly at the range
		goalAt(stopRange/2, 0),
	} {
		if got := (ScriptedPolicy{}).Act(obs); got != ActionStop {
			t.Errorf("Act at distance %v = %v, want %v", obs.GoalDist, got, ActionStop)
		}
	}

	// Just outside, it commits to a direction instead
	if got := (ScriptedPolicy{}).Act(goalAt(0, -(stopRange + 0.1))); got != ActionN {
		t.Errorf("Act just outside stopRange = %v, want %v", got, ActionN)
	}
}

func TestActionValid(t *testing.T) {
	for a := ActionStop; a <= ActionNW; a++ {
		if !a.Valid() {
			t.Errorf("Valid(%v) = false, want true", a)
		}
	}
	// What an off-by-one in the Python action mapping would send
	for _, a := range []Action{-1, ActionNW + 1, 99} {
		if a.Valid() {
			t.Errorf("Valid(%d) = true, want false", a)
		}
	}
}
