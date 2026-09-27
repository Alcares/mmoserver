package bot

import "math"

// Action is what a policy decides: one of the 8 directions a keyboard can send. There is no
// stop: standing still is for whoever decides the bot has arrived, not for the steering policy.
// The integers are the contract with Python training pipeline.
type Action int

type Policy interface {
	Act(observation *Observation) Action
}

// ScriptedPolicy heads straight for the goal: the nearest of the 8 directions. It is the baseline a trained policy has to beat, and the control that says whether
// a failing training run is the algorithm or the environment.
//
// Quantising to 45° leaves the heading up to 22.5° off the true bearing, so it makes at worst
// cos(22.5°) ≈ 0.924 of full-speed progress per tick. Taking ~8% more ticks than a straight
// line is the floor for any 8-direction policy, not a fault in this one.
type ScriptedPolicy struct{}

func (p ScriptedPolicy) Act(observation *Observation) Action {
	// Atan2(dx, -dy) is the bearing clockwise from north, which is the order the Action
	// constants are declared in, so the sector is the Action.
	bearing := math.Atan2(float64(observation.goalDX), float64(-observation.goalDY))
	sector := int(math.Round(bearing / (math.Pi / 4)))

	return Action((sector%8 + 8) % 8) // Round gives -4..4
}

// Clockwise from north. The world's +Y points south.
const (
	ActionN Action = iota
	ActionNE
	ActionE
	ActionSE
	ActionS
	ActionSW
	ActionW
	ActionNW
)

// actionVectors is the one place an Action becomes a direction. Diagonals stay at (±1, ±1),
// the way two held keys arrive; the world normalizes anything longer than unit length itself.
var actionVectors = [...][2]float64{
	ActionN:  {0, -1},
	ActionNE: {1, -1},
	ActionE:  {1, 0},
	ActionSE: {1, 1},
	ActionS:  {0, 1},
	ActionSW: {-1, 1},
	ActionW:  {-1, 0},
	ActionNW: {-1, -1},
}

// ActionCount is how many actions the table defines, and so the width of a policy network's output layer
const ActionCount = len(actionVectors)

// Valid reports whether a indexes the table. The integers are the contract with Python, so a
// caller taking them from outside Go checks here rather than letting Vector panic.
func (a Action) Valid() bool {
	return a >= ActionN && int(a) < len(actionVectors)
}

// Vector returns the movement input ready for EnqueueMovement. Out-of-range actions panic;
// check Valid first when the action did not come from this package.
func (a Action) Vector() (vx, vy float64) {
	v := actionVectors[a]
	return v[0], v[1]
}
