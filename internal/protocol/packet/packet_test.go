package packet

import (
	"testing"

	"github.com/Relixik/gomc/internal/protocol/codec"
)

func TestHandshakeDecode(t *testing.T) {
	w := codec.NewWriter()
	w.VarInt(775)
	w.String("localhost")
	w.UShort(25565)
	w.VarInt(IntentLogin)

	r := codec.NewReader(w.Bytes())
	var h Handshake
	h.Decode(r)
	if r.Err() != nil {
		t.Fatalf("decode err: %v", r.Err())
	}
	if h.ProtocolVersion != 775 || h.ServerAddress != "localhost" ||
		h.ServerPort != 25565 || h.NextState != IntentLogin {
		t.Errorf("decoded %+v", h)
	}
}

func TestRegistryLookup(t *testing.T) {
	if d, ok := NewServerbound(StateHandshaking, idHandshake); !ok {
		t.Error("handshake not registered")
	} else if _, isHandshake := d.(*Handshake); !isHandshake {
		t.Errorf("got %T, want *Handshake", d)
	}
	if d, ok := NewServerbound(StateStatus, idStatusPing); !ok {
		t.Error("status ping not registered")
	} else if _, isPing := d.(*StatusPing); !isPing {
		t.Errorf("got %T, want *StatusPing", d)
	}
	if d, ok := NewServerbound(StatePlay, idPlayConfirmTeleport); !ok {
		t.Error("play confirm-teleport not registered")
	} else if _, isConfirm := d.(*ConfirmTeleport); !isConfirm {
		t.Errorf("got %T, want *ConfirmTeleport", d)
	}
	if _, ok := NewServerbound(StatePlay, 0x7E); ok {
		t.Error("expected no Play packet registered at 0x7E")
	}
}

func TestStatusClientboundEncode(t *testing.T) {
	resp := &StatusResponse{JSON: "x"}
	if resp.ID() != 0x00 {
		t.Errorf("response id = %#x", resp.ID())
	}
	w := codec.NewWriter()
	resp.Encode(w)
	r := codec.NewReader(w.Bytes())
	if got := r.String(0); got != "x" {
		t.Errorf("response payload = %q", got)
	}

	pong := &StatusPong{Payload: 0x1122334455667788}
	if pong.ID() != 0x01 {
		t.Errorf("pong id = %#x", pong.ID())
	}
	w = codec.NewWriter()
	pong.Encode(w)
	r = codec.NewReader(w.Bytes())
	if got := r.Long(); got != 0x1122334455667788 {
		t.Errorf("pong payload = %#x", got)
	}
}

func TestStatusPingDecode(t *testing.T) {
	w := codec.NewWriter()
	w.Long(42)
	r := codec.NewReader(w.Bytes())
	var p StatusPing
	p.Decode(r)
	if p.Payload != 42 || r.Err() != nil {
		t.Errorf("ping payload = %d err=%v", p.Payload, r.Err())
	}
}
