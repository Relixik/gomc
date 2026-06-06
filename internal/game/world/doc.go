// Package world holds the authoritative world model: Dimension, Chunk, the
// paletted containers for block states and biomes, heightmaps, and the
// serialization used by the Chunk Data + Update Light packet.
//
// Paletted containers bit-pack indices into []int64 with NO entry spanning a
// long boundary (each long is padded). Three palette kinds: single-valued
// (bits-per-entry 0, empty data array), indirect (4-8 bpe for blocks, 1-3 for
// biomes, with a local palette), and direct (global registry IDs). Heightmaps
// are long-arrays at bits-per-entry = ceil(log2(height+1)). All pure manual bit
// math.
//
// All world mutation happens on the single tick goroutine (internal/game/loop),
// so this package needs no mutexes on world state.
//
// Stdlib only.
package world
