package game

import (
	"math/rand"
	"testing"

	pb "github.com/alcares/mmoserver/gen/go/game/v1"
	"google.golang.org/protobuf/proto"
)

// drain decodes every message queued on c.Send without blocking
func drain(t *testing.T, c *Client) []*pb.ServerMessage {
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
	w := NewWorld(rand.New(rand.NewSource(1)))
	c := &Client{Send: make(chan []byte, 32)}

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
	if msgs[1].GetPlayerInventory() == nil {
		t.Errorf("second message = %T, want PlayerInventory", msgs[1].Msg)
	}
	if msgs[2].GetWorldSnapshot() == nil {
		t.Errorf("third message = %T, want WorldSnapshot", msgs[2].Msg)
	}
}

func TestJoinSpawnsAtCentre(t *testing.T) {
	w := NewWorld(rand.New(rand.NewSource(1)))
	for range 3 {
		player, err := w.Join(&Client{Send: make(chan []byte, 32)})
		if err != nil {
			t.Fatal(err)
		}
		if player.Pos != SpawnPos {
			t.Errorf("player %d spawned at %v, want %v", player.ID, player.Pos, SpawnPos)
		}
	}
}

func TestJoinRejectsWhenFull(t *testing.T) {
	w := NewWorld(rand.New(rand.NewSource(1)))
	for range MaxPlayers {
		if _, err := w.Join(&Client{Send: make(chan []byte, 32)}); err != nil {
			t.Fatal(err)
		}
	}

	c := &Client{Send: make(chan []byte, 32)}
	if _, err := w.Join(c); err == nil {
		t.Fatal("Join with MaxPlayers already in: want error")
	}
	if len(w.players) != MaxPlayers || len(c.Send) != 0 {
		t.Errorf("rejected join left %d players and %d queued messages, want %d and none", len(w.players), len(c.Send), MaxPlayers)
	}
}
