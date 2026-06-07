package packet

import (
	"testing"

	"github.com/Relixik/gomc/internal/protocol/codec"
)

func TestSetCarriedItemDecode(t *testing.T) {
	w := codec.NewWriter()
	w.Short(3)
	var p SetCarriedItem
	p.Decode(codec.NewReader(w.Bytes()))
	if p.Slot != 3 {
		t.Errorf("slot = %d, want 3", p.Slot)
	}
}

func TestSetCreativeModeSlotDecode(t *testing.T) {
	// Slot 36 (first hotbar), count 1, item id 1 (stone), then empty components.
	w := codec.NewWriter()
	w.Short(36)
	w.VarInt(1) // count
	w.VarInt(1) // item id (stone)
	w.VarInt(0) // components to add
	w.VarInt(0) // components to remove
	var p SetCreativeModeSlot
	p.Decode(codec.NewReader(w.Bytes()))
	if p.Slot != 36 || !p.HasItem || p.ItemID != 1 {
		t.Errorf("got slot=%d hasItem=%v id=%d; want 36,true,1", p.Slot, p.HasItem, p.ItemID)
	}

	// An empty slot is just count 0.
	w = codec.NewWriter()
	w.Short(40)
	w.VarInt(0)
	p = SetCreativeModeSlot{}
	p.Decode(codec.NewReader(w.Bytes()))
	if p.HasItem {
		t.Error("empty slot should decode HasItem=false")
	}
}

func TestUseItemOnDecode(t *testing.T) {
	// Include a world-border-hit flag before the sequence to prove the trailing
	// VarInt is recovered regardless of how many bool flags precede it.
	w := codec.NewWriter()
	w.VarInt(0)             // hand: main
	w.Position(10, -60, 20) // clicked block
	w.VarInt(1)             // face: up
	w.Float(0.5)            // cursor x
	w.Float(1.0)            // cursor y
	w.Float(0.5)            // cursor z
	w.Bool(false)           // inside block
	w.Bool(false)           // world border hit (newer protocols)
	w.VarInt(200)           // sequence
	var p UseItemOn
	p.Decode(codec.NewReader(w.Bytes()))
	if p.X != 10 || p.Y != -60 || p.Z != 20 {
		t.Errorf("pos = %d,%d,%d; want 10,-60,20", p.X, p.Y, p.Z)
	}
	if p.Face != 1 {
		t.Errorf("face = %d, want 1", p.Face)
	}
	if p.Sequence != 200 {
		t.Errorf("sequence = %d, want 200", p.Sequence)
	}
}

func TestTrailingVarInt(t *testing.T) {
	cases := []struct {
		bytes []byte
		want  int32
	}{
		{[]byte{0x05}, 5},                     // bare single byte
		{[]byte{0x00, 0x05}, 5},               // one leading bool
		{[]byte{0x00, 0x00, 0xC8, 0x01}, 200}, // two bools + 2-byte varint
	}
	for _, c := range cases {
		if got := trailingVarInt(c.bytes); got != c.want {
			t.Errorf("trailingVarInt(% x) = %d, want %d", c.bytes, got, c.want)
		}
	}
}
