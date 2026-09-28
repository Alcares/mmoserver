package game

import (
	"errors"
	"math"
	"math/rand"
	"testing"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
	"github.com/alcares/mmoserver/backend/internal/geometry"
	"google.golang.org/protobuf/proto"
)

func testWorld(t *testing.T) *World {
	t.Helper()
	return NewWorld("test", WorldConfig{Rng: rand.New(rand.NewSource(1))})
}

// drain decodes every message queued on c.Send without blocking
func drain(t *testing.T, c *SendQueue) []*pb.ServerMessage {
	t.Helper()
	var msgs []*pb.ServerMessage
	for {
		select {
		case payload := <-c.Send:
			var msg pb.ServerMessage
			if err := proto.Unmarshal(payload, &msg); err != nil {
				t.Fatal(err)
			}
			msgs = append(msgs, &msg)
		default:
			return msgs
		}
	}
}

func TestJoinSendsStationsBeforeSnapshots(t *testing.T) {
	w := testWorld(t)
	c := NewSendQueue()

	if _, err := w.Join(c); err != nil {
		t.Fatal(err)
	}
	w.Tick(NewSpatialGrid())

	msgs := drain(t, c)
	if len(msgs) < 3 {
		t.Fatalf("got %d messages, want at least 3", len(msgs))
	}

	initial := msgs[0].GetInitialState()
	if initial == nil {
		t.Fatalf("first message = %T, want InitialState", msgs[0].Msg)
	}
	if len(initial.StationLayout) != len(w.Stations) {
		t.Errorf("station layout has %d stations, want %d", len(initial.StationLayout), len(w.Stations))
	}
	// Clients and the bot use these instead of keeping their own copies
	if initial.TradeRange != TradeRange || initial.PlayerRadius != PlayerRadius || initial.StationRadius != StationRadius {
		t.Errorf("initial state sends trade range %v, player radius %v, station radius %v; want %v, %v, %v",
			initial.TradeRange, initial.PlayerRadius, initial.StationRadius, TradeRange, PlayerRadius, StationRadius)
	}
	if msgs[1].GetPlayerInventory() == nil {
		t.Errorf("second message = %T, want PlayerInventory", msgs[1].Msg)
	}
	if msgs[2].GetWorldSnapshot() == nil {
		t.Errorf("third message = %T, want WorldSnapshot", msgs[2].Msg)
	}
}

func TestJoinSpawnsAtCentre(t *testing.T) {
	w := testWorld(t)
	for range 3 {
		player, err := w.Join(NewSendQueue())
		if err != nil {
			t.Fatal(err)
		}
		if player.Pos != SpawnPos {
			t.Errorf("player %d spawned at %v, want %v", player.ID, player.Pos, SpawnPos)
		}
	}
}

// A world takes MaxPlayers joins and refuses the next without queueing it anything
func TestJoinRejectsWhenFull(t *testing.T) {
	const maxPlayers = 3
	w := NewWorld("test", WorldConfig{Rng: rand.New(rand.NewSource(1)), MaxPlayers: maxPlayers})
	for range maxPlayers {
		if _, err := w.Join(NewSendQueue()); err != nil {
			t.Fatal(err)
		}
	}

	c := NewSendQueue()
	if _, err := w.Join(c); !errors.Is(err, ErrGameFull) {
		t.Fatalf("Join with %d players already in: got %v, want ErrGameFull", maxPlayers, err)
	}
	if len(w.players) != maxPlayers || len(c.Send) != 0 {
		t.Errorf("rejected join left %d players and %d queued messages, want %d and none", len(w.players), len(c.Send), maxPlayers)
	}
}

// Unset or out-of-range MaxPlayers falls back to PlayerLimit
func TestMaxPlayersDefaultsToLimit(t *testing.T) {
	for _, maxPlayers := range []int{0, -1, PlayerLimit + 1} {
		w := NewWorld("test", WorldConfig{Rng: rand.New(rand.NewSource(1)), MaxPlayers: maxPlayers})
		if w.config.MaxPlayers != PlayerLimit {
			t.Errorf("MaxPlayers %d became %d, want %d", maxPlayers, w.config.MaxPlayers, PlayerLimit)
		}
	}
}

// MaxPlayers below MinPlayers is raised to it, so the game can still fill up enough to start
func TestMaxPlayersRaisedToMin(t *testing.T) {
	w := NewWorld("test", WorldConfig{Rng: rand.New(rand.NewSource(1)), MinPlayers: 3, MaxPlayers: 2})
	if w.config.MaxPlayers != 3 {
		t.Errorf("MaxPlayers 2 with MinPlayers 3 became %d, want 3", w.config.MaxPlayers)
	}
}

func TestMovementDistanceTraveled(t *testing.T) {
	w := testWorld(t)

	c := NewSendQueue()
	player, err := w.Join(c)
	if err != nil {
		t.Fatal(err)
	}
	const gameTicks = 10
	targetDistance := gameTicks * MoveSpeed * TickDuration

	for _, pos := range []geometry.Vec2f{{X: 1.0, Y: 0.0}, {X: 0.0, Y: 1.0}, {X: 1.0, Y: 1.0}} {
		// Each leg from the spawn, so the walks don't add up into a station
		player.Pos = SpawnPos
		startingPos := player.Pos
		w.EnqueueMovement(PlayerMovementInput{
			PlayerID: player.ID,
			Vx:       pos.X,
			Vy:       pos.Y,
		})

		grid := NewSpatialGrid()
		for range gameTicks {
			w.Tick(grid)
		}

		actualDistance := math.Sqrt(math.Pow(player.Pos.X-startingPos.X, 2) + math.Pow(player.Pos.Y-startingPos.Y, 2))

		const epsilon = 1e-9
		if math.Abs(actualDistance-targetDistance) > epsilon {
			t.Errorf("player position advanced %v, wanted: %v", actualDistance, targetDistance)
		}
	}

}
