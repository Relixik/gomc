package packet

import (
	"testing"

	"github.com/Relixik/gomc/internal/protocol/codec"
)

func TestPlayerActionDecode(t *testing.T) {
	w := codec.NewWriter()
	w.VarInt(DigStart)
	w.Position(10, -61, -5)
	w.UByte(1) // face
	w.VarInt(7)

	r := codec.NewReader(w.Bytes())
	var p PlayerAction
	p.Decode(r)
	if r.Err() != nil {
		t.Fatalf("decode: %v", r.Err())
	}
	if p.Status != DigStart {
		t.Errorf("status = %d, want %d", p.Status, DigStart)
	}
	if p.X != 10 || p.Y != -61 || p.Z != -5 {
		t.Errorf("pos = %d,%d,%d", p.X, p.Y, p.Z)
	}
	if p.Face != 1 || p.Sequence != 7 {
		t.Errorf("face=%d seq=%d", p.Face, p.Sequence)
	}
	if _, ok := NewServerbound(StatePlay, idPlayPlayerAction); !ok {
		t.Error("player action not registered")
	}
}

func TestBlockUpdateAndAckEncode(t *testing.T) {
	bu := &BlockUpdate{X: 10, Y: -61, Z: -5, BlockState: 0}
	if bu.ID() != 0x08 {
		t.Errorf("block update id = %#x, want 0x08", bu.ID())
	}
	w := codec.NewWriter()
	bu.Encode(w)
	r := codec.NewReader(w.Bytes())
	if x, y, z := r.Position(); x != 10 || y != -61 || z != -5 {
		t.Errorf("pos = %d,%d,%d", x, y, z)
	}
	if bs := r.VarInt(); bs != 0 {
		t.Errorf("block state = %d, want 0 (air)", bs)
	}

	ack := &BlockChangedAck{Sequence: 7}
	if ack.ID() != 0x04 {
		t.Errorf("ack id = %#x, want 0x04", ack.ID())
	}
	w = codec.NewWriter()
	ack.Encode(w)
	if seq := codec.NewReader(w.Bytes()).VarInt(); seq != 7 {
		t.Errorf("ack sequence = %d, want 7", seq)
	}
}
