package game

import (
	"sort"
	"time"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
)

func (w *World) toProtoGameStatus() *pb.GameStatus {
	var remainingMs int32
	if w.phaseEndTick > w.tick {
		remainingMs = int32(float64(w.phaseEndTick-w.tick) * TickDuration * 1000)
	}

	return &pb.GameStatus{
		Phase:       w.phase,
		PlayerCount: int32(len(w.players)),
		MinPlayers:  int32(w.config.MinPlayers),
		RemainingMs: remainingMs,
	}
}

// setPhase moves to next and marks the status for broadcast; d of 0 means open-ended
func (w *World) setPhase(next pb.GamePhase, d time.Duration) {
	w.phase = next
	if d > 0 {
		w.phaseEndTick = w.tick + ticks(d)
	} else {
		w.phaseEndTick = 0
	}
	w.statusDirty = true
}

// advancePhase runs the round's state machine; caller holds w.Mu.
func (w *World) advancePhase() {
	if w.config.CloseWhenEmpty && len(w.players)-w.countBots() == 0 && w.nextPlayerID > 1 {
		w.finish() // everyone left
		return
	}
	switch w.phase {
	case pb.GamePhase_GAME_PHASE_WAITING:
		if len(w.players) >= w.config.MinPlayers {
			w.setPhase(pb.GamePhase_GAME_PHASE_COUNTDOWN, w.config.StartCountdown)
		} else if w.tick > ticks(w.config.LobbyTTL) {
			w.finish() // nobody ever joined
		}
	case pb.GamePhase_GAME_PHASE_COUNTDOWN:
		if w.tick >= w.phaseEndTick {
			w.setPhase(pb.GamePhase_GAME_PHASE_RUNNING, w.config.Duration)
			if w.config.RandomEvents {
				w.scheduleEvents()
			}
		}
	case pb.GamePhase_GAME_PHASE_RUNNING:
		if w.tick >= w.phaseEndTick {
			w.finish()
		}
	}
}

func (w *World) netWorth(p *Player) uint64 {
	total := p.balance
	for cType, units := range p.commodities {
		if units == 0 {
			continue
		}
		total += units * w.Commodities[cType].sellPrice(units)
	}
	return total
}

func (w *World) finish() {
	w.setPhase(pb.GamePhase_GAME_PHASE_FINISHED, 0)

	standings := make([]*pb.PlayerFinalStanding, 0, len(w.players))
	for _, player := range w.players {
		standings = append(standings, &pb.PlayerFinalStanding{
			Id:               player.ID,
			Name:             player.Name,
			NetWorth:         w.netWorth(player),
			TradeVolumeCents: player.tradeVolume,
			UnitsTraded:      player.unitsTraded,
		})
	}
	sort.Slice(standings, func(i, j int) bool {
		return standings[i].NetWorth > standings[j].NetWorth
	})

	w.endAllEvents()

	w.statusDirty = false
	w.sendToAll(&pb.ServerMessage{
		Msg: &pb.ServerMessage_GameStatus{GameStatus: w.toProtoGameStatus()},
	})
	w.sendToAll(&pb.ServerMessage{
		Msg: &pb.ServerMessage_GameOver{GameOver: &pb.GameOver{Standings: standings}},
	})
}

func (w *World) countBots() int {
	n := 0
	for _, player := range w.players {
		if player.IsBot {
			n++
		}
	}
	return n
}
