package loop

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Relixik/gomc/internal/protocol/codec"
)

func quietHub() *Hub {
	return New(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// recvID waits for one packet on out and returns its packet id.
func recvID(t *testing.T, out <-chan []byte) int32 {
	t.Helper()
	select {
	case body := <-out:
		return codec.NewReader(body).VarInt()
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a broadcast packet")
		return -1
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
	// P2 learns about P1 (spawn) and gets the full list (info).
	if id := recvID(t, out2); id != 0x01 {
		t.Errorf("P2<-P1 spawn id = %#x, want 0x01", id)
	}
	if id := recvID(t, out2); id != 0x46 {
		t.Errorf("P2 list info id = %#x, want 0x46", id)
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
