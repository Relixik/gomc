package world

import "github.com/Relixik/gomc/internal/protocol/codec"

const (
	// SectionCount is the number of 16-block-tall sections in the overworld
	// (height 384, min build Y -64).
	SectionCount = 24

	sectionBlocks = 16 * 16 * 16 // 4096 cells per section
	sectionBiomes = 4 * 4 * 4    // 64 biome cells per section
	columnCells   = 16 * 16      // 256 columns in a chunk (for heightmaps)

	// surfaceHeight is the heightmap value for the flat surface: the grass top
	// sits at Y -61, so 1 + (-61 - -64) = 4 (verified against the vanilla
	// superflat capture).
	surfaceHeight = 4
	heightmapBits = 9 // ceil(log2(world height 384 + 1))
)

// superflatPayload is the ChunkData body (after the X,Z ints) shared by every
// column of a superflat world; built once at package init.
var superflatPayload = buildSuperflatPayload()

// SuperflatPayload returns the Chunk Data (and Update Light) packet body that
// follows the X,Z chunk coordinates: heightmaps, the 24 sections, the empty
// block-entity list, and full-bright sky light. The returned slice is shared and
// must not be mutated.
func SuperflatPayload() []byte { return superflatPayload }

// fullBrightLight is one section's sky-light array: 4096 cells × 4 bits = 2048
// bytes, every nibble 15 (full daylight).
var fullBrightLight = func() []byte {
	b := make([]byte, 2048)
	for i := range b {
		b[i] = 0xFF
	}
	return b
}()

func buildSuperflatPayload() []byte {
	w := codec.NewWriter()

	// Heightmaps: WORLD_SURFACE (1), MOTION_BLOCKING_NO_LEAVES (5), and
	// MOTION_BLOCKING (4), all flat at surfaceHeight — the three vanilla sends.
	// Each is a length-prefixed long array (unlike the section containers).
	column := make([]uint32, columnCells)
	for i := range column {
		column[i] = surfaceHeight
	}
	w.VarInt(3)
	for _, kind := range []int32{1, 5, 4} {
		w.VarInt(kind)
		hw := codec.NewWriter()
		writeLongs(hw, column, heightmapBits, func(v uint32) uint32 { return v })
		w.VarInt(int32(hw.Len() / 8))
		w.Raw(hw.Bytes())
	}

	// Section data (length-prefixed).
	sw := codec.NewWriter()
	for s := 0; s < SectionCount; s++ {
		writeSection(sw, s)
	}
	w.VarInt(int32(sw.Len()))
	w.Raw(sw.Bytes())

	// Block entities: none.
	w.VarInt(0)

	// Light. Mirror vanilla's superflat masks — explicit sky light only for the
	// ground section and the air section just above it (light-sections 1 and 2);
	// the higher air sections default to full sky client-side, and the filler
	// section below the world (light-section 0) is empty/dark. The two arrays we
	// do send are full-bright so nothing renders dark; exact vanilla light values
	// are a later refinement.
	w.BitSet([]uint64{0b110}) // sky-light mask: light-sections 1, 2
	w.BitSet(nil)             // block-light mask: none
	w.BitSet([]uint64{0b001}) // empty sky-light mask: light-section 0
	w.BitSet([]uint64{0b111}) // empty block-light mask: light-sections 0, 1, 2
	w.VarInt(2)               // two sky-light arrays follow
	w.ByteArray(fullBrightLight)
	w.ByteArray(fullBrightLight)
	w.VarInt(0) // no block-light arrays

	return w.Bytes()
}

// writeSection encodes one chunk section: the non-air block count, a 2-byte
// field that is always zero (verified against vanilla — it is NOT part of a
// 4-byte count), the block-state container, and the biome container.
func writeSection(w *codec.Writer, sectionIdx int) {
	blocks := superflatBlocks(sectionIdx)
	nonAir := 0
	for _, b := range blocks {
		if b != Air {
			nonAir++
		}
	}
	w.Short(int16(nonAir))
	w.Short(0)
	writePalettedContainer(w, blocks, blockPalette)

	biomes := make([]uint32, sectionBiomes)
	for i := range biomes {
		biomes[i] = PlainsBiome
	}
	writePalettedContainer(w, biomes, biomePalette)
}

// superflatBlocks returns the 4096 block-state ids for one section of the
// classic flat preset: bedrock / dirt / dirt / grass at the very bottom of the
// world, air above. Cell index is y*256 + z*16 + x, so the first 256 cells are
// the lowest layer.
func superflatBlocks(sectionIdx int) []uint32 {
	blocks := make([]uint32, sectionBlocks) // zero value == Air
	if sectionIdx != 0 {
		return blocks // every section above the floor is pure air
	}
	for i := range blocks {
		switch i / columnCells { // relative Y within the section
		case 0:
			blocks[i] = Bedrock // Y -64
		case 1, 2:
			blocks[i] = Dirt // Y -63, -62
		case 3:
			blocks[i] = GrassBlock // Y -61
		default:
			blocks[i] = Air
		}
	}
	return blocks
}
