package session

import (
	"context"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"

	"github.com/Relixik/gomc/internal/protocol/auth"
	"github.com/Relixik/gomc/internal/protocol/codec"
	"github.com/Relixik/gomc/internal/protocol/frame"
	"github.com/Relixik/gomc/internal/protocol/packet"
	"github.com/Relixik/gomc/internal/protocol/text"
)

// offlineOpts disables online mode and compression: the minimal lifecycle used
// by the status/offline-login tests, which read clientbound packets directly.
var offlineOpts = Options{CompressionThreshold: frame.CompressionDisabled}

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
	sess := New(serverConn, status, offlineOpts, quietLogger())
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

// TestLoginOffline drives handshake (next=login) -> LoginStart -> LoginSuccess
// and asserts the offline UUID is derived from the name.
func TestLoginOffline(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	sess := New(serverConn, text.StatusResponse{}, offlineOpts, quietLogger())
	go sess.Serve(context.Background())

	client := frame.NewConn(clientConn)
	writePacket(t, client, 0x00, func(w *codec.Writer) {
		w.VarInt(775)
		w.String("localhost")
		w.UShort(25565)
		w.VarInt(packet.IntentLogin)
	})
	// LoginStart: name + a client UUID that the server should ignore in offline mode.
	writePacket(t, client, 0x00, func(w *codec.Writer) {
		w.String("Notch")
		w.UUID(codec.UUID{})
	})

	body, err := client.ReadPacket()
	if err != nil {
		t.Fatalf("read LoginSuccess: %v", err)
	}
	r := codec.NewReader(body)
	if id := r.VarInt(); id != 0x02 {
		t.Fatalf("LoginSuccess id = %#x", id)
	}
	gotUUID := r.UUID()
	gotName := r.String(16)
	nProps := r.VarInt()
	if r.Err() != nil {
		t.Fatalf("decode LoginSuccess: %v", r.Err())
	}
	if gotName != "Notch" {
		t.Errorf("name = %q, want Notch", gotName)
	}
	if gotUUID != auth.OfflineUUID("Notch") {
		t.Errorf("uuid = %s, want offline UUID", gotUUID)
	}
	if nProps != 0 {
		t.Errorf("properties = %d, want 0", nProps)
	}

	// Acknowledge login; the session must accept it (transition to Configuration)
	// without erroring.
	writePacket(t, client, 0x03, func(*codec.Writer) {})
	_ = clientConn.Close()
}

// TestLoginOnlineMode drives the full M2 login: handshake -> LoginStart ->
// Encryption Request -> Encryption Response (real RSA) -> encryption enabled ->
// Set Compression -> compression enabled -> Login Success, asserting the
// authenticated profile's UUID/name come back through the encrypted, compressed
// framing. The Mojang check is stubbed so the test needs no network.
func TestLoginOnlineMode(t *testing.T) {
	kp, err := auth.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	wantProfile := &auth.Profile{ID: "069a79f444e94726a5befca90e38aaf5", Name: "Notch"}

	var gotHash, gotUser string
	opts := Options{
		OnlineMode:           true,
		KeyPair:              kp,
		CompressionThreshold: 16,
		Authenticate: func(_ context.Context, username, serverHash string) (*auth.Profile, error) {
			gotUser, gotHash = username, serverHash
			return wantProfile, nil
		},
	}

	clientConn, serverConn := net.Pipe()
	sess := New(serverConn, text.StatusResponse{}, opts, quietLogger())
	go sess.Serve(context.Background())

	client := frame.NewConn(clientConn)
	writePacket(t, client, 0x00, func(w *codec.Writer) {
		w.VarInt(775)
		w.String("localhost")
		w.UShort(25565)
		w.VarInt(packet.IntentLogin)
	})
	writePacket(t, client, 0x00, func(w *codec.Writer) {
		w.String("Notch")
		w.UUID(codec.UUID{})
	})

	// Encryption Request (cb 0x01): empty server id, public key, verify token,
	// should-authenticate.
	body, err := client.ReadPacket()
	if err != nil {
		t.Fatalf("read EncryptionRequest: %v", err)
	}
	r := codec.NewReader(body)
	if id := r.VarInt(); id != 0x01 {
		t.Fatalf("EncryptionRequest id = %#x", id)
	}
	if serverID := r.String(20); serverID != "" {
		t.Errorf("server id = %q, want empty", serverID)
	}
	pubDER := r.ByteArray()
	token := r.ByteArray()
	shouldAuth := r.Bool()
	if r.Err() != nil {
		t.Fatalf("decode EncryptionRequest: %v", r.Err())
	}
	if !shouldAuth {
		t.Error("ShouldAuthenticate = false, want true")
	}

	// Client side: parse the public key, pick a 16-byte secret, RSA-encrypt the
	// secret and the echoed token (PKCS#1 v1.5), exactly like a real client.
	pub, err := x509.ParsePKIXPublicKey(pubDER)
	if err != nil {
		t.Fatalf("parse public key: %v", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		t.Fatalf("public key is %T, want *rsa.PublicKey", pub)
	}
	secret := []byte("0123456789abcdef")
	encSecret, err := rsa.EncryptPKCS1v15(crand.Reader, rsaPub, secret)
	if err != nil {
		t.Fatal(err)
	}
	encToken, err := rsa.EncryptPKCS1v15(crand.Reader, rsaPub, token)
	if err != nil {
		t.Fatal(err)
	}
	writePacket(t, client, 0x01, func(w *codec.Writer) {
		w.ByteArray(encSecret)
		w.ByteArray(encToken)
	})
	// The next byte in each direction is encrypted (the client enables it right
	// after sending Encryption Response, mirroring the server).
	if err := client.EnableEncryption(secret); err != nil {
		t.Fatal(err)
	}

	// Set Compression (cb 0x03): encrypted, but not yet compressed.
	body, err = client.ReadPacket()
	if err != nil {
		t.Fatalf("read SetCompression: %v", err)
	}
	r = codec.NewReader(body)
	if id := r.VarInt(); id != 0x03 {
		t.Fatalf("SetCompression id = %#x", id)
	}
	thr := r.VarInt()
	if thr != 16 {
		t.Errorf("threshold = %d, want 16", thr)
	}
	client.SetCompressionThreshold(int(thr))

	// Login Success (cb 0x02): the first packet under the encrypted+compressed
	// framing. Its UUID/name come from the authenticated profile.
	body, err = client.ReadPacket()
	if err != nil {
		t.Fatalf("read LoginSuccess: %v", err)
	}
	r = codec.NewReader(body)
	if id := r.VarInt(); id != 0x02 {
		t.Fatalf("LoginSuccess id = %#x", id)
	}
	gotUUID := r.UUID()
	gotName := r.String(16)
	if r.Err() != nil {
		t.Fatalf("decode LoginSuccess: %v", r.Err())
	}
	wantUUID, _ := wantProfile.UUID()
	if gotUUID != wantUUID {
		t.Errorf("uuid = %s, want %s", gotUUID, wantUUID)
	}
	if gotName != "Notch" {
		t.Errorf("name = %q, want Notch", gotName)
	}

	// The Mojang check saw the claimed username and the hash over the same
	// (empty) server id, secret, and public key.
	if gotUser != "Notch" {
		t.Errorf("authenticate username = %q", gotUser)
	}
	if wantHash := auth.ServerHash("", secret, pubDER); gotHash != wantHash {
		t.Errorf("server hash = %q, want %q", gotHash, wantHash)
	}

	_ = clientConn.Close()
}

// TestLoginOnlineModeWrongToken asserts a tampered verify token is rejected: the
// session must error (and close) rather than complete the login.
func TestLoginOnlineModeWrongToken(t *testing.T) {
	kp, err := auth.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	opts := Options{
		OnlineMode:           true,
		KeyPair:              kp,
		CompressionThreshold: frame.CompressionDisabled,
		Authenticate: func(context.Context, string, string) (*auth.Profile, error) {
			return nil, fmt.Errorf("must not be called on a bad token")
		},
	}

	clientConn, serverConn := net.Pipe()
	done := make(chan struct{})
	go func() {
		New(serverConn, text.StatusResponse{}, opts, quietLogger()).Serve(context.Background())
		close(done)
	}()

	client := frame.NewConn(clientConn)
	writePacket(t, client, 0x00, func(w *codec.Writer) {
		w.VarInt(775)
		w.String("localhost")
		w.UShort(25565)
		w.VarInt(packet.IntentLogin)
	})
	writePacket(t, client, 0x00, func(w *codec.Writer) {
		w.String("Notch")
		w.UUID(codec.UUID{})
	})

	body, err := client.ReadPacket()
	if err != nil {
		t.Fatalf("read EncryptionRequest: %v", err)
	}
	r := codec.NewReader(body)
	_ = r.VarInt() // id
	_ = r.String(20)
	pubDER := r.ByteArray()
	_ = r.ByteArray() // token (deliberately not echoed)
	pub, err := x509.ParsePKIXPublicKey(pubDER)
	if err != nil {
		t.Fatal(err)
	}
	rsaPub := pub.(*rsa.PublicKey)

	secret := []byte("0123456789abcdef")
	encSecret, _ := rsa.EncryptPKCS1v15(crand.Reader, rsaPub, secret)
	encWrongToken, _ := rsa.EncryptPKCS1v15(crand.Reader, rsaPub, []byte{0xFF, 0xFF, 0xFF, 0xFF})
	writePacket(t, client, 0x01, func(w *codec.Writer) {
		w.ByteArray(encSecret)
		w.ByteArray(encWrongToken)
	})

	<-done // Serve must return (login refused) rather than hang
}

func TestInvalidNextStateClosesConn(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	sess := New(serverConn, text.StatusResponse{}, offlineOpts, quietLogger())
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
