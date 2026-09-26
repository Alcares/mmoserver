package sim

import (
	"math"
	"math/rand"
	"testing"

	"github.com/alcares/mmoserver/backend/internal/bot"
	"github.com/alcares/mmoserver/backend/internal/game"
)

// reversal reports whether b points exactly opposite a: what a policy oscillating between two
// directions near the goal would look like, instead of converging on it.
func reversal(a, b bot.Action) bool {
	if a == bot.ActionStop || b == bot.ActionStop {
		return false
	}
	avx, avy := a.Vector()
	bvx, bvy := b.Vector()
	return avx == -bvx && avy == -bvy
}

// walkToGoal runs one episode under the scripted policy and returns the last result and the
// number of steps taken. It is the best route the 9 actions allow, so it is also the reference
// every trained policy is measured against.
func walkToGoal(t testing.TB, e *Env, seed int64) (StepResult, int) {
	t.Helper()
	obs, err := e.Reset(seed)
	if err != nil {
		t.Fatal(err)
	}

	var policy bot.ScriptedPolicy
	prev := bot.ActionStop

	for step := 1; ; step++ {
		action := policy.Act(obs)
		if reversal(prev, action) {
			t.Fatalf("seed %d, step %d: %v reversed into %v, %.2f from the goal",
				seed, step, prev, action, obs.GoalDist)
		}

		res, err := e.Step(action)
		if err != nil {
			t.Fatal(err)
		}
		if res.Terminated || res.Truncated {
			// stopRange mirrors game.TradeRange by hand. If the bot would keep walking from a
			// state the env calls arrived, the two have drifted apart.
			if res.Terminated && policy.Act(res.Obs) != bot.ActionStop {
				t.Errorf("seed %d: arrived %.2f from the goal but the policy would keep going; "+
					"bot.stopRange and game.TradeRange disagree", seed, res.Obs.GoalDist)
			}
			return res, step
		}
		prev, obs = action, res.Obs
	}
}

// noStations leaves the map empty. Stations are solid, and ScriptedPolicy can't see them, so
// on the real layout a goal lined up behind one leaves it pushing into it for good. That is
// the blind baseline doing its job, not the env failing, so the env's own contract is tested
// where nothing stands in the way.
func noStations(*rand.Rand) []*game.TradingStation { return nil }

func TestEnvReachesGoal(t *testing.T) {
	e := NewEnv()
	e.Layout = noStations
	for seed := range int64(20) {
		start, err := e.Reset(seed)
		if err != nil {
			t.Fatal(err)
		}
		// The same floor the env reports to training
		optimal := optimalSteps(start.GoalDist)

		res, steps := walkToGoal(t, e, seed)
		if !res.Terminated {
			t.Fatalf("seed %d: goal %v not reached in %d steps", seed, e.goal, steps)
		}
		// Fewer than optimal is impossible: Step ran more than one tick, or the walk started
		// from a stale observation. Above it, goals are random points now, so a bearing up to
		// 22.5° off the nearest action costs 1/cos(22.5°) = 1.082; the +2 covers the integer
		// granularity that dominates short episodes. Measured over 2000 seeds: mean 1.05x,
		// worst 1.14x, all inside this bound.
		if limit := optimal + optimal/8 + 2; steps < optimal || steps > limit {
			t.Errorf("seed %d: took %d steps, optimal is %d, allowed %d", seed, steps, optimal, limit)
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
		res, err := e.Step(bot.ActionStop)
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

	if _, err := e.Step(bot.ActionStop); err == nil {
		t.Error("Step after the episode ended: want error")
	}
}

func TestEnvStepBeforeReset(t *testing.T) {
	if _, err := NewEnv().Step(bot.ActionE); err == nil {
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

// The action integers come from Python over gRPC, so an out-of-range one has to be an error
// rather than an index-out-of-range panic in Action.Vector.
func TestEnvRejectsInvalidAction(t *testing.T) {
	e := NewEnv()
	if _, err := e.Reset(0); err != nil {
		t.Fatal(err)
	}

	for _, a := range []bot.Action{-1, bot.ActionNW + 1, 99} {
		if _, err := e.Step(a); err == nil {
			t.Errorf("Step(%d): want error, got nil", a)
		}
	}

	// A rejected action must not consume the episode
	if _, err := e.Step(bot.ActionN); err != nil {
		t.Errorf("Step after a rejected action: %v", err)
	}
}
