package packet

import "github.com/Relixik/gomc/internal/protocol/codec"

// SetCarriedItem selects one of the nine hotbar slots (0..8). (Play, sb, 0x35.)
type SetCarriedItem struct {
	Slot int16
}

func (p *SetCarriedItem) Decode(r *codec.Reader) { p.Slot = r.Short() }

// SetCreativeModeSlot sets an inventory slot to an item stack, sent in creative
// mode when the player picks from the creative menu or rearranges the inventory.
// We decode only the slot index and the item id (the leading Count + Item ID of
// the Slot); the trailing item components are left unread — the framer consumes
// the whole packet regardless, and we only need the id to know what block is in
// hand. (Play, sb, 0x38.)
type SetCreativeModeSlot struct {
	Slot    int16
	ItemID  int32
	HasItem bool
}

func (p *SetCreativeModeSlot) Decode(r *codec.Reader) {
	p.Slot = r.Short()
	if count := r.VarInt(); count > 0 {
		p.ItemID = r.VarInt()
		p.HasItem = true
	}
}

// UseItemOn is sent when the player right-clicks a block face — the place / use
// action. We need the clicked Location and Face to compute the placement cell,
// and the Sequence to acknowledge the client's prediction. The fields between
// Face and Sequence (cursor floats, an inside-block flag, and newer trailing
// flags such as world-border-hit) vary by protocol; since Sequence is the final
// VarInt, we read Hand/Location/Face — stable across versions — then pull the
// trailing VarInt from whatever remains, which is robust to the exact tail
// layout. (Play, sb, 0x42.)
type UseItemOn struct {
	Hand     int32
	X, Y, Z  int32
	Face     int32
	Sequence int32
}

func (p *UseItemOn) Decode(r *codec.Reader) {
	p.Hand = r.VarInt()
	p.X, p.Y, p.Z = r.Position()
	p.Face = r.VarInt()
	p.Sequence = trailingVarInt(r.RemainingBytes())
}

// trailingVarInt extracts the VarInt that ends b (a packet's final field),
// skipping any preceding fixed bytes. A VarInt's non-final bytes have the high
// bit set, so the value starts at the byte (scanning back from the end) whose
// predecessor has the high bit clear — a boundary that the leading bool flags
// (always 0x00/0x01) provide.
func trailingVarInt(b []byte) int32 {
	if len(b) == 0 {
		return 0
	}
	i := len(b) - 1
	for i > 0 && b[i-1]&0x80 != 0 {
		i--
	}
	var result uint32
	var shift uint
	for ; i < len(b); i++ {
		result |= uint32(b[i]&0x7F) << shift
		shift += 7
	}
	return int32(result)
}
