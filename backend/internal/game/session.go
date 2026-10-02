package game

import (
	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
)

func (w *World) addPlayer(c Client, a Account) (*Player, error) {
	if len(w.players) >= w.config.MaxPlayers {
		return nil, ErrGameFull
	}

	playerID := w.nextPlayerID
	w.nextPlayerID++
	player := NewPlayer(playerID, SpawnPos, a)
	// Which client drives it is the only thing that makes a player a bot, and it is decided here.
	_, player.IsBot = c.(*BotClient)

	w.players[playerID] = player
	w.clients[playerID] = c

	w.statusDirty = true

	return player, nil
}

// Join adds a player for c and queues its InitialGameState and PlayerInventory ahead of any WorldSnapshot
func (w *World) Join(c Client, a Account) (*Player, error) {
	w.Mu.Lock()
	defer w.Mu.Unlock()

	// Allow late joins
	if w.phase != pb.GamePhase_GAME_PHASE_WAITING && w.phase != pb.GamePhase_GAME_PHASE_COUNTDOWN {
		w.logPlayerJoin(nil, ErrGameInProgress)
		return nil, ErrGameInProgress
	}

	player, err := w.addPlayer(c, a)
	if err != nil {
		w.logPlayerJoin(nil, err)
		return nil, err
	}

	w.sendTo(player.ID, &pb.ServerMessage{
		Msg: &pb.ServerMessage_InitialState{InitialState: w.initialState()},
	})
	w.sendTo(player.ID, &pb.ServerMessage{
		Msg: &pb.ServerMessage_PlayerInventory{PlayerInventory: player.ToProtoInventory()},
	})

	w.logPlayerJoin(player, nil)
	return player, nil
}

// initialState describes the current station layout; built per join so it never goes stale
func (w *World) initialState() *pb.InitialGameState {
	stations := make([]*pb.TradingStation, 0, len(w.Stations))
	for _, s := range w.Stations {
		stations = append(stations, s.ToProto())
	}
	return &pb.InitialGameState{
		StationLayout: stations,
		GameId:        w.gameID,
		WorldSize:     WorldMaxX,
		TradeRange:    TradeRange,
		PlayerRadius:  PlayerRadius,
		StationRadius: StationRadius,
	}
}

// Leave removes a player whose client has gone; safe from any goroutine
func (w *World) Leave(playerID uint32) {
	w.Mu.Lock()
	defer w.Mu.Unlock()
	w.removePlayer(playerID)
}

func (w *World) removePlayer(playerID uint32) {
	player := w.players[playerID]
	w.logPlayerLeave(player)

	delete(w.players, playerID)
	if w.phase == pb.GamePhase_GAME_PHASE_RUNNING {
		w.departed[playerID] = player
	}
	delete(w.clients, playerID)

	w.statusDirty = true
}
