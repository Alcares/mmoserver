package sim

import "testing"

// BenchmarkEpisode measures whole episodes: one Reset, which builds a fresh World, plus the
// Steps the perfect policy needs to walk to the goal. steps/s is the number that decides
// whether the Python bridge has to batch environments.
func BenchmarkEpisode(b *testing.B) {
	e := NewEnv()
	episodes, steps := 0, 0

	for seed := int64(0); b.Loop(); seed++ {
		_, n := walkToGoal(b, e, seed)
		episodes++
		steps += n
	}

	b.ReportMetric(float64(steps)/b.Elapsed().Seconds(), "steps/s")
	b.ReportMetric(float64(steps)/float64(episodes), "steps/ep")
}

// BenchmarkStep measures one Step alone: Tick, marshal, drain, Unmarshal, Encode. Restarting
// a finished episode is excluded from the timer, so ns/op here is the per-step cost the
// bridge's batch size has to beat. Subtract it from BenchmarkEpisode to price a Reset.
func BenchmarkStep(b *testing.B) {
	e := NewEnv()
	seed := int64(0)
	obs, err := e.Reset(seed)
	if err != nil {
		b.Fatal(err)
	}

	for b.Loop() {
		v := obs.Vectorise()
		res, err := e.Step(float64(v[2]/v[4]), float64(v[3]/v[4]))
		if err != nil {
			b.Fatal(err)
		}
		obs = res.Obs

		if res.Terminated || res.Truncated {
			b.StopTimer()
			seed++
			obs, err = e.Reset(seed)
			if err != nil {
				b.Fatal(err)
			}
			b.StartTimer()
		}
	}
}
