package world

// Block state ids in the 26.1.2 global palette, taken from the vanilla data
// generator's blocks.json (the default state of each block). These are the
// values that go into a chunk section's block-state paletted container.
const (
	Air        uint32 = 0
	Stone      uint32 = 1
	GrassBlock uint32 = 9 // snowy=false (the default grass_block state)
	Dirt       uint32 = 10
	Bedrock    uint32 = 85
)

// PlainsBiome is the network id of minecraft:plains — its index in the synced
// biome registry (captured vanilla order). Every superflat section uses it.
const PlainsBiome uint32 = 40

// MaxBlockStateID is the highest block-state id in 26.1.2 (from blocks.json).
// A direct (paletteless) block container therefore needs ceil(log2(29873)) = 15
// bits per entry.
const MaxBlockStateID = 29872
