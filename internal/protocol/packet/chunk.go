package packet

import "github.com/Relixik/gomc/internal/protocol/codec"

// ChunkData is the "Chunk Data and Update Light" packet (Play, cb, 0x2D). The
// chunk's content — heightmaps, paletted section data, block entities, and light
// — is built by the game/world package and carried here as a pre-encoded body
// (everything after the X,Z coordinates), so the protocol layer stays free of
// world logic.
type ChunkData struct {
	X, Z    int32
	Payload []byte // body after X,Z: heightmaps, section data, block entities, light
}

func (p *ChunkData) ID() int32 { return idPlayChunkData }

func (p *ChunkData) Encode(w *codec.Writer) {
	w.Int(p.X)
	w.Int(p.Z)
	w.Raw(p.Payload)
}

// UnloadChunk tells the client to drop a chunk column that left the view
// distance. The field order is Z THEN X — a long-standing protocol quirk,
// confirmed against a 26.1.2 capture. (Play, cb, 0x25.)
type UnloadChunk struct{ X, Z int32 }

func (p *UnloadChunk) ID() int32 { return idPlayUnloadChunk }

func (p *UnloadChunk) Encode(w *codec.Writer) {
	w.Int(p.Z)
	w.Int(p.X)
}
