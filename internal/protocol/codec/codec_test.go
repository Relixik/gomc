package codec

import (
	"bytes"
	"errors"
	"testing"
)

func TestVarInt(t *testing.T) {
	cases := []struct {
		v    int32
		want []byte
	}{
		{0, []byte{0x00}},
		{1, []byte{0x01}},
		{2, []byte{0x02}},
		{127, []byte{0x7f}},
		{128, []byte{0x80, 0x01}},
		{255, []byte{0xff, 0x01}},
		{25565, []byte{0xdd, 0xc7, 0x01}},
		{2097151, []byte{0xff, 0xff, 0x7f}},
		{2147483647, []byte{0xff, 0xff, 0xff, 0xff, 0x07}},
		{-1, []byte{0xff, 0xff, 0xff, 0xff, 0x0f}},
		{-2147483648, []byte{0x80, 0x80, 0x80, 0x80, 0x08}},
	}
	for _, c := range cases {
		w := NewWriter()
		w.VarInt(c.v)
		if !bytes.Equal(w.Bytes(), c.want) {
			t.Errorf("write VarInt(%d) = % x, want % x", c.v, w.Bytes(), c.want)
		}
		r := NewReader(c.want)
		got := r.VarInt()
		if err := r.Err(); err != nil {
			t.Errorf("read VarInt(% x): unexpected err %v", c.want, err)
		}
		if got != c.v {
			t.Errorf("read VarInt(% x) = %d, want %d", c.want, got, c.v)
		}
		if r.Remaining() != 0 {
			t.Errorf("read VarInt(%d) left %d trailing bytes", c.v, r.Remaining())
		}
	}
}

func TestVarLong(t *testing.T) {
	cases := []struct {
		v    int64
		want []byte
	}{
		{0, []byte{0x00}},
		{1, []byte{0x01}},
		{127, []byte{0x7f}},
		{128, []byte{0x80, 0x01}},
		{255, []byte{0xff, 0x01}},
		{2147483647, []byte{0xff, 0xff, 0xff, 0xff, 0x07}},
		{4294967295, []byte{0xff, 0xff, 0xff, 0xff, 0x0f}},
		{9223372036854775807, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f}},
		{-1, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01}},
		{-2147483648, []byte{0x80, 0x80, 0x80, 0x80, 0xf8, 0xff, 0xff, 0xff, 0xff, 0x01}},
		{-9223372036854775808, []byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x01}},
	}
	for _, c := range cases {
		w := NewWriter()
		w.VarLong(c.v)
		if !bytes.Equal(w.Bytes(), c.want) {
			t.Errorf("write VarLong(%d) = % x, want % x", c.v, w.Bytes(), c.want)
		}
		r := NewReader(c.want)
		got := r.VarLong()
		if err := r.Err(); err != nil {
			t.Errorf("read VarLong(% x): unexpected err %v", c.want, err)
		}
		if got != c.v {
			t.Errorf("read VarLong(% x) = %d, want %d", c.want, got, c.v)
		}
		if r.Remaining() != 0 {
			t.Errorf("read VarLong(%d) left %d trailing bytes", c.v, r.Remaining())
		}
	}
}

func TestVarIntTooLong(t *testing.T) {
	// Six continuation bytes: never terminates within the 5-byte cap.
	r := NewReader([]byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80})
	_ = r.VarInt()
	if !errors.Is(r.Err(), ErrVarIntTooLong) {
		t.Errorf("expected ErrVarIntTooLong, got %v", r.Err())
	}
}

func TestVarLongTooLong(t *testing.T) {
	r := NewReader([]byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80})
	_ = r.VarLong()
	if !errors.Is(r.Err(), ErrVarLongTooLong) {
		t.Errorf("expected ErrVarLongTooLong, got %v", r.Err())
	}
}

func TestNumerics(t *testing.T) {
	t.Run("UShort", func(t *testing.T) {
		w := NewWriter()
		w.UShort(25565)
		if !bytes.Equal(w.Bytes(), []byte{0x63, 0xdd}) {
			t.Errorf("UShort = % x", w.Bytes())
		}
	})
	t.Run("Int", func(t *testing.T) {
		w := NewWriter()
		w.Int(-1)
		if !bytes.Equal(w.Bytes(), []byte{0xff, 0xff, 0xff, 0xff}) {
			t.Errorf("Int(-1) = % x", w.Bytes())
		}
	})
	t.Run("Long", func(t *testing.T) {
		w := NewWriter()
		w.Long(1)
		if !bytes.Equal(w.Bytes(), []byte{0, 0, 0, 0, 0, 0, 0, 1}) {
			t.Errorf("Long(1) = % x", w.Bytes())
		}
	})
	t.Run("Float", func(t *testing.T) {
		w := NewWriter()
		w.Float(1.0)
		if !bytes.Equal(w.Bytes(), []byte{0x3f, 0x80, 0x00, 0x00}) {
			t.Errorf("Float(1.0) = % x", w.Bytes())
		}
	})
	t.Run("Double", func(t *testing.T) {
		w := NewWriter()
		w.Double(1.0)
		if !bytes.Equal(w.Bytes(), []byte{0x3f, 0xf0, 0, 0, 0, 0, 0, 0}) {
			t.Errorf("Double(1.0) = % x", w.Bytes())
		}
	})
	t.Run("Bool", func(t *testing.T) {
		w := NewWriter()
		w.Bool(true)
		w.Bool(false)
		if !bytes.Equal(w.Bytes(), []byte{0x01, 0x00}) {
			t.Errorf("Bool = % x", w.Bytes())
		}
	})
}

func TestNumericRoundTrip(t *testing.T) {
	w := NewWriter()
	w.Byte(-7)
	w.UByte(200)
	w.Short(-12345)
	w.UShort(54321)
	w.Int(-1234567)
	w.Long(-1234567890123)
	w.Float(3.5)
	w.Double(-2.718281828)
	w.Angle(128)

	r := NewReader(w.Bytes())
	if got := r.Byte(); got != -7 {
		t.Errorf("Byte = %d", got)
	}
	if got := r.UByte(); got != 200 {
		t.Errorf("UByte = %d", got)
	}
	if got := r.Short(); got != -12345 {
		t.Errorf("Short = %d", got)
	}
	if got := r.UShort(); got != 54321 {
		t.Errorf("UShort = %d", got)
	}
	if got := r.Int(); got != -1234567 {
		t.Errorf("Int = %d", got)
	}
	if got := r.Long(); got != -1234567890123 {
		t.Errorf("Long = %d", got)
	}
	if got := r.Float(); got != 3.5 {
		t.Errorf("Float = %v", got)
	}
	if got := r.Double(); got != -2.718281828 {
		t.Errorf("Double = %v", got)
	}
	if got := r.Angle(); got != 128 {
		t.Errorf("Angle = %d", got)
	}
	if r.Err() != nil || r.Remaining() != 0 {
		t.Errorf("err=%v remaining=%d", r.Err(), r.Remaining())
	}
}

func TestString(t *testing.T) {
	cases := []struct {
		s    string
		want []byte
	}{
		{"", []byte{0x00}},
		{"hi", []byte{0x02, 'h', 'i'}},
		{"é", []byte{0x02, 0xc3, 0xa9}}, // U+00E9 in UTF-8
	}
	for _, c := range cases {
		w := NewWriter()
		w.String(c.s)
		if !bytes.Equal(w.Bytes(), c.want) {
			t.Errorf("write String(%q) = % x, want % x", c.s, w.Bytes(), c.want)
		}
		r := NewReader(c.want)
		if got := r.String(0); got != c.s || r.Err() != nil {
			t.Errorf("read String(% x) = %q err=%v, want %q", c.want, got, r.Err(), c.s)
		}
	}
}

func TestStringTooLong(t *testing.T) {
	// Length prefix claims 8 chars * 4 + 1 = 33 bytes against a max of 8.
	r := NewReader([]byte{0x21}) // VarInt 33, then nothing
	_ = r.String(8)
	if !errors.Is(r.Err(), ErrStringTooLong) {
		t.Errorf("expected ErrStringTooLong, got %v", r.Err())
	}
}

func TestStringInvalidUTF8(t *testing.T) {
	r := NewReader([]byte{0x01, 0xff}) // length 1, byte 0xff is not valid UTF-8
	_ = r.String(0)
	if !errors.Is(r.Err(), ErrStringInvalid) {
		t.Errorf("expected ErrStringInvalid, got %v", r.Err())
	}
}

func TestPositionGolden(t *testing.T) {
	w := NewWriter()
	w.Position(1, 2, 3)
	want := []byte{0x00, 0x00, 0x00, 0x40, 0x00, 0x00, 0x30, 0x02}
	if !bytes.Equal(w.Bytes(), want) {
		t.Errorf("Position(1,2,3) = % x, want % x", w.Bytes(), want)
	}
}

func TestPositionRoundTrip(t *testing.T) {
	cases := [][3]int32{
		{0, 0, 0},
		{1, 2, 3},
		{-1, -1, -1},
		{33554431, 2047, -33554432},  // max x, max y, min z
		{-33554432, -2048, 33554431}, // min x, min y, max z
		{1000000, 320, -1000000},
	}
	for _, c := range cases {
		w := NewWriter()
		w.Position(c[0], c[1], c[2])
		r := NewReader(w.Bytes())
		x, y, z := r.Position()
		if r.Err() != nil {
			t.Errorf("Position%v read err: %v", c, r.Err())
		}
		if x != c[0] || y != c[1] || z != c[2] {
			t.Errorf("Position%v round-trip = (%d,%d,%d)", c, x, y, z)
		}
	}
}

func TestUUID(t *testing.T) {
	const s = "00112233-4455-6677-8899-aabbccddeeff"
	u, err := ParseUUID(s)
	if err != nil {
		t.Fatalf("ParseUUID: %v", err)
	}
	want := UUID{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	if u != want {
		t.Errorf("ParseUUID = % x", u)
	}
	if u.String() != s {
		t.Errorf("String = %q, want %q", u.String(), s)
	}

	w := NewWriter()
	w.UUID(u)
	if !bytes.Equal(w.Bytes(), want[:]) {
		t.Errorf("write UUID = % x", w.Bytes())
	}
	r := NewReader(w.Bytes())
	if got := r.UUID(); got != u || r.Err() != nil {
		t.Errorf("read UUID = % x err=%v", got, r.Err())
	}

	if _, err := ParseUUID("not-a-uuid"); err == nil {
		t.Error("expected error for invalid UUID")
	}
}

func TestByteArrayAndBitSet(t *testing.T) {
	w := NewWriter()
	w.ByteArray([]byte{0xde, 0xad, 0xbe, 0xef})
	w.BitSet([]uint64{0x1, 0xFFFFFFFFFFFFFFFF})

	r := NewReader(w.Bytes())
	if got := r.ByteArray(); !bytes.Equal(got, []byte{0xde, 0xad, 0xbe, 0xef}) {
		t.Errorf("ByteArray round-trip = % x", got)
	}
	bs := r.BitSet()
	if len(bs) != 2 || bs[0] != 0x1 || bs[1] != 0xFFFFFFFFFFFFFFFF {
		t.Errorf("BitSet round-trip = %#x", bs)
	}
	if r.Err() != nil || r.Remaining() != 0 {
		t.Errorf("err=%v remaining=%d", r.Err(), r.Remaining())
	}
}

func TestTruncatedAndStickyError(t *testing.T) {
	r := NewReader([]byte{0x00, 0x01}) // only 2 bytes
	_ = r.Int()                        // wants 4 -> truncated
	if !errors.Is(r.Err(), ErrTruncated) {
		t.Errorf("expected ErrTruncated, got %v", r.Err())
	}
	// Sticky: a subsequent read is a no-op returning the zero value, error unchanged.
	if got := r.Long(); got != 0 {
		t.Errorf("post-error read returned %d, want 0", got)
	}
	if !errors.Is(r.Err(), ErrTruncated) {
		t.Errorf("sticky error changed to %v", r.Err())
	}
}
