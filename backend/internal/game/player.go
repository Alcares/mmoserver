package game

import (
	"fmt"

	pb "github.com/alcares/mmoserver/backend/gen/go/game/v1"
)

const (
	StartingBalance = 1000_00 // cents, i.e. $1000.00
)

type Player struct {
	ID        uint32
	Name      string
	IsBot     bool
	Pos       Vec2f
	TargetDir Vec2f // Current intended movement heading (-1 to 1)
	Speed     float64
	// private
	balance     uint64
	commodities map[pb.CommodityType]uint64 // whole units
	tradeVolume uint64
	unitsTraded uint64
}

func NewPlayer(id uint32, pos Vec2f) *Player {
	commodityTypes := GetCommodityTypes()

	ownedCommodities := make(map[pb.CommodityType]uint64, len(commodityTypes))
	for _, v := range commodityTypes {
		ownedCommodities[v] = 0
	}

	return &Player{
		ID:          id,
		Name:        fmt.Sprintf("Player %d", id),
		Pos:         pos,
		Speed:       MoveSpeed,
		balance:     StartingBalance,
		commodities: ownedCommodities,
	}
}

// ToProtoState maps internal domain state to wire DTO
func (p *Player) ToProtoState() *pb.PlayerState {
	return &pb.PlayerState{
		Id:    p.ID,
		Name:  p.Name,
		X:     float32(p.Pos.X),
		Y:     float32(p.Pos.Y),
		IsBot: p.IsBot,
	}
}

func (p *Player) ToProtoInventory() *pb.PlayerInventory {
	commodities := make([]*pb.OwnedCommodity, 0, len(p.commodities))

	for cType, amount := range p.commodities {
		commodities = append(commodities, &pb.OwnedCommodity{
			Type:   cType,
			Amount: amount,
		})
	}

	return &pb.PlayerInventory{
		Balance:       p.balance,
		ActiveEffects: make([]*pb.ActiveEffect, 0),
		Commodities:   commodities,
	}
}

// PlayerMovementInput bundles the movement command with who sent it
type PlayerMovementInput struct {
	PlayerID uint32
	Vx       float64
	Vy       float64
}
