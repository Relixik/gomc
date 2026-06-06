package codec

import (
	"encoding/binary"
	"math"
)

// Bool writes a boolean as a single byte (0 or 1).
func (w *Writer) Bool(v bool) {
	if v {
		w.buf.WriteByte(1)
	} else {
		w.buf.WriteByte(0)
	}
}

// Byte writes a signed byte.
func (w *Writer) Byte(v int8) { w.buf.WriteByte(byte(v)) }

// UByte writes an unsigned byte.
func (w *Writer) UByte(v byte) { w.buf.WriteByte(v) }

// Angle writes an angle as 1 byte = 1/256 of a full turn.
func (w *Writer) Angle(v byte) { w.buf.WriteByte(v) }

// Short writes a big-endian signed 16-bit integer.
func (w *Writer) Short(v int16) {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], uint16(v))
	w.buf.Write(b[:])
}

// UShort writes a big-endian unsigned 16-bit integer.
func (w *Writer) UShort(v uint16) {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], v)
	w.buf.Write(b[:])
}

// Int writes a big-endian signed 32-bit integer.
func (w *Writer) Int(v int32) {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], uint32(v))
	w.buf.Write(b[:])
}

// Long writes a big-endian signed 64-bit integer.
func (w *Writer) Long(v int64) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(v))
	w.buf.Write(b[:])
}

// Float writes a big-endian IEEE-754 32-bit float.
func (w *Writer) Float(v float32) {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], math.Float32bits(v))
	w.buf.Write(b[:])
}

// Double writes a big-endian IEEE-754 64-bit float.
func (w *Writer) Double(v float64) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], math.Float64bits(v))
	w.buf.Write(b[:])
}

// VarInt writes a variable-length 32-bit integer (plain two's-complement, so
// negatives always consume the full 5 bytes).
func (w *Writer) VarInt(v int32) {
	uv := uint32(v)
	for {
		if uv&^uint32(0x7F) == 0 {
			w.buf.WriteByte(byte(uv))
			return
		}
		w.buf.WriteByte(byte(uv&0x7F) | 0x80)
		uv >>= 7
	}
}

// VarLong writes a variable-length 64-bit integer (up to 10 bytes).
func (w *Writer) VarLong(v int64) {
	uv := uint64(v)
	for {
		if uv&^uint64(0x7F) == 0 {
			w.buf.WriteByte(byte(uv))
			return
		}
		w.buf.WriteByte(byte(uv&0x7F) | 0x80)
		uv >>= 7
	}
}

// String writes a VarInt-length-prefixed UTF-8 string.
func (w *Writer) String(s string) {
	w.VarInt(int32(len(s)))
	w.buf.WriteString(s)
}

// Identifier writes a namespaced identifier (as a String).
func (w *Writer) Identifier(s string) { w.String(s) }

// UUID writes a 128-bit UUID (16 bytes).
func (w *Writer) UUID(u UUID) { w.buf.Write(u[:]) }

// Position writes a block position packed as x:26, z:26, y:12 into a 64-bit
// big-endian value (modern X,Z,Y layout).
func (w *Writer) Position(x, y, z int32) {
	v := (int64(x&0x3FFFFFF) << 38) | (int64(z&0x3FFFFFF) << 12) | int64(y&0xFFF)
	w.Long(v)
}

// Raw writes raw bytes with no length prefix.
func (w *Writer) Raw(b []byte) { w.buf.Write(b) }

// ByteArray writes a VarInt-length-prefixed byte slice.
func (w *Writer) ByteArray(b []byte) {
	w.VarInt(int32(len(b)))
	w.buf.Write(b)
}

// BitSet writes a BitSet as a VarInt long-count followed by that many longs.
func (w *Writer) BitSet(longs []uint64) {
	w.VarInt(int32(len(longs)))
	for _, l := range longs {
		w.Long(int64(l))
	}
}
