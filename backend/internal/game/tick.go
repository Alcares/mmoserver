package game

import (
	"math"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
)

func (w *World) Tick(grid *SpatialGrid) {
	w.tick++
	w.advancePhase()

	w.drainInputs()
	w.stepMovement()
	w.replicate(grid)
	w.broadcastMarket()

	if w.statusDirty {
		w.statusDirty = false
		w.sendToAll(&pb.ServerMessage{
			Msg: &pb.ServerMessage_GameStatus{GameStatus: w.toProtoGameStatus()},
		})
	}
}

// drainInputs empties both queues, storing the latest target direction vector per player
// and filling any trade order that arrived since the last tick
func (w *World) drainInputs() {
	for {
		select {
		case input := <-w.movementQueue:
			player, exists := w.players[input.PlayerID]
			if !exists {
				continue
			}

			dirX := clampFloat(input.Vx, -1.0, 1.0)
			dirY := clampFloat(input.Vy, -1.0, 1.0)

			// Normalize diagonal movement to prevent moving faster diagonally
			lenSq := dirX*dirX + dirY*dirY
			if lenSq > 1.0 {
				invLen := 1.0 / math.Sqrt(lenSq)
				dirX *= invLen
				dirY *= invLen
			}

			player.TargetDir.X = dirX
			player.TargetDir.Y = dirY

		case order := <-w.tradeQueue:
			player, exists := w.players[order.PlayerID]
			if !exists {
				continue
			}

			receipt := w.executeTrade(player, order)
			w.sendTo(order.PlayerID, &pb.ServerMessage{Msg: &pb.ServerMessage_Trade{Trade: receipt}})
			if receipt.Success {
				w.sendTo(order.PlayerID, &pb.ServerMessage{
					Msg: &pb.ServerMessage_PlayerInventory{PlayerInventory: player.ToProtoInventory()},
				})
			}

		default:
			return
		}
	}
}

// stepMovement is the continuous physics update: Pos += Velocity * dt, clamped to the map
func (w *World) stepMovement() {
	for _, player := range w.players {
		player.Pos.X += player.TargetDir.X * player.Speed * TickDuration
		player.Pos.Y += player.TargetDir.Y * player.Speed * TickDuration

		w.pushOutOfStations(player)

		// Keep within map boundaries
		if player.Pos.X < WorldMinX {
			player.Pos.X = WorldMinX
		}
		if player.Pos.X > WorldMaxX {
			player.Pos.X = WorldMaxX
		}
		if player.Pos.Y < WorldMinY {
			player.Pos.Y = WorldMinY
		}
		if player.Pos.Y > WorldMaxY {
			player.Pos.Y = WorldMaxY
		}
	}
}

// pushOutOfStations moves a player that ended its step inside a station back onto the station's
// edge, along the line from its centre.
func (w *World) pushOutOfStations(player *Player) {
	const minDist = PlayerRadius + StationRadius
	for _, station := range w.Stations {
		if EuclideanDistance(player.Pos, station.Pos) < minDist {
			player.Pos = projectOntoCircle(station.Pos, player.Pos, minDist)
		}
	}
}

// replicate re-indexes positions into spatial partitions, then sends every client the
// WorldSnapshot for its own area of interest
func (w *World) replicate(grid *SpatialGrid) {
	grid.Clear()
	for _, player := range w.players {
		grid.Insert(player.ID, player.Pos)
	}

	for id := range w.clients {
		player, exists := w.players[id]
		if !exists {
			continue
		}

		candidates := grid.QueryRadius(player.Pos, DefaultFOVRadius)

		// Add self first
		protoPlayers := []*pb.PlayerState{player.ToProtoState()}

		// Fine-grained narrow phase: Euclidean distance filter
		maxDistSq := DefaultFOVRadius * DefaultFOVRadius
		for _, id := range candidates {
			if id == player.ID {
				continue // Skip self (already added)
			}
			other, ok := w.players[id]
			if !ok {
				continue
			}

			dx := other.Pos.X - player.Pos.X
			dy := other.Pos.Y - player.Pos.Y
			if (dx*dx + dy*dy) <= maxDistSq {
				protoPlayers = append(protoPlayers, other.ToProtoState())
			}
		}

		msg := &pb.ServerMessage{
			Msg: &pb.ServerMessage_WorldSnapshot{
				WorldSnapshot: &pb.WorldSnapshot{
					Tick:    w.tick,
					Players: protoPlayers,
				},
			},
		}

		w.sendTo(id, msg)
	}
}

// broadcastMarket sends the market state: identical for every client, so marshal once
func (w *World) broadcastMarket() {
	quotes := make([]*pb.PriceQuote, 0, len(w.Commodities))
	for cType, state := range w.Commodities {
		price := state.buyPrice(1)

		var deltaBasisPoints int32
		if state.lastPrice > 0 {
			deltaBasisPoints = int32((int64(price) - int64(state.lastPrice)) * 10000 / int64(state.lastPrice))
		}
		state.lastPrice = price

		quotes = append(quotes, state.toProtoQuote(cType, deltaBasisPoints))
	}

	w.sendToAll(&pb.ServerMessage{
		Msg: &pb.ServerMessage_MarketState{
			MarketState: &pb.MarketState{
				Tick:   w.tick,
				Quotes: quotes,
			},
		},
	})
}
