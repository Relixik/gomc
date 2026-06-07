package packet

import (
	"testing"

	"github.com/Relixik/gomc/internal/protocol/codec"
)

func TestChunkDataEncode(t *testing.T) {
	p := &ChunkData{X: -3, Z: 5, Payload: []byte{0xAB, 0xCD}}
	if p.ID() != 0x2D {
		t.Errorf("id = %#x, want 0x2D", p.ID())
	}
	w := codec.NewWriter()
	p.Encode(w)
	r := codec.NewReader(w.Bytes())
	if x := r.Int(); x != -3 {
		t.Errorf("X = %d, want -3", x)
	}
	if z := r.Int(); z != 5 {
		t.Errorf("Z = %d, want 5", z)
	}
	if rest := r.RemainingBytes(); len(rest) != 2 || rest[0] != 0xAB || rest[1] != 0xCD {
		t.Errorf("payload = % x, want ab cd", rest)
	}
}

// TestUnloadChunkEncodesZThenX pins the inverted field order (Z before X) — a
// protocol quirk verified against a 26.1.2 capture.
func TestUnloadChunkEncodesZThenX(t *testing.T) {
	p := &UnloadChunk{X: 3, Z: 7}
	if p.ID() != 0x25 {
		t.Errorf("id = %#x, want 0x25", p.ID())
	}
	w := codec.NewWriter()
	p.Encode(w)
	r := codec.NewReader(w.Bytes())
	if z := r.Int(); z != 7 {
		t.Errorf("first int = %d, want Z=7", z)
	}
	if x := r.Int(); x != 3 {
		t.Errorf("second int = %d, want X=3", x)
	}
}
