package packet

import (
	"bytes"
	"testing"

	"github.com/Relixik/gomc/internal/protocol/codec"
	"github.com/Relixik/gomc/internal/protocol/text"
)

// TestSystemChatEncode pins the layout: a plain text component as a nameless
// network-NBT String tag (type 0x08, u16 length, modified-UTF-8 bytes) followed
// by the overlay boolean.
func TestSystemChatEncode(t *testing.T) {
	p := &SystemChat{Content: text.Plain("hi"), Overlay: false}
	if p.ID() != 0x79 {
		t.Errorf("id = %#x, want 0x79", p.ID())
	}
	w := codec.NewWriter()
	p.Encode(w)
	want := []byte{0x08, 0x00, 0x02, 'h', 'i', 0x00} // string tag "hi" + overlay false
	if !bytes.Equal(w.Bytes(), want) {
		t.Errorf("encoded % x, want % x", w.Bytes(), want)
	}
}
