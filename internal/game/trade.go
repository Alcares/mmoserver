package game

import pb "github.com/alcares/mmoserver/gen/go/game/v1"

// TradeOrder created out of proto TradeRequest
type TradeOrder struct {
	PlayerID    uint32
	SequenceID  uint32
	Intent      pb.OrderIntent
	CashAmount  uint64
	BasisPoints uint32
}
