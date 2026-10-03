package game

import (
	"slices"
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

func (w *World) finish() {
	w.setPhase(pb.GamePhase_GAME_PHASE_FINISHED, 0)

	players := slices.Collect(w.everyone())

	commodityUnits := make(map[pb.CommodityType]uint64, len(GetCommodityTypes()))
	commodityPrice := make(map[pb.CommodityType]uint64, len(GetCommodityTypes()))
	netWorth := make(map[uint32]uint64, len(players))

	for _, player := range players {
		for cType, count := range player.commodities {
			commodityUnits[cType] += count
		}
	}

	for _, cType := range GetCommodityTypes() {
		commodityPrice[cType] = w.Commodities[cType].sellPrice(commodityUnits[cType])
	}

	for _, player := range players {
		netWorth[player.ID] = playerNetWorth(player, commodityPrice)
	}

	sort.Slice(players, func(i, j int) bool {
		return netWorth[players[i].ID] > netWorth[players[j].ID]
	})

	standings := make([]*pb.PlayerFinalStanding, 0, len(players))
	for i, player := range players {
		standings = append(standings, &pb.PlayerFinalStanding{
			Id:               player.ID,
			Name:             player.Name,
			NetWorth:         netWorth[player.ID],
			TradeVolumeCents: player.tradeVolume,
			UnitsTraded:      player.unitsTraded,
		})
		w.logStanding(i+1, player, netWorth[player.ID])
	}

	// Only matchmade games are rated: a private one could be set up with alt accounts to farm rating
	if w.config.IsPublic {
		w.updateElo(standings)
	}

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

func playerNetWorth(p *Player, prices map[pb.CommodityType]uint64) uint64 {
	total := p.balance
	for cType, units := range p.commodities {
		total += units * prices[cType]
	}
	return total
}
