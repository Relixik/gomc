package loop

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Relixik/gomc/internal/game/world"
	"github.com/Relixik/gomc/internal/protocol/codec"
)

func quietHub() *Hub {
	return New(world.NewWorld(), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// recvBody waits for one packet on out and returns its raw body.
func recvBody(t *testing.T, out <-chan []byte) []byte {
	t.Helper()
	select {
	case body := <-out:
		return body
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a broadcast packet")
		return nil
	}
}

// recvID waits for one packet and returns its packet id.
func recvID(t *testing.T, out <-chan []byte) int32 {
	t.Helper()
	return codec.NewReader(recvBody(t, out)).VarInt()
}

// drainN discards n packets (e.g. the join handshake) before the part under test.
func drainN(t *testing.T, out <-chan []byte, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		recvBody(t, out)
	}
}

// TestHubPresence checks the join/leave fan-out: a lone player gets its own list
// entry, a second join makes each see the other (info + spawn), and a leave tells
// the remaining player to despawn it.
func TestHubPresence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := quietHub()
	go h.Run(ctx)

	out1 := make(chan []byte, 64)
	eid1 := h.NextEntityID()
	h.Join(JoinRequest{EntityID: eid1, UUID: codec.UUID{1}, Name: "P1", Y: -60, Out: out1})
	// Alone: P1 receives a Player Info Update for itself (skin/tab list).
	if id := recvID(t, out1); id != 0x46 {
		t.Fatalf("P1 self info id = %#x, want 0x46", id)
	}

	out2 := make(chan []byte, 64)
	eid2 := h.NextEntityID()
	if eid2 == eid1 {
		t.Fatal("entity ids not unique")
	}
	h.Join(JoinRequest{EntityID: eid2, UUID: codec.UUID{2}, Name: "P2", X: 5, Y: -60, Z: 5, Out: out2})

	// P1 learns about P2: info then spawn.
	if id := recvID(t, out1); id != 0x46 {
		t.Errorf("P1<-P2 info id = %#x, want 0x46", id)
	}
	if id := recvID(t, out1); id != 0x01 {
		t.Errorf("P1<-P2 spawn id = %#x, want 0x01", id)
	}
	// P2 gets the full player list (info) first, THEN the entity to spawn — the
	// client needs the profile before the entity renders.
	if id := recvID(t, out2); id != 0x46 {
		t.Errorf("P2 list info id = %#x, want 0x46", id)
	}
	if id := recvID(t, out2); id != 0x01 {
		t.Errorf("P2<-P1 spawn id = %#x, want 0x01", id)
	}

	// P2 leaves: P1 is told to remove the entity and the list entry.
	h.Leave(eid2)
	if id := recvID(t, out1); id != 0x4D {
		t.Errorf("P1 remove-entity id = %#x, want 0x4D", id)
	}
	if id := recvID(t, out1); id != 0x45 {
		t.Errorf("P1 info-remove id = %#x, want 0x45", id)
	}
}

// TestHubMovement checks that a position move is broadcast as a Set Entity
// Position delta (newBlock-oldBlock)*4096, and a rotation-only move as Update
// Entity Rotation followed by Set Head Rotation.
func TestHubMovement(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := quietHub()
	go h.Run(ctx)

	out1 := make(chan []byte, 64)
	out2 := make(chan []byte, 64)
	eid1 := h.NextEntityID()
	h.Join(JoinRequest{EntityID: eid1, UUID: codec.UUID{1}, Name: "P1", Y: -60, Out: out1})
	drainN(t, out1, 1) // self info
	eid2 := h.NextEntityID()
	h.Join(JoinRequest{EntityID: eid2, UUID: codec.UUID{2}, Name: "P2", Y: -60, Out: out2})
	drainN(t, out1, 2) // P2 info + spawn
	drainN(t, out2, 2) // P1 spawn + list

	// P1 walks +1.5 blocks on X (no rotation): a move_entity_pos delta.
	h.Move(MoveRequest{EntityID: eid1, X: 1.5, Y: -60, Z: 0, OnGround: true})
	r := codec.NewReader(recvBody(t, out2))
	if id := r.VarInt(); id != 0x35 {
		t.Fatalf("move id = %#x, want 0x35 (pos)", id)
	}
	if eid := r.VarInt(); eid != eid1 {
		t.Errorf("move eid = %d, want %d", eid, eid1)
	}
	if dx := r.Short(); dx != 6144 { // 1.5 blocks * 4096
		t.Errorf("delta X = %d, want 6144", dx)
	}
	if dy, dz := r.Short(), r.Short(); dy != 0 || dz != 0 {
		t.Errorf("delta Y/Z = %d/%d, want 0", dy, dz)
	}
	if !r.Bool() {
		t.Error("onGround = false, want true")
	}

	// P1 only turns: an Update Entity Rotation then a Set Head Rotation.
	h.Move(MoveRequest{EntityID: eid1, X: 1.5, Y: -60, Z: 0, Yaw: 90, OnGround: true})
	if id := recvID(t, out2); id != 0x38 {
		t.Errorf("rotation id = %#x, want 0x38", id)
	}
	if id := recvID(t, out2); id != 0x53 {
		t.Errorf("head id = %#x, want 0x53", id)
	}
}

// TestHubChat checks chat is broadcast to everyone as a System Chat carrying
// "<name> message".
func TestHubChat(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := quietHub()
	go h.Run(ctx)

	out := make(chan []byte, 64)
	eid := h.NextEntityID()
	h.Join(JoinRequest{EntityID: eid, UUID: codec.UUID{1}, Name: "P1", Out: out})
	drainN(t, out, 1) // self info

	h.Chat(eid, "hello")
	body := recvBody(t, out)
	if id := codec.NewReader(body).VarInt(); id != 0x79 {
		t.Fatalf("chat id = %#x, want 0x79", id)
	}
	if !bytes.Contains(body, []byte("<P1> hello")) {
		t.Errorf("chat body missing formatted message: % x", body)
	}
}

// TestHubBreak checks a block break mutates the shared world and is broadcast as
// a Block Update.
func TestHubBreak(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := world.NewWorld()
	h := New(w, slog.New(slog.NewTextHandler(io.Discard, nil)))
	go h.Run(ctx)

	out := make(chan []byte, 64)
	eid := h.NextEntityID()
	h.Join(JoinRequest{EntityID: eid, UUID: codec.UUID{1}, Name: "P1", Out: out})
	drainN(t, out, 1) // self info

	h.Break(0, -61, 0)
	if id := recvID(t, out); id != 0x08 {
		t.Fatalf("break broadcast id = %#x, want 0x08", id)
	}
	if got := w.ChunkPayload(0, 0); bytes.Equal(got, world.SuperflatPayload()) {
		t.Error("world chunk should be modified after a break")
	}
}

// TestHubPlace checks a block place mutates the shared world and is broadcast as
// a Block Update carrying the placed state.
func TestHubPlace(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := world.NewWorld()
	h := New(w, slog.New(slog.NewTextHandler(io.Discard, nil)))
	go h.Run(ctx)

	out := make(chan []byte, 64)
	eid := h.NextEntityID()
	h.Join(JoinRequest{EntityID: eid, UUID: codec.UUID{1}, Name: "P1", Out: out})
	drainN(t, out, 1) // self info

	h.Place(0, -59, 0, world.Stone)
	r := codec.NewReader(recvBody(t, out))
	if id := r.VarInt(); id != 0x08 {
		t.Fatalf("place broadcast id = %#x, want 0x08", id)
	}
	r.Position() // x, y, z
	if st := r.VarInt(); st != int32(world.Stone) {
		t.Errorf("placed state = %d, want %d", st, world.Stone)
	}
	if got := w.ChunkPayload(0, 0); bytes.Equal(got, world.SuperflatPayload()) {
		t.Error("world chunk should be modified after a place")
	}
}
