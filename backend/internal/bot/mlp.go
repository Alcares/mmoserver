package bot

import (
	"fmt"
	"math"
	"os"

	botv1 "github.com/alcares/mmoserver/backend/gen/go/bot/v1"
	"google.golang.org/protobuf/proto"
)

// MLPPolicy runs a network exported from training. It satisfies Policy, so it drops in
// anywhere ScriptedPolicy goes: the server, the spectator and the sim.
type MLPPolicy struct {
	layers []mlpLayer
	width  int // Widest layer output, so one scratch size serves every layer.
}

type mlpLayer struct {
	in, out int
	// Row-major, out rows of in each, exactly as torch.nn.Linear stores [out, in]. Flat rather
	// than [][]float32 because that is how it arrives on the wire and how the loop wants it.
	weight []float32
	bias   []float32
}

// LoadMLPPolicy reads a policy exported by rl-training.
func LoadMLPPolicy(path string) (*MLPPolicy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("bot: read policy: %w", err)
	}
	var msg botv1.Policy
	if err := proto.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("bot: parse %s: %w", path, err)
	}
	policy, err := newMLPPolicy(&msg)
	if err != nil {
		return nil, fmt.Errorf("bot: %s: %w", path, err)
	}
	return policy, nil
}

// NewMLPPolicy validates an exported network and prepares it for Act.
//
// The checks are the point. A policy trained against a different observation layout, or one
// whose weights were transposed on export, produces a network that loads and runs and is
// simply wrong - worst of all on a square hidden layer, where the dimensions still agree. Each
// failure below is one that would otherwise surface as a bot that moves badly for no visible
// reason.
func newMLPPolicy(msg *botv1.Policy) (*MLPPolicy, error) {
	if msg.GetObsSize() != ObsSize {
		return nil, fmt.Errorf("policy expects %d observations, this build encodes %d", msg.GetObsSize(), ObsSize)
	}
	if int(msg.GetActionCount()) != ActionCount {
		return nil, fmt.Errorf("policy has %d actions, this build defines %d", msg.GetActionCount(), ActionCount)
	}
	if msg.GetActivation() != botv1.Activation_ACTIVATION_TANH {
		return nil, fmt.Errorf("unsupported activation %v", msg.GetActivation())
	}
	if len(msg.GetLayers()) == 0 {
		return nil, fmt.Errorf("policy has no layers")
	}

	policy := &MLPPolicy{layers: make([]mlpLayer, 0, len(msg.GetLayers()))}
	want := uint32(ObsSize) // each layer's input must be the previous layer's output
	for i, l := range msg.GetLayers() {
		switch {
		case l.GetInFeatures() != want:
			return nil, fmt.Errorf("layer %d takes %d inputs, previous layer emits %d", i, l.GetInFeatures(), want)
		case uint32(len(l.GetWeight())) != l.GetInFeatures()*l.GetOutFeatures():
			return nil, fmt.Errorf("layer %d has %d weights, want %d*%d - exported transposed?",
				i, len(l.GetWeight()), l.GetOutFeatures(), l.GetInFeatures())
		case uint32(len(l.GetBias())) != l.GetOutFeatures():
			return nil, fmt.Errorf("layer %d has %d biases, want %d", i, len(l.GetBias()), l.GetOutFeatures())
		}
		policy.layers = append(policy.layers, mlpLayer{
			in:     int(l.GetInFeatures()),
			out:    int(l.GetOutFeatures()),
			weight: l.GetWeight(),
			bias:   l.GetBias(),
		})
		policy.width = max(policy.width, int(l.GetOutFeatures()))
		want = l.GetOutFeatures()
	}
	if int(want) != ActionCount {
		return nil, fmt.Errorf("last layer emits %d values, want %d actions", want, ActionCount)
	}
	return policy, nil
}

// Act returns the highest-scoring action. No softmax: it is monotonic, so the argmax of the
// logits is the argmax of the probabilities, and a deterministic policy needs nothing else.
func (p *MLPPolicy) Act(observation *Observation) Action {
	vector := observation.Vectorise() // bound first; a function result is not addressable

	src := make([]float32, p.width)
	dst := make([]float32, p.width)
	h := src[:ObsSize]
	copy(h, vector[:])

	last := len(p.layers) - 1
	for i, layer := range p.layers {
		h = layer.forward(h, dst, i != last) // no activation on the logits
		src, dst = dst, src                  // h aliases src, so the next layer writes elsewhere
	}

	best := 0
	for i := 1; i < len(h); i++ {
		if h[i] > h[best] {
			best = i
		}
	}
	return Action(best)
}

// forward computes activation(weight @ x + bias) into dst and returns the part it filled.
func (l mlpLayer) forward(x, dst []float32, activate bool) []float32 {
	for o := range l.out {
		sum := l.bias[o]
		for i, w := range l.weight[o*l.in : (o+1)*l.in] {
			sum += w * x[i]
		}
		if activate {
			// float64 round-trip: more accurate than a float32 tanh, not less.
			sum = float32(math.Tanh(float64(sum)))
		}
		dst[o] = sum
	}
	return dst[:l.out]
}
