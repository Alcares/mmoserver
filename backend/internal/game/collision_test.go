package game

import (
	"math"
	"math/rand"
	"testing"

	"github.com/alcares/mmoserver/backend/internal/geometry"
)

// contact is how close a player's centre can get to a station's
const contact = PlayerRadius + StationRadius

// TestContactWithinTradeRange: the closest a player can stand to a station has to be close
// enough to trade from, or collisions lock every station.
func TestContactWithinTradeRange(t *testing.T) {
	if contact >= TradeRange {
		t.Errorf("PlayerRadius+StationRadius = %v, not below TradeRange %v: the closest a player can stand is out of trading range",
			contact, TradeRange)
	}
}

// oneStation returns a world whose only station is at pos, and a player joined into it.
func oneStation(t *testing.T, pos geometry.Vec2f) (*World, *Player) {
	t.Helper()
	w := NewWorld("test", WorldConfig{
		Rng: rand.New(rand.NewSource(1)),
		Layout: func(*rand.Rand) []*TradingStation {
			return []*TradingStation{{Label: "TEST", Pos: pos}}
		},
	})
	player, err := w.Join(NewSendQueue(), Account{})
	if err != nil {
		t.Fatal(err)
	}
	return w, player
}

// walk holds dir for ticks, failing the test if any tick ends with the player inside a station.
func walk(t *testing.T, w *World, player *Player, dir geometry.Vec2f, ticks int) {
	t.Helper()
	w.EnqueueMovement(PlayerMovementInput{PlayerID: player.ID, Vx: dir.X, Vy: dir.Y})
	grid := NewSpatialGrid()
	for tick := 1; tick <= ticks; tick++ {
		w.Tick(grid)
		assertOutsideStations(t, w, player, tick)
	}
}

func assertOutsideStations(t *testing.T, w *World, player *Player, tick int) {
	t.Helper()
	const epsilon = 1e-9
	for _, s := range w.Stations {
		if d := geometry.EuclideanDistance(player.Pos, s.Pos); d < contact-epsilon {
			t.Fatalf("tick %d: player at %v is %.4f from station %s at %v, closer than contact %v",
				tick, player.Pos, d, s.Label, s.Pos, contact)
		}
	}
}

func TestCollisionHeadOnStopsAtContact(t *testing.T) {
	station := geometry.Vec2f{X: 150, Y: 125}
	w, player := oneStation(t, station)
	player.Pos = geometry.Vec2f{X: station.X - contact - 5, Y: station.Y}

	// 5 units to cover, then 25 more ticks of pushing straight into it
	walk(t, w, player, geometry.Vec2f{X: 1, Y: 0}, 34)

	want := geometry.Vec2f{X: station.X - contact, Y: station.Y}
	if geometry.EuclideanDistance(player.Pos, want) > 1e-9 {
		t.Errorf("pushing into a station's centre ended at %v, want to rest at contact %v", player.Pos, want)
	}
}

func TestCollisionOnCentrePushesNorth(t *testing.T) {
	station := geometry.Vec2f{X: 150, Y: 125}
	w, player := oneStation(t, station)
	player.Pos = station // no direction from the centre to push along

	walk(t, w, player, geometry.Vec2f{}, 1)

	want := geometry.Vec2f{X: station.X, Y: station.Y - contact} // +y is south
	if geometry.EuclideanDistance(player.Pos, want) > 1e-9 {
		t.Errorf("a player on a station's centre ended at %v, want %v", player.Pos, want)
	}
}

func TestCollisionGlancingSlidesPast(t *testing.T) {
	station := geometry.Vec2f{X: 150, Y: 125}
	w, player := oneStation(t, station)
	// Heading east, a third of the contact distance south of the centre line: a hit, not a miss
	start := geometry.Vec2f{X: station.X - contact - 5, Y: station.Y + contact/3}
	player.Pos = start

	const ticks = 40 // 24 units: enough to clear the station if the slide works
	walk(t, w, player, geometry.Vec2f{X: 1, Y: 0}, ticks)

	if player.Pos.X <= station.X+contact {
		t.Fatalf("a glancing hit ended at %v, still not past the station at %v; it should slide around", player.Pos, station)
	}
	if player.Pos.Y < start.Y {
		t.Errorf("slid north to %v; it started south of the centre line, so it should go round the south side", player.Pos)
	}
	// The slide costs distance along x, never more than the straight walk would have covered
	if straight := start.X + ticks*MoveSpeed*TickDuration; player.Pos.X > straight+1e-9 {
		t.Errorf("ended at x=%v, past the %v an unobstructed walk reaches; the push added speed", player.Pos.X, straight)
	}
}

// Every station, approached straight at its centre from 16 directions, can be traded at:
// the player stops at contact, which has to be within TradeRange, and nearestStation, which
// gates every trade, has to find that station from there.
func TestCollisionEveryStationTradeableFromEverySide(t *testing.T) {
	w := testWorld(t)
	player, err := w.Join(NewSendQueue(), Account{})
	if err != nil {
		t.Fatal(err)
	}

	const directions = 16
	for _, s := range w.Stations {
		for i := range directions {
			angle := 2 * math.Pi * float64(i) / directions
			out := geometry.Vec2f{X: math.Cos(angle), Y: math.Sin(angle)}
			player.Pos = geometry.Vec2f{X: s.Pos.X + out.X*(contact+5), Y: s.Pos.Y + out.Y*(contact+5)}

			walk(t, w, player, geometry.Vec2f{X: -out.X, Y: -out.Y}, 30)

			if d := geometry.EuclideanDistance(player.Pos, s.Pos); d > TradeRange {
				t.Errorf("station %s from %.0f°: stopped %.2f from it, outside TradeRange %v", s.Label, angle*180/math.Pi, d, TradeRange)
			}
			if got := w.nearestStation(player.Pos); got != s {
				t.Errorf("station %s from %.0f°: nearestStation at %v is %v, want this station", s.Label, angle*180/math.Pi, player.Pos, got)
			}
		}
	}
}

// A long walk on the real layout never ends a tick inside a station. Half the legs head at a
// station, so contacts from every angle are guaranteed rather than left to chance.
func TestCollisionRandomWalkNeverEndsInsideStation(t *testing.T) {
	w := testWorld(t)
	player, err := w.Join(NewSendQueue(), Account{})
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(7))
	grid := NewSpatialGrid()

	contacts := 0
	for tick := 1; tick <= 20_000; tick++ {
		if tick%20 == 1 { // a new leg every second
			var dir geometry.Vec2f
			if rng.Intn(2) == 0 {
				s := w.Stations[rng.Intn(len(w.Stations))]
				dir = geometry.Vec2f{X: s.Pos.X - player.Pos.X, Y: s.Pos.Y - player.Pos.Y}
				dir.X, dir.Y = dir.X+rng.NormFloat64(), dir.Y+rng.NormFloat64() // not always dead-centre
			} else {
				angle := rng.Float64() * 2 * math.Pi
				dir = geometry.Vec2f{X: math.Cos(angle), Y: math.Sin(angle)}
			}
			w.EnqueueMovement(PlayerMovementInput{PlayerID: player.ID, Vx: dir.X, Vy: dir.Y})
		}

		w.Tick(grid)
		assertOutsideStations(t, w, player, tick)
		for _, s := range w.Stations {
			if geometry.EuclideanDistance(player.Pos, s.Pos) < contact+1e-6 {
				contacts++
			}
		}
	}

	// Otherwise the walk never touched a station and the assertion above proved nothing
	if contacts < 100 {
		t.Errorf("only %d ticks ended touching a station; the walk is not exercising collisions", contacts)
	}
}
