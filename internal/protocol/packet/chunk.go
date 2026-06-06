package packet

import "github.com/Relixik/gomc/internal/protocol/codec"

// overworldSections is the number of 16-block-tall sections in the overworld
// (height 384, min Y -64). The client expects exactly this many sections in the
// Chunk Data "Data" field, derived from the dimension_type it has from the
// shared core pack.
const overworldSections = 24

// PlainsBiome is the global id of minecraft:plains in our synced biome registry
// (its position in the captured vanilla biome order).
const PlainsBiome int32 = 40

// ChunkData is the Chunk Data and Update Light packet for an empty (void)
// column: every section is single-valued air. (Play, cb, 0x2D.)
//
// The single-valued paletted-container format is [bits-per-entry=0][VarInt
// value] with NO data-array length (the long count has been calculated, not
// sent, since 1.21.5 — confirmed against minecraft.wiki and the captured
// vanilla air sections). Heightmaps and light are sent empty for a void column.
type ChunkData struct {
	X, Z    int32
	BiomeID int32 // global biome id applied to every section (cosmetic for void)
}

func (p *ChunkData) ID() int32 { return idPlayChunkData }
func (p *ChunkData) Encode(w *codec.Writer) {
	w.Int(p.X)
	w.Int(p.Z)

	// Heightmaps: empty prefixed array.
	w.VarInt(0)

	// Data: all-air sections (single-valued air block states + single biome).
	data := codec.NewWriter()
	for s := 0; s < overworldSections; s++ {
		// Per-section header: 4 bytes for an all-air section (the non-air block
		// count Short plus 2 bytes the client also consumes), all zero — matches
		// captured vanilla void sections byte-for-byte.
		data.Int(0)
		// Block states: single-valued air (bits-per-entry 0, value 0). The data
		// array length is NOT sent (calculated since 1.21.5).
		data.UByte(0)
		data.VarInt(0)
		// Biomes: single-valued.
		data.UByte(0)
		data.VarInt(p.BiomeID)
	}
	w.VarInt(int32(data.Len()))
	w.Raw(data.Bytes())

	// Block entities: none.
	w.VarInt(0)

	// Light: empty masks, no light data arrays.
	w.BitSet(nil) // sky light mask
	w.BitSet(nil) // block light mask
	w.BitSet(nil) // empty sky light mask
	w.BitSet(nil) // empty block light mask
	w.VarInt(0)   // sky light arrays
	w.VarInt(0)   // block light arrays
}
