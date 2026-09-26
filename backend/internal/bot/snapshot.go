package bot

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

// snapshotPattern matches ExportSnapshots' policy_<timesteps>.pb.
const snapshotPattern = `^policy_(\d+)\.pb$`

var snapshotName = regexp.MustCompile(snapshotPattern)

// SnapshotPolicy plays a training run's exported policies in order, one per Reload.
// Not safe for concurrent use.
type SnapshotPolicy struct {
	dir      string
	fallback Policy
	current  Policy
	shown    int64 // timesteps of the snapshot in current; -1 before the first
	newest   int64 // timesteps of the newest snapshot in dir at the last Reload; -1 if none
}

// NewSnapshotPolicy runs fallback until a snapshot loads, and loads the earliest one right away.
func NewSnapshotPolicy(dir string, fallback Policy) *SnapshotPolicy {
	p := &SnapshotPolicy{dir: dir, fallback: fallback, current: fallback, shown: -1, newest: -1}
	p.Reload()
	return p
}

func (p *SnapshotPolicy) Act(observation *Observation) Action {
	return p.current.Act(observation)
}

// Timesteps is the current snapshot's training step, or -1 on the fallback.
func (p *SnapshotPolicy) Timesteps() int64 { return p.shown }

// NewestTimesteps is the newest snapshot's training step, or -1 if there is none.
func (p *SnapshotPolicy) NewestTimesteps() int64 { return p.newest }

func (p *SnapshotPolicy) String() string {
	if p.shown < 0 {
		return "fallback (no snapshot in " + p.dir + " yet)"
	}
	return fmt.Sprintf("snapshot %d", p.shown)
}

// Reload advances to the oldest snapshot newer than the current one.
func (p *SnapshotPolicy) Reload() {
	entries, err := os.ReadDir(p.dir)
	if err != nil {
		return // no archive yet
	}

	var next int64 = -1
	for _, e := range entries {
		m := snapshotName.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		steps, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			continue
		}
		p.newest = max(p.newest, steps)
		if steps > p.shown && (next < 0 || steps < next) {
			next = steps
		}
	}

	if next < 0 {
		return
	}

	path := filepath.Join(p.dir, fmt.Sprintf("policy_%d.pb", next))
	policy, err := LoadMLPPolicy(path)
	if err != nil {
		// Skip it rather than retry it forever.
		log.Printf("Snapshot %s: %v", path, err)
		p.shown = next
		return
	}

	p.current, p.shown = policy, next
}
