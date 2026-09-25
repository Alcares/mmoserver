package sim

import (
	"context"
	"math/rand"
	"sync"

	simv1 "github.com/alcares/mmoserver/backend/gen/go/sim/v1"
	"github.com/alcares/mmoserver/backend/internal/bot"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	simv1.UnimplementedEnvServer

	mu           sync.Mutex
	environments []*Env
	// One seed stream per env
	seedStreams []*rand.Rand
}

// observation copies an observation into its wire message
func observation(o *bot.Observation) *simv1.Observation {
	v := o.Vectorise()
	return &simv1.Observation{Observation: v[:]}
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Reset(ctx context.Context, r *simv1.ResetRequest) (*simv1.ResetResponse, error) {
	if len(r.Seeds) == 0 {
		return nil, status.Error(codes.InvalidArgument, "seeds must not be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.environments = make([]*Env, len(r.Seeds))
	s.seedStreams = make([]*rand.Rand, len(r.Seeds))
	observations := make([]*simv1.Observation, len(r.Seeds))

	for i, seed := range r.Seeds {
		s.environments[i] = NewEnv()
		s.seedStreams[i] = rand.New(rand.NewSource(seed))

		obs, err := s.environments[i].Reset(seed)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "env %d reset: %v", i, err)
		}
		observations[i] = observation(obs)
	}

	return &simv1.ResetResponse{
		Observations: observations,
		ObsSize:      bot.ObsSize,
		ActionCount:  uint32(bot.ActionCount),
	}, nil
}

func (s *Server) Step(ctx context.Context, r *simv1.StepRequest) (*simv1.StepResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.environments) == 0 {
		return nil, status.Error(codes.FailedPrecondition, "Step before Reset")
	}
	if len(r.Actions) != len(s.environments) {
		return nil, status.Errorf(codes.InvalidArgument,
			"got %d actions, want %d", len(r.Actions), len(s.environments))
	}

	observations := make([]*simv1.Observation, len(r.Actions))
	finalObservations := make([]*simv1.Observation, len(r.Actions))
	rewards := make([]float32, len(r.Actions))
	terminated := make([]bool, len(r.Actions))
	truncated := make([]bool, len(r.Actions))
	optimalSteps := make([]uint32, len(r.Actions))

	for i, action := range r.Actions {
		res, err := s.environments[i].Step(bot.Action(action))
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "env %d: %v", i, err)
		}

		rewards[i] = res.Reward
		terminated[i] = res.Terminated
		truncated[i] = res.Truncated

		if !res.Terminated && !res.Truncated {
			observations[i] = observation(res.Obs)
			finalObservations[i] = &simv1.Observation{}
			continue
		}

		// observations carries the next episode's first observation; the one this episode ended on travels in
		// finalObservations, because PPO bootstraps a truncated episode's value from it.
		finalObservations[i] = observation(res.Obs)
		optimalSteps[i] = uint32(res.OptimalSteps)

		next, err := s.environments[i].Reset(s.seedStreams[i].Int63())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "env %d autoreset: %v", i, err)
		}
		observations[i] = observation(next)
	}

	return &simv1.StepResponse{
		Observations:      observations,
		Rewards:           rewards,
		Terminated:        terminated,
		Truncated:         truncated,
		FinalObservations: finalObservations,
		OptimalSteps:      optimalSteps,
	}, nil
}
