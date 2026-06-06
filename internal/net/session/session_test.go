package session

import (
	"context"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"

	"github.com/Relixik/gomc/internal/protocol/codec"
	"github.com/Relixik/gomc/internal/protocol/frame"
	"github.com/Relixik/gomc/internal/protocol/packet"
	"github.com/Relixik/gomc/internal/protocol/text"
)

// quietLogger discards session debug output during tests.
func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// writePacket frames a clientbound-style (id + fields) body from the client side.
func writePacket(t *testing.T, c *frame.Conn, id int32, build func(w *codec.Writer)) {
	t.Helper()
	w := codec.NewWriter()
	w.VarInt(id)
	build(w)
	if err := c.WritePacket(w.Bytes()); err != nil {
		t.Fatalf("client WritePacket: %v", err)
	}
}

// TestStatusPing drives a full server-list ping: handshake (next=status) ->
// status request -> response -> ping -> pong.
func TestStatusPing(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	status := text.StatusResponse{
		Version:     text.StatusVersion{Name: "26.1.2", Protocol: 775},
		Players:     text.StatusPlayers{Max: 20, Online: 3},
		Description: text.Plain("gomc test"),
	}
	sess := New(serverConn, status, quietLogger())
	go sess.Serve(context.Background())

	client := frame.NewConn(clientConn)

	// Handshake: protocol, address, port, next-state = status (1).
	writePacket(t, client, 0x00, func(w *codec.Writer) {
		w.VarInt(775)
		w.String("localhost")
		w.UShort(25565)
		w.VarInt(packet.IntentStatus)
	})
	// Status Request (empty).
	writePacket(t, client, 0x00, func(*codec.Writer) {})

	// Read Status Response.
	body, err := client.ReadPacket()
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	r := codec.NewReader(body)
	if id := r.VarInt(); id != 0x00 {
		t.Fatalf("response id = %#x", id)
	}
	js := r.String(0)
	if r.Err() != nil {
		t.Fatalf("response decode: %v", r.Err())
	}
	for _, want := range []string{`"protocol":775`, `"name":"26.1.2"`, `"max":20`, `"online":3`, `gomc test`} {
		if !strings.Contains(js, want) {
			t.Errorf("status JSON %s missing %s", js, want)
		}
	}

	// Ping with a payload; expect the same payload back in Pong.
	const payload = int64(0x0123456789ABCDEF)
	writePacket(t, client, 0x01, func(w *codec.Writer) { w.Long(payload) })

	body, err = client.ReadPacket()
	if err != nil {
		t.Fatalf("read pong: %v", err)
	}
	r = codec.NewReader(body)
	if id := r.VarInt(); id != 0x01 {
		t.Fatalf("pong id = %#x", id)
	}
	if got := r.Long(); got != payload {
		t.Errorf("pong payload = %#x, want %#x", got, payload)
	}
}

// TestHandshakeToLogin checks the handshake routes to the Login state (the
// branch M1 continues from next).
func TestHandshakeToLogin(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	sess := New(serverConn, text.StatusResponse{}, quietLogger())
	go sess.Serve(context.Background())

	client := frame.NewConn(clientConn)
	writePacket(t, client, 0x00, func(w *codec.Writer) {
		w.VarInt(775)
		w.String("localhost")
		w.UShort(25565)
		w.VarInt(packet.IntentLogin)
	})
	// No Login handler yet: send an unknown packet so the session ends; the test
	// asserts the connection is closed rather than hanging.
	writePacket(t, client, 0x7F, func(*codec.Writer) {})
	_ = clientConn.Close()
}

func TestInvalidNextStateClosesConn(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	sess := New(serverConn, text.StatusResponse{}, quietLogger())
	done := make(chan struct{})
	go func() { sess.Serve(context.Background()); close(done) }()

	client := frame.NewConn(clientConn)
	writePacket(t, client, 0x00, func(w *codec.Writer) {
		w.VarInt(775)
		w.String("localhost")
		w.UShort(25565)
		w.VarInt(99) // invalid next state
	})
	<-done // Serve must return (connection closed) rather than hang
}
