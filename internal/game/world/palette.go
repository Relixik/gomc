package world

import "github.com/Relixik/gomc/internal/protocol/codec"

// paletteKind parameters bound a paletted container's bits-per-entry: below
// minBits the value is bumped (the protocol's minimum indirect width), above
// maxBits the direct (paletteless, global-id) format is used with directBits.
type paletteKind struct {
	minBits, maxBits, directBits int
}

// Block-state and biome containers have different width thresholds (verified
// against vanilla: a 4-state section serialises at 4 bpe, a single-state section
// at 0 bpe / single value).
var (
	blockPalette = paletteKind{minBits: 4, maxBits: 8, directBits: 15}
	biomePalette = paletteKind{minBits: 1, maxBits: 3, directBits: 7}
)

// writePalettedContainer encodes one paletted container (block states for 4096
// cells, or biomes for 64) to the wire as [bits-per-entry][palette][packed
// longs]. The long-array length is NOT prefixed — the client computes it from
// the bits-per-entry and the cell count, which is how vanilla 26.1.2 serialises
// it (confirmed byte-for-byte against a captured superflat chunk).
//
// The palette is built in first-seen scan order, which matches vanilla (it adds
// states bottom-up as the section is filled), so a single-block section yields a
// single-valued container and a 4-block superflat floor yields a 4-entry palette
// at 4 bpe in the same order vanilla emits.
func writePalettedContainer(w *codec.Writer, values []uint32, kind paletteKind) {
	palette := make([]uint32, 0, 16)
	index := make(map[uint32]int)
	for _, v := range values {
		if _, ok := index[v]; !ok {
			index[v] = len(palette)
			palette = append(palette, v)
		}
	}

	if len(palette) == 1 {
		// Single-valued: bits-per-entry 0, the value, and no data array.
		w.UByte(0)
		w.VarInt(int32(palette[0]))
		return
	}

	bits := bitsFor(len(palette))
	if bits <= kind.maxBits {
		if bits < kind.minBits {
			bits = kind.minBits
		}
		w.UByte(byte(bits))
		w.VarInt(int32(len(palette)))
		for _, p := range palette {
			w.VarInt(int32(p))
		}
		writeLongs(w, values, bits, func(v uint32) uint32 { return uint32(index[v]) })
		return
	}

	// Direct: indices are the global ids themselves, no palette section.
	w.UByte(byte(kind.directBits))
	writeLongs(w, values, kind.directBits, func(v uint32) uint32 { return v })
}

// writeLongs packs one entry per cell into 64-bit longs at `bits` bits each,
// least-significant entry first, with floor(64/bits) entries per long and no
// entry straddling a long boundary (the unused high bits stay zero). map2idx
// converts a cell value to the bits actually stored (palette index or global id).
func writeLongs(w *codec.Writer, values []uint32, bits int, map2idx func(uint32) uint32) {
	per := 64 / bits
	mask := uint64(1)<<uint(bits) - 1
	var cur uint64
	n := 0
	for _, v := range values {
		cur |= (uint64(map2idx(v)) & mask) << uint(n*bits)
		n++
		if n == per {
			w.Long(int64(cur))
			cur, n = 0, 0
		}
	}
	if n > 0 {
		w.Long(int64(cur))
	}
}

// bitsFor returns the smallest number of bits that can index a palette of size
// n (n >= 2): ceil(log2(n)).
func bitsFor(n int) int {
	bits := 0
	for (1 << bits) < n {
		bits++
	}
	return bits
}
