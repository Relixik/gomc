package world

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/Relixik/gomc/internal/protocol/codec"
)

func sectionBytes(idx int) []byte {
	w := codec.NewWriter()
	writeSection(w, idx, nil)
	return w.Bytes()
}

// TestAirSectionMatchesVanilla pins an all-air section to the exact 8 bytes a
// vanilla 26.1.2 server emits (count 0, single-air block states, single-plains
// biome) — the same bytes seen in the void capture.
func TestAirSectionMatchesVanilla(t *testing.T) {
	got := sectionBytes(1)
	want := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x28}
	if !bytes.Equal(got, want) {
		t.Errorf("air section = % x, want % x", got, want)
	}
}

// TestFloorSectionMatchesVanilla pins the bottom (floor) section against the
// captured superflat chunk: Short(1024) count, Short(0), a 4-bpe block palette
// [bedrock,dirt,grass,air], 256 longs, and a single-plains biome tail.
func TestFloorSectionMatchesVanilla(t *testing.T) {
	got := sectionBytes(0)
	if len(got) != 2060 {
		t.Fatalf("floor section len = %d, want 2060", len(got))
	}
	// count=1024 (04 00), reserved 00 00, bpe=4, palette len=4, [85,10,9,0].
	wantPrefix := []byte{0x04, 0x00, 0x00, 0x00, 0x04, 0x04, 0x55, 0x0a, 0x09, 0x00}
	if !bytes.Equal(got[:10], wantPrefix) {
		t.Errorf("floor prefix = % x, want % x", got[:10], wantPrefix)
	}
	if tail := got[len(got)-2:]; !bytes.Equal(tail, []byte{0x00, 0x28}) {
		t.Errorf("biome tail = % x, want 00 28 (single plains)", tail)
	}
	// The all-air region (palette index 3) packs to 0x3 per 4-bit entry; the
	// longs start at offset 10, so long #64 (entries 1024..1039) is all-air.
	airLong := got[10+64*8 : 10+65*8]
	wantLong := []byte{0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33}
	if !bytes.Equal(airLong, wantLong) {
		t.Errorf("air-region long = % x, want all 0x33", airLong)
	}
	// The bottom layer is all bedrock (palette index 0) -> long #0 is zero.
	if firstLong := got[10:18]; !bytes.Equal(firstLong, make([]byte, 8)) {
		t.Errorf("bedrock long = % x, want zero", firstLong)
	}
}

func TestPaletteSingleValued(t *testing.T) {
	w := codec.NewWriter()
	vals := make([]uint32, sectionBiomes)
	for i := range vals {
		vals[i] = PlainsBiome
	}
	writePalettedContainer(w, vals, biomePalette)
	if want := []byte{0x00, 0x28}; !bytes.Equal(w.Bytes(), want) { // bpe 0, value 40
		t.Errorf("single-valued = % x, want % x", w.Bytes(), want)
	}
}

func TestPaletteBumpsToMinBits(t *testing.T) {
	// A 4-entry block palette needs 2 bits but is bumped to the 4-bit indirect
	// minimum; 4096 entries at 4 bpe = 256 longs.
	w := codec.NewWriter()
	blocks := superflatBlocks(0)
	writePalettedContainer(w, blocks, blockPalette)
	r := codec.NewReader(w.Bytes())
	if bpe := r.UByte(); bpe != 4 {
		t.Fatalf("bpe = %d, want 4", bpe)
	}
	if n := r.VarInt(); n != 4 {
		t.Fatalf("palette len = %d, want 4", n)
	}
	pal := []int32{r.VarInt(), r.VarInt(), r.VarInt(), r.VarInt()}
	want := []int32{85, 10, 9, 0}
	for i := range want {
		if pal[i] != want[i] {
			t.Errorf("palette[%d] = %d, want %d", i, pal[i], want[i])
		}
	}
	if rem := r.Remaining(); rem != 256*8 {
		t.Errorf("data array = %d bytes, want %d (256 longs)", rem, 256*8)
	}
}

func TestHeightmapPacking(t *testing.T) {
	w := codec.NewWriter()
	col := make([]uint32, columnCells)
	for i := range col {
		col[i] = surfaceHeight
	}
	writeLongs(w, col, heightmapBits, func(v uint32) uint32 { return v })
	if n := w.Len() / 8; n != 37 {
		t.Fatalf("heightmap longs = %d, want 37", n)
	}
	var want uint64
	for k := 0; k < 7; k++ { // seven 9-bit values per long
		want |= uint64(surfaceHeight) << uint(k*heightmapBits)
	}
	if got := binary.BigEndian.Uint64(w.Bytes()[:8]); got != want {
		t.Errorf("first heightmap long = %#x, want %#x", got, want)
	}
}

func TestWorldBlockOverride(t *testing.T) {
	w := NewWorld()
	if got := w.ChunkPayload(0, 0); !bytes.Equal(got, SuperflatPayload()) {
		t.Error("a pristine chunk should be the shared base payload")
	}

	// Break the grass block at (0,-61,0); the chunk payload must change.
	if !w.SetBlock(0, -61, 0, Air) {
		t.Fatal("SetBlock returned false for an in-range position")
	}
	if got := w.ChunkPayload(0, 0); bytes.Equal(got, SuperflatPayload()) {
		t.Error("a modified chunk should differ from the base payload")
	}
	// An untouched neighbour still shares the base.
	if got := w.ChunkPayload(1, 0); !bytes.Equal(got, SuperflatPayload()) {
		t.Error("an untouched chunk should still be the base payload")
	}
	// Out-of-range edits are rejected.
	if w.SetBlock(0, 1000, 0, Air) {
		t.Error("SetBlock above the world should fail")
	}
}

// TestSuperflatPayloadStructure walks the whole cached payload and asserts it
// decodes cleanly with no trailing bytes — catching any framing drift across
// heightmaps, section data, block entities, and light.
func TestSuperflatPayloadStructure(t *testing.T) {
	r := codec.NewReader(SuperflatPayload())

	if n := r.VarInt(); n != 3 {
		t.Fatalf("heightmap count = %d, want 3", n)
	}
	for i := 0; i < 3; i++ {
		r.VarInt()        // type
		n := r.VarInt()   // long count
		r.Raw(int(n) * 8) // longs
	}
	dataLen := r.VarInt()
	r.Raw(int(dataLen)) // section data
	if be := r.VarInt(); be != 0 {
		t.Errorf("block entities = %d, want 0", be)
	}
	r.BitSet() // sky mask
	r.BitSet() // block mask
	r.BitSet() // empty sky mask
	r.BitSet() // empty block mask
	nSky := r.VarInt()
	for i := int32(0); i < nSky; i++ {
		r.ByteArray()
	}
	nBlk := r.VarInt()
	for i := int32(0); i < nBlk; i++ {
		r.ByteArray()
	}
	if r.Err() != nil {
		t.Fatalf("decode payload: %v", r.Err())
	}
	if rem := r.Remaining(); rem != 0 {
		t.Errorf("trailing bytes after payload: %d", rem)
	}
}
