package game

import (
	"cmp"
	"log/slog"
	"math"
	"slices"
	"time"
)

const (
	matchInterval = 1 * time.Second
	// fillWait is how long a group short of MaxPlayers waits for more players before it starts anyway
	fillWait = 60 * time.Second

	baseTolerance = 25
	maxTolerance  = 250
	timeToMax     = 120 * time.Second
)

// queueable is a client that can wait for a public game
type queueable interface {
	Client
	InWorld() (*World, uint32)
}

// ticket is one player waiting in the queue for a public game
type ticket struct {
	client   Client
	account  Account
	queuedAt time.Time
	onMatch  func(*World, *Player)
}

// matchTolerance is how wide a rating range a player accepts in their game; it widens as they wait
func matchTolerance(waited time.Duration) float64 {
	curr := (waited.Seconds() / timeToMax.Seconds()) * maxTolerance
	return min(curr+baseTolerance, maxTolerance)
}

// Enqueue queues c for a public game. onMatch runs once c has joined one, on the matchmaker's goroutine
func (m *Master) Enqueue(c queueable, a Account, onMatch func(*World, *Player)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// check InWorld() to prevent data races which could cause players to double enqueue
	if w, _ := c.InWorld(); w != nil || slices.ContainsFunc(m.queue, func(t *ticket) bool { return t.client == c }) {
		return
	}
	m.queue = append(m.queue, &ticket{client: c, account: a, queuedAt: time.Now(), onMatch: onMatch})
}

// Dequeue takes c out of the queue; once it returns, c is never matched unless queued again
func (m *Master) Dequeue(c Client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.queue = slices.DeleteFunc(m.queue, func(t *ticket) bool { return t.client == c })
}

// RunMatchmaker forms public games from the queue with config, forever
func (m *Master) RunMatchmaker(config WorldConfig) {
	config.IsPublic = true
	config.sanitize()

	t := time.NewTicker(matchInterval)
	defer t.Stop()

	for now := range t.C {
		m.matchQueue(now, config)
	}
}

func (m *Master) matchQueue(now time.Time, config WorldConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, group := range formGroups(m.queue, now, config.MinPlayers, config.MaxPlayers) {
		world, err := m.register(config)
		if err != nil {
			// The group stays queued and is retried once a game ends
			slog.Warn("matchmaking paused", "err", err, "queued", len(m.queue))
			return
		}
		// Everyone joins before the world runs, so it can't start its countdown on a half-joined group
		for _, t := range group {
			player, err := world.Join(t.client, t.account)
			if err != nil {
				slog.Error("matched player failed to join", "game", world.gameID, "err", err)
				continue
			}
			t.onMatch(world, player)
		}
		m.queue = slices.DeleteFunc(m.queue, func(t *ticket) bool { return slices.Contains(group, t) })
		m.start(world)
	}
}

// formGroups picks the groups that should start a game now, longest-waiting player first.
// A group's rating range fits the narrowest tolerance in it, so nobody gets a wider match than
// they have waited for. A group short of maxPlayers forms only once its first player has waited fillWait.
// queue must be in the order players joined it, which Enqueue keeps.
func formGroups(queue []*ticket, now time.Time, minPlayers, maxPlayers int) [][]*ticket {
	n := len(queue)

	// In rating order, a player's closest-rated rivals are its neighbours; ties keep queue order
	order := make([]int, n)
	for q := range order {
		order[q] = q
	}
	slices.SortFunc(order, func(a, b int) int {
		return cmp.Or(cmp.Compare(queue[a].account.Rating, queue[b].account.Rating), cmp.Compare(a, b))
	})
	byRating := make([]*ticket, n)
	position := make([]int, n) // position[q] is where queue[q] sits in byRating
	for i, q := range order {
		byRating[i] = queue[q]
		position[q] = i
	}

	// prev and next link the players not in a group yet, so a walk never steps over a grouped one
	prev, next := make([]int, n), make([]int, n)
	for i := range n {
		prev[i], next[i] = i-1, i+1
	}
	taken := make([]bool, n)

	var groups [][]*ticket
	for q, anchor := range queue {
		i := position[q]
		if taken[i] {
			continue
		}

		members := []int{i} // positions in byRating
		lowest, highest := anchor.account.Rating, anchor.account.Rating
		tolerance := matchTolerance(now.Sub(anchor.queuedAt))

		// Walk outwards from the anchor, always to the closer-rated side
		l, r := prev[i], next[i]
		for len(members) < maxPlayers {
			// A side is done once its next player is out of reach: the range only grows and the tolerance only shrinks
			if l >= 0 && highest-byRating[l].account.Rating > tolerance {
				l = -1
			}
			if r < n && byRating[r].account.Rating-lowest > tolerance {
				r = n
			}
			if l < 0 && r >= n {
				break
			}

			j := r
			if r >= n || (l >= 0 && closer(anchor, byRating[l], byRating[r])) {
				j = l
				l = prev[l]
			} else {
				r = next[r]
			}

			t := byRating[j]
			lo, hi := min(lowest, t.account.Rating), max(highest, t.account.Rating)
			tol := min(tolerance, matchTolerance(now.Sub(t.queuedAt)))
			if hi-lo > tol {
				continue
			}
			members = append(members, j)
			lowest, highest, tolerance = lo, hi, tol
		}

		full := len(members) == maxPlayers
		if len(members) < minPlayers || (!full && now.Sub(anchor.queuedAt) < fillWait) {
			continue
		}
		group := make([]*ticket, len(members))
		for k, j := range members {
			group[k] = byRating[j]
			taken[j] = true
			if prev[j] >= 0 {
				next[prev[j]] = next[j]
			}
			if next[j] < n {
				prev[next[j]] = prev[j]
			}
		}
		groups = append(groups, group)
	}
	return groups
}

// closer reports whether a is closer in rating to anchor than b is, the longer-waiting one on a tie
func closer(anchor, a, b *ticket) bool {
	da := math.Abs(a.account.Rating - anchor.account.Rating)
	db := math.Abs(b.account.Rating - anchor.account.Rating)
	if da != db {
		return da < db
	}
	return a.queuedAt.Before(b.queuedAt)
}
