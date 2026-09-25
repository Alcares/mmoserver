package game

import pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"

func (w *World) addPlayer(client Client) (*Player, error) {
	if len(w.players) >= MaxPlayers {
		return nil, ErrGameFull
	}

	playerID := w.nextPlayerID
	w.nextPlayerID++
	player := NewPlayer(playerID, SpawnPos)
	// Which client drives it is the only thing that makes a player a bot, and it is decided here.
	_, player.IsBot = client.(*BotClient)

	w.players[playerID] = player
	w.clients[playerID] = client

	w.statusDirty = true

	return player, nil
}

// Join adds a player for c and queues its InitialGameState and PlayerInventory ahead of any WorldSnapshot
func (w *World) Join(c Client) (*Player, error) {
	w.Mu.Lock()
	defer w.Mu.Unlock()

	// Allow late joins
	if w.phase != pb.GamePhase_GAME_PHASE_WAITING && w.phase != pb.GamePhase_GAME_PHASE_COUNTDOWN {
		return nil, ErrGameInProgress
	}

	player, err := w.addPlayer(c)
	if err != nil {
		return nil, err
	}

	w.sendTo(player.ID, &pb.ServerMessage{
		Msg: &pb.ServerMessage_InitialState{InitialState: w.initialState()},
	})
	w.sendTo(player.ID, &pb.ServerMessage{
		Msg: &pb.ServerMessage_PlayerInventory{PlayerInventory: player.ToProtoInventory()},
	})

	return player, nil
}

// initialState describes the current station layout; built per join so it never goes stale
func (w *World) initialState() *pb.InitialGameState {
	stations := make([]*pb.TradingStation, 0, len(w.Stations))
	for _, s := range w.Stations {
		stations = append(stations, s.ToProto())
	}
	return &pb.InitialGameState{StationLayout: stations, GameId: w.gameID, WorldSize: WorldMaxX}
}

func (w *World) removePlayer(playerID uint32) {
	delete(w.players, playerID)
	delete(w.clients, playerID)

	w.statusDirty = true
}
