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
