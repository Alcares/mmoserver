package game

import (
	"math"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
	"github.com/alcares/mmoserver/backend/internal/geometry"
)

func (w *World) Tick(grid *SpatialGrid) {
	w.tick++
	w.advancePhase()
	if w.config.RandomEvents {
		w.randomEvents()
	}

	w.drainInputs()
	w.stepMovement()
	w.poolTrade() // pool after player trades - else player trade would be rejected
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
		if geometry.EuclideanDistance(player.Pos, station.Pos) < minDist {
			player.Pos = geometry.ProjectOntoCircle(station.Pos, player.Pos, minDist)
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

func (w *World) randomEvents() {
	if w.phase != pb.GamePhase_GAME_PHASE_RUNNING {
		return
	}

	if len(w.eventTicks) > 0 && w.tick >= w.eventTicks[0] {
		w.eventTicks = w.eventTicks[1:]
		w.startNextEvent()
	}
	for id, e := range w.activeEvents {
		if w.tick >= e.endTick {
			w.endEvent(id)
		}
	}
}

// poolTrade makes the trades floods and shortages push through their pools, on the tick each
// starts and at a fixed interval after, to keep their own move on target
func (w *World) poolTrade() {
	for id, e := range w.activeEvents {
		var trade func(*CommodityState, uint64) uint64
		var intent pb.OrderIntent

		switch e.eventType {
		case pb.RandomCommodityEventType_RANDOM_COMMODITY_EVENT_SUPPLY_FLOOD:
			trade, intent = (*CommodityState).sell, pb.OrderIntent_INTENT_SELL
		case pb.RandomCommodityEventType_RANDOM_COMMODITY_EVENT_SUPPLY_SHORTAGE:
			trade, intent = (*CommodityState).buy, pb.OrderIntent_INTENT_BUY
		default:
			continue
		}

		if w.tick != e.startTick && w.tick%poolTradeTickFrequency != 0 {
			continue
		}

		pool := w.Commodities[e.commodity]
		soFar := float64(e.movedBasisPoints) / wholeBasisPoints
		target := e.target(w.tick)
		if math.Abs(target-1) <= math.Abs(soFar-1) {
			continue // already as far as the schedule asks
		}
		// Less than a unit's worth waits: the gap grows until the next step covers it
		units := unitsToward(pool.unitReserve, soFar, target)
		if units == 0 {
			continue
		}

		before := pool.buyPrice(1)
		if before == 0 {
			continue
		}
		next := *pool // dry run on a copy, so the step that would cross the cap never happens
		if trade(&next, units) == 0 {
			continue
		}
		// A pool left with a unit or less has no buy price; skip rather than zero the running move
		after := next.buyPrice(1)
		if after == 0 {
			continue
		}
		// Rounded in the direction the price moves, so rounding drift never carries an event past its cap
		moved := e.movedBasisPoints * after / before
		if after > before {
			moved = ceilDiv(e.movedBasisPoints*after, before)
		}
		// A flood only moves the price down and a shortage only up, so each meets just its own side
		if moved < wholeBasisPoints-e.capBasisPoints || moved > wholeBasisPoints+e.capBasisPoints {
			continue
		}

		total := trade(pool, units)
		e.movedBasisPoints = moved
		w.logPoolTrade(id, e, intent, units, total, pool)
	}
}
