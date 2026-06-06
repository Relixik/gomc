package nbt

import (
	"bytes"
	"errors"
	"testing"

	"github.com/Relixik/gomc/internal/protocol/codec"
)

func TestNetworkGoldenCompound(t *testing.T) {
	// {"foo": Int(42)} as nameless-root network NBT.
	root := NewCompound().Set("foo", Int(42))
	w := codec.NewWriter()
	WriteNetwork(w, root)

	want := []byte{
		TagCompound,               // root type, NO name (network form)
		TagInt,                    // entry type
		0x00, 0x03, 'f', 'o', 'o', // entry name (modified-UTF8, UShort len)
		0x00, 0x00, 0x00, 0x2a, // Int 42
		TagEnd, // compound terminator
	}
	if !bytes.Equal(w.Bytes(), want) {
		t.Fatalf("WriteNetwork = % x\nwant            % x", w.Bytes(), want)
	}

	r := codec.NewReader(w.Bytes())
	got, err := ReadNetwork(r)
	if err != nil {
		t.Fatalf("ReadNetwork: %v", err)
	}
	c, ok := got.(*Compound)
	if !ok {
		t.Fatalf("root is %T, want *Compound", got)
	}
	v, _ := c.Get("foo")
	if v != Int(42) {
		t.Errorf("foo = %v, want Int(42)", v)
	}
}

func TestNetworkStringRoot(t *testing.T) {
	// Text components travel as a String-tag root in network NBT.
	w := codec.NewWriter()
	WriteNetwork(w, String("hi"))
	want := []byte{TagString, 0x00, 0x02, 'h', 'i'}
	if !bytes.Equal(w.Bytes(), want) {
		t.Fatalf("WriteNetwork(String) = % x, want % x", w.Bytes(), want)
	}
	r := codec.NewReader(w.Bytes())
	got, err := ReadNetwork(r)
	if err != nil {
		t.Fatalf("ReadNetwork: %v", err)
	}
	if got != String("hi") {
		t.Errorf("got %v, want String(\"hi\")", got)
	}
}

func TestDiskRootHasName(t *testing.T) {
	w := codec.NewWriter()
	WriteDisk(w, "root", NewCompound().Set("x", Byte(1)))
	want := []byte{
		TagCompound,
		0x00, 0x04, 'r', 'o', 'o', 't', // root name (disk form)
		TagByte,
		0x00, 0x01, 'x',
		0x01,
		TagEnd,
	}
	if !bytes.Equal(w.Bytes(), want) {
		t.Fatalf("WriteDisk = % x\nwant         % x", w.Bytes(), want)
	}
	r := codec.NewReader(w.Bytes())
	name, root, err := ReadDisk(r)
	if err != nil {
		t.Fatalf("ReadDisk: %v", err)
	}
	if name != "root" {
		t.Errorf("name = %q, want root", name)
	}
	if c, ok := root.(*Compound); !ok || c.Len() != 1 {
		t.Errorf("root = %v", root)
	}
}

func TestModifiedUTF8(t *testing.T) {
	t.Run("null becomes C0 80", func(t *testing.T) {
		got := encodeMUTF8("\x00")
		if !bytes.Equal(got, []byte{0xC0, 0x80}) {
			t.Errorf("encode(NUL) = % x, want c0 80", got)
		}
		s, err := decodeMUTF8(got)
		if err != nil || s != "\x00" {
			t.Errorf("decode = %q err=%v", s, err)
		}
	})
	t.Run("supplementary char as surrogate pair", func(t *testing.T) {
		const emoji = "\U0001F600" // 😀, above U+FFFF
		got := encodeMUTF8(emoji)
		// CESU-8: two 3-byte sequences (one per surrogate), 6 bytes total.
		if len(got) != 6 {
			t.Errorf("encode(emoji) = % x (len %d), want 6 bytes", got, len(got))
		}
		s, err := decodeMUTF8(got)
		if err != nil || s != emoji {
			t.Errorf("decode = %q err=%v, want emoji", s, err)
		}
	})
	t.Run("ascii matches plain utf8", func(t *testing.T) {
		if !bytes.Equal(encodeMUTF8("hello"), []byte("hello")) {
			t.Error("ascii should match plain utf8")
		}
	})
}

func TestRoundTripAllTypes(t *testing.T) {
	root := NewCompound().
		Set("b", Byte(-5)).
		Set("s", Short(300)).
		Set("i", Int(-70000)).
		Set("l", Long(1<<40)).
		Set("f", Float(1.5)).
		Set("d", Double(-3.25)).
		Set("str", String("héllo")).
		Set("ba", ByteArray{1, 2, 3}).
		Set("ia", IntArray{10, -20, 30}).
		Set("la", LongArray{1 << 40, -(1 << 40)}).
		Set("list", List{ElemType: TagInt, Elems: []Tag{Int(1), Int(2), Int(3)}}).
		Set("nested", NewCompound().Set("deep", String("ok")))

	w := codec.NewWriter()
	WriteNetwork(w, root)

	r := codec.NewReader(w.Bytes())
	got, err := ReadNetwork(r)
	if err != nil {
		t.Fatalf("ReadNetwork: %v", err)
	}
	if r.Remaining() != 0 {
		t.Errorf("trailing bytes: %d", r.Remaining())
	}

	c := got.(*Compound)
	check := func(k string, want Tag) {
		v, ok := c.Get(k)
		if !ok {
			t.Errorf("missing key %q", k)
			return
		}
		switch want := want.(type) {
		case ByteArray:
			if !bytes.Equal([]byte(v.(ByteArray)), []byte(want)) {
				t.Errorf("%s = %v", k, v)
			}
		default:
			// scalars compare directly; composites checked separately
		}
	}
	if v, _ := c.Get("str"); v != String("héllo") {
		t.Errorf("str = %v", v)
	}
	if v, _ := c.Get("i"); v != Int(-70000) {
		t.Errorf("i = %v", v)
	}
	check("ba", ByteArray{1, 2, 3})
	if v, _ := c.Get("list"); v.(List).Elems[2] != Int(3) {
		t.Errorf("list = %v", v)
	}
	if nested, _ := c.Get("nested"); nested.(*Compound).Len() != 1 {
		t.Errorf("nested = %v", nested)
	}
}

func TestEmptyListWritesEndType(t *testing.T) {
	w := codec.NewWriter()
	WriteNetwork(w, List{ElemType: TagInt, Elems: nil})
	// List root: type byte (9), then element-type (End for empty), then Int length 0.
	want := []byte{TagList, TagEnd, 0x00, 0x00, 0x00, 0x00}
	if !bytes.Equal(w.Bytes(), want) {
		t.Errorf("empty list = % x, want % x", w.Bytes(), want)
	}
}

func TestDecodeUnknownTagFails(t *testing.T) {
	r := codec.NewReader([]byte{0x63}) // type 99
	_, err := ReadNetwork(r)
	if err == nil {
		t.Error("expected error for unknown tag type")
	}
}

func TestDecodeTruncatedFails(t *testing.T) {
	// Compound root, Int entry, but payload truncated.
	r := codec.NewReader([]byte{TagCompound, TagInt, 0x00, 0x01, 'a', 0x00})
	_, err := ReadNetwork(r)
	if !errors.Is(err, codec.ErrTruncated) {
		t.Errorf("expected ErrTruncated, got %v", err)
	}
}
