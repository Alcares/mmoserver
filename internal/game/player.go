package game

import pb "github.com/alcares/mmoserver/gen/go/game/v1"

type Player struct {
	ID        uint32
	Name      string
	Pos       Vec2f
	TargetDir Vec2f // Current intended movement heading (-1 to 1)
	Speed     float64
	// private
	balance     uint64
	commodities []OwnedCommodity
}

type OwnedCommodity struct {
	cType  pb.CommodityType
	amount uint32
}

// ToProtoState maps internal domain state to wire DTO
func (p *Player) ToProtoState() *pb.PlayerState {
	return &pb.PlayerState{
		Id:   p.ID,
		Name: p.Name,
		X:    float32(p.Pos.X),
		Y:    float32(p.Pos.Y),
	}
}

func (p *Player) ToProtoInventory() *pb.PlayerInventory {
	return &pb.PlayerInventory{
		Balance:       p.balance,
		ActiveEffects: make([]*pb.ActiveEffect, 0),
		Commodities:   make([]*pb.OwnedCommodity, 0),
	}
}

// PlayerMovementInput bundles the movement command with who sent it
type PlayerMovementInput struct {
	PlayerID uint32
	Vx       float64
	Vy       float64
}
