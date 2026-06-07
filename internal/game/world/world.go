package world

import "sync"

// minBuildY is the lowest buildable Y (the bottom of section 0).
const minBuildY = -64

// World holds the mutable block overrides layered on top of the base superflat
// terrain. A single RWMutex guards them: the game loop writes (block changes are
// rare), session goroutines read while encoding chunks. Positions outside the
// buildable height are ignored.
type World struct {
	mu        sync.RWMutex
	overrides map[[2]int32]map[int]uint32 // chunk -> (column-local index -> state id)
}

// NewWorld returns an empty superflat world (no modifications yet).
func NewWorld() *World {
	return &World{overrides: make(map[[2]int32]map[int]uint32)}
}

// ChunkCoord returns the chunk coordinate containing block coordinate c.
func ChunkCoord(c int32) int32 { return c >> 4 }

// blockIndex maps an absolute block position to its column-local index (matching
// buildPayload's ordering), or -1 if Y is out of the buildable range.
func blockIndex(x, y, z int32) int {
	if y < minBuildY || y >= minBuildY+SectionCount*16 {
		return -1
	}
	return int(y-minBuildY)*256 + int(z&15)*16 + int(x&15)
}

// SetBlock sets the block at absolute (x,y,z) to state, returning false if the
// position is outside the buildable height. Intended for the single writer (the
// game loop).
func (w *World) SetBlock(x, y, z int32, state uint32) bool {
	idx := blockIndex(x, y, z)
	if idx < 0 {
		return false
	}
	key := [2]int32{ChunkCoord(x), ChunkCoord(z)}
	w.mu.Lock()
	defer w.mu.Unlock()
	col := w.overrides[key]
	if col == nil {
		col = make(map[int]uint32)
		w.overrides[key] = col
	}
	col[idx] = state
	return true
}

// ChunkPayload returns the ChunkData body for chunk (cx,cz), applying any block
// overrides. Unmodified columns share the cached pristine payload.
func (w *World) ChunkPayload(cx, cz int32) []byte {
	w.mu.RLock()
	col := w.overrides[[2]int32{cx, cz}]
	if len(col) == 0 {
		w.mu.RUnlock()
		return superflatPayload
	}
	cp := make(map[int]uint32, len(col)) // copy so encoding runs lock-free
	for k, v := range col {
		cp[k] = v
	}
	w.mu.RUnlock()
	return buildPayload(cp)
}
