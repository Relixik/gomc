package packet

import (
	"bytes"
	"testing"

	"github.com/Relixik/gomc/internal/protocol/codec"
)

func TestEncryptionRequestEncode(t *testing.T) {
	req := &EncryptionRequest{
		ServerID:           "",
		PublicKey:          []byte{0xDE, 0xAD, 0xBE, 0xEF},
		VerifyToken:        []byte{0x01, 0x02, 0x03, 0x04},
		ShouldAuthenticate: true,
	}
	if req.ID() != 0x01 {
		t.Errorf("id = %#x, want 0x01", req.ID())
	}
	w := codec.NewWriter()
	req.Encode(w)

	// String("") + ByteArray(4) + ByteArray(4) + Bool(true).
	want := []byte{
		0x00,                         // server id length 0
		0x04, 0xDE, 0xAD, 0xBE, 0xEF, // public key
		0x04, 0x01, 0x02, 0x03, 0x04, // verify token
		0x01, // should authenticate
	}
	if !bytes.Equal(w.Bytes(), want) {
		t.Errorf("encoded % x, want % x", w.Bytes(), want)
	}

	// Round-trip the fields back out.
	r := codec.NewReader(w.Bytes())
	if id := r.String(20); id != "" {
		t.Errorf("server id = %q, want empty", id)
	}
	if pk := r.ByteArray(); !bytes.Equal(pk, req.PublicKey) {
		t.Errorf("public key = % x", pk)
	}
	if tok := r.ByteArray(); !bytes.Equal(tok, req.VerifyToken) {
		t.Errorf("verify token = % x", tok)
	}
	if !r.Bool() {
		t.Error("should authenticate = false, want true")
	}
	if r.Err() != nil || r.Remaining() != 0 {
		t.Errorf("err=%v remaining=%d", r.Err(), r.Remaining())
	}
}

func TestEncryptionResponseDecode(t *testing.T) {
	w := codec.NewWriter()
	w.ByteArray([]byte{0xAA, 0xBB})
	w.ByteArray([]byte{0xCC, 0xDD, 0xEE})

	r := codec.NewReader(w.Bytes())
	var resp EncryptionResponse
	resp.Decode(r)
	if r.Err() != nil {
		t.Fatalf("decode: %v", r.Err())
	}
	if !bytes.Equal(resp.SharedSecret, []byte{0xAA, 0xBB}) {
		t.Errorf("shared secret = % x", resp.SharedSecret)
	}
	if !bytes.Equal(resp.VerifyToken, []byte{0xCC, 0xDD, 0xEE}) {
		t.Errorf("verify token = % x", resp.VerifyToken)
	}
	if r.Remaining() != 0 {
		t.Errorf("trailing bytes: %d", r.Remaining())
	}
}

func TestSetCompressionEncode(t *testing.T) {
	sc := &SetCompression{Threshold: 256}
	if sc.ID() != 0x03 {
		t.Errorf("id = %#x, want 0x03", sc.ID())
	}
	w := codec.NewWriter()
	sc.Encode(w)
	if want := []byte{0x80, 0x02}; !bytes.Equal(w.Bytes(), want) { // VarInt(256)
		t.Errorf("encoded % x, want % x", w.Bytes(), want)
	}
	r := codec.NewReader(w.Bytes())
	if got := r.VarInt(); got != 256 {
		t.Errorf("threshold = %d, want 256", got)
	}
}

func TestLoginRegistryLookup(t *testing.T) {
	for _, id := range []int32{idLoginStart, idEncryptionResponse, idLoginAcknowledged} {
		if _, ok := NewServerbound(StateLogin, id); !ok {
			t.Errorf("login serverbound %#x not registered", id)
		}
	}
	if d, ok := NewServerbound(StateLogin, idEncryptionResponse); !ok {
		t.Error("encryption response not registered")
	} else if _, isResp := d.(*EncryptionResponse); !isResp {
		t.Errorf("got %T, want *EncryptionResponse", d)
	}
}
