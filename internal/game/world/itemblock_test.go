package world

import "testing"

// TestBlockStateForItem checks the embedded item->block-state mapping resolves
// the common building blocks and rejects non-block ids.
func TestBlockStateForItem(t *testing.T) {
	if len(itemToBlock) < 900 {
		t.Fatalf("item->block map has %d entries; embed/parse looks broken", len(itemToBlock))
	}
	cases := []struct {
		item  int32
		state uint32
	}{
		{1, Stone},       // stone item -> stone block
		{28, Dirt},       // dirt
		{27, GrassBlock}, // grass_block
	}
	for _, c := range cases {
		got, ok := BlockStateForItem(c.item)
		if !ok || got != c.state {
			t.Errorf("BlockStateForItem(%d) = %d,%v; want %d,true", c.item, got, ok, c.state)
		}
	}
	// An empty hand / unknown item does not place a block.
	if _, ok := BlockStateForItem(-1); ok {
		t.Error("item -1 should not map to a block")
	}
}

// TestBlockStateForItemAxis checks pillar-like blocks orient along the placement
// axis while plain blocks ignore it, and that the Y variant equals the default.
func TestBlockStateForItemAxis(t *testing.T) {
	const oakLog = 134 // x=136, y=137, z=138 in 26.1.2
	x, okX := BlockStateForItemAxis(oakLog, AxisX)
	y, okY := BlockStateForItemAxis(oakLog, AxisY)
	z, okZ := BlockStateForItemAxis(oakLog, AxisZ)
	if !okX || !okY || !okZ || x == y || y == z || x == z {
		t.Fatalf("oak_log axis states not distinct: x=%d,%v y=%d,%v z=%d,%v", x, okX, y, okY, z, okZ)
	}
	if def, _ := BlockStateForItem(oakLog); def != y {
		t.Errorf("oak_log default state %d != Y-axis state %d", def, y)
	}

	// A plain block ignores the axis and always yields its single state.
	for _, axis := range []int{AxisX, AxisY, AxisZ} {
		if got, ok := BlockStateForItemAxis(1, axis); !ok || got != Stone {
			t.Errorf("stone on axis %d = %d,%v; want %d,true", axis, got, ok, Stone)
		}
	}
}
