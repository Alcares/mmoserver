package sim

import (
	"math"
	"testing"

	"github.com/alcares/mmoserver/backend/internal/game"
)

// walkToGoal runs one episode stepping straight at the goal at full speed and returns the last
// result and the number of steps taken
func walkToGoal(t *testing.T, e *Env, seed int64) (StepResult, int) {
	t.Helper()
	obs, err := e.Reset(seed)
	if err != nil {
		t.Fatal(err)
	}

	for step := 1; ; step++ {
		// The observation's offsets are ÷ worldSize, and the server never scales a short input
		// up, so sending them raw would crawl. Send the unit direction instead.
		v := obs.Vectorise()
		res, err := e.Step(float64(v[2]/v[4]), float64(v[3]/v[4]))
		if err != nil {
			t.Fatal(err)
		}
		if res.Terminated || res.Truncated {
			return res, step
		}
		obs = res.Obs
	}
}

func TestEnvReachesGoal(t *testing.T) {
	e := NewEnv()
	for seed := range int64(20) {
		start, err := e.Reset(seed)
		if err != nil {
			t.Fatal(err)
		}
		// Full speed covers MoveSpeed*TickDuration per tick; the bot must be within TradeRange
		optimal := int(math.Ceil(float64(start.GoalDist-game.TradeRange) / (game.MoveSpeed * game.TickDuration)))

		res, steps := walkToGoal(t, e, seed)
		if !res.Terminated {
			t.Fatalf("seed %d: goal %v not reached in %d steps", seed, e.goal, steps)
		}
		if steps > optimal+1 {
			t.Errorf("seed %d: took %d steps, optimal is %d", seed, steps, optimal)
		}
	}
}

func TestEnvSameSeedSameStart(t *testing.T) {
	a, err := NewEnv().Reset(42)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewEnv().Reset(42)
	if err != nil {
		t.Fatal(err)
	}
	if a.Vectorise() != b.Vectorise() {
		t.Errorf("same seed, different first observation: %v vs %v", a.Vectorise(), b.Vectorise())
	}
}

func TestEnvTruncatesWhenStandingStill(t *testing.T) {
	e := NewEnv()
	if _, err := e.Reset(1); err != nil {
		t.Fatal(err)
	}

	for step := 1; step <= MaxSteps; step++ {
		res, err := e.Step(0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if res.Terminated {
			t.Fatal("terminated without moving; spawn is inside TradeRange of the goal")
		}
		if res.Truncated != (step == MaxSteps) {
			t.Fatalf("step %d: truncated = %v", step, res.Truncated)
		}
	}

	if _, err := e.Step(0, 0); err == nil {
		t.Error("Step after the episode ended: want error")
	}
}

func TestEnvStepBeforeReset(t *testing.T) {
	if _, err := NewEnv().Step(1, 0); err == nil {
		t.Error("Step before Reset: want error")
	}
}

func TestReward(t *testing.T) {
	fullSpeed := float32(game.MoveSpeed * game.TickDuration)
	cases := []struct {
		name           string
		prevDist, dist float32
		arrived        bool
		want           float32
	}{
		{"full-speed progress", 10, 10 - fullSpeed, false, 1 - stepPenalty},
		{"standing still", 10, 10, false, -stepPenalty},
		{"moving away", 10, 10 + fullSpeed, false, -1 - stepPenalty},
		{"arriving", 5 + fullSpeed, 5, true, 1 - stepPenalty + arrivalBonus},
	}
	for _, c := range cases {
		if got := reward(c.prevDist, c.dist, c.arrived); math.Abs(float64(got-c.want)) > 1e-4 {
			t.Errorf("%s: reward = %v, want %v", c.name, got, c.want)
		}
	}
}
