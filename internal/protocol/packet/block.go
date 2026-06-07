package packet

import "github.com/Relixik/gomc/internal/protocol/codec"

// Player Action statuses (the dig lifecycle).
const (
	DigStart  = 0 // started digging (an instant break in creative)
	DigFinish = 2 // finished digging (a survival break)
)

// PlayerAction is sent for digging and a few inventory actions. We use it for
// block breaking: Status drives the dig, Location is the targeted block, and
// Sequence must be echoed in a Block Changed Ack so the client keeps its
// prediction. (Play, sb, 0x29.)
type PlayerAction struct {
	Status   int32
	X, Y, Z  int32
	Face     byte
	Sequence int32
}

func (p *PlayerAction) Decode(r *codec.Reader) {
	p.Status = r.VarInt()
	p.X, p.Y, p.Z = r.Position()
	p.Face = r.UByte()
	p.Sequence = r.VarInt()
}

// BlockUpdate sets a single block to a new state for the client. (Play, cb,
// 0x08.)
type BlockUpdate struct {
	X, Y, Z    int32
	BlockState int32
}

func (p *BlockUpdate) ID() int32 { return idPlayBlockUpdate }

func (p *BlockUpdate) Encode(w *codec.Writer) {
	w.Position(p.X, p.Y, p.Z)
	w.VarInt(p.BlockState)
}

// BlockChangedAck confirms the server processed the client's block action up to
// Sequence, so the client commits (rather than rolls back) its predicted change.
// (Play, cb, 0x04.)
type BlockChangedAck struct {
	Sequence int32
}

func (p *BlockChangedAck) ID() int32 { return idPlayBlockChangedAck }

func (p *BlockChangedAck) Encode(w *codec.Writer) {
	w.VarInt(p.Sequence)
}
