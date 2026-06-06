package codec

import (
	"encoding/binary"
	"math"
	"unicode/utf8"
)

func (r *Reader) rawByte() byte {
	b := r.take(1)
	if b == nil {
		return 0
	}
	return b[0]
}

// Bool reads a single byte as a boolean (any non-zero value is true).
func (r *Reader) Bool() bool { return r.rawByte() != 0 }

// Byte reads a signed byte.
func (r *Reader) Byte() int8 { return int8(r.rawByte()) }

// UByte reads an unsigned byte.
func (r *Reader) UByte() byte { return r.rawByte() }

// Angle reads an angle encoded as 1 byte = 1/256 of a full turn.
func (r *Reader) Angle() byte { return r.rawByte() }

// Short reads a big-endian signed 16-bit integer.
func (r *Reader) Short() int16 {
	b := r.take(2)
	if b == nil {
		return 0
	}
	return int16(binary.BigEndian.Uint16(b))
}

// UShort reads a big-endian unsigned 16-bit integer.
func (r *Reader) UShort() uint16 {
	b := r.take(2)
	if b == nil {
		return 0
	}
	return binary.BigEndian.Uint16(b)
}

// Int reads a big-endian signed 32-bit integer.
func (r *Reader) Int() int32 {
	b := r.take(4)
	if b == nil {
		return 0
	}
	return int32(binary.BigEndian.Uint32(b))
}

// Long reads a big-endian signed 64-bit integer.
func (r *Reader) Long() int64 {
	b := r.take(8)
	if b == nil {
		return 0
	}
	return int64(binary.BigEndian.Uint64(b))
}

// Float reads a big-endian IEEE-754 32-bit float.
func (r *Reader) Float() float32 {
	b := r.take(4)
	if b == nil {
		return 0
	}
	return math.Float32frombits(binary.BigEndian.Uint32(b))
}

// Double reads a big-endian IEEE-754 64-bit float.
func (r *Reader) Double() float64 {
	b := r.take(8)
	if b == nil {
		return 0
	}
	return math.Float64frombits(binary.BigEndian.Uint64(b))
}

// VarInt reads a variable-length 32-bit integer (plain two's-complement, LSB
// group first, max 5 bytes). Overlong encodings set ErrVarIntTooLong.
func (r *Reader) VarInt() int32 {
	var result uint32
	var shift uint
	for i := 0; i < maxVarIntBytes; i++ {
		b := r.take(1)
		if b == nil {
			return 0
		}
		result |= uint32(b[0]&0x7F) << shift
		if b[0]&0x80 == 0 {
			return int32(result)
		}
		shift += 7
	}
	r.err = ErrVarIntTooLong
	return 0
}

// VarLong reads a variable-length 64-bit integer (max 10 bytes). Overlong
// encodings set ErrVarLongTooLong.
func (r *Reader) VarLong() int64 {
	var result uint64
	var shift uint
	for i := 0; i < maxVarLongBytes; i++ {
		b := r.take(1)
		if b == nil {
			return 0
		}
		result |= uint64(b[0]&0x7F) << shift
		if b[0]&0x80 == 0 {
			return int64(result)
		}
		shift += 7
	}
	r.err = ErrVarLongTooLong
	return 0
}

// String reads a VarInt-length-prefixed UTF-8 string. maxLen caps the length in
// characters (<= 0 means MaxStringLen); the byte length is bounded at maxLen*4
// for anti-DoS, and invalid UTF-8 sets ErrStringInvalid.
func (r *Reader) String(maxLen int) string {
	if maxLen <= 0 {
		maxLen = MaxStringLen
	}
	n := r.VarInt()
	if r.err != nil {
		return ""
	}
	if n < 0 {
		r.err = ErrNegativeLength
		return ""
	}
	if int(n) > maxLen*4 {
		r.err = ErrStringTooLong
		return ""
	}
	b := r.take(int(n))
	if b == nil {
		return ""
	}
	if !utf8.Valid(b) {
		r.err = ErrStringInvalid
		return ""
	}
	return string(b)
}

// Identifier reads a namespaced identifier (a String capped at MaxStringLen).
func (r *Reader) Identifier() string { return r.String(MaxStringLen) }

// UUID reads a 128-bit UUID (16 bytes).
func (r *Reader) UUID() UUID {
	var u UUID
	b := r.take(16)
	if b == nil {
		return u
	}
	copy(u[:], b)
	return u
}

// Position reads a block position packed into a 64-bit big-endian value as
// x:26, z:26, y:12 (the modern X,Z,Y layout), sign-extending each field.
func (r *Reader) Position() (x, y, z int32) {
	v := r.Long()
	if r.err != nil {
		return 0, 0, 0
	}
	x = int32(v >> 38)
	y = int32(v << 52 >> 52)
	z = int32(v << 26 >> 38)
	return x, y, z
}

// Raw reads exactly n raw bytes, returning a copy.
func (r *Reader) Raw(n int) []byte {
	b := r.take(n)
	if b == nil {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

// ByteArray reads a VarInt-length-prefixed byte slice, returning a copy.
func (r *Reader) ByteArray() []byte {
	n := r.VarInt()
	if r.err != nil {
		return nil
	}
	return r.Raw(int(n))
}

// RemainingBytes reads and returns a copy of all unread bytes.
func (r *Reader) RemainingBytes() []byte {
	if r.err != nil {
		return nil
	}
	return r.Raw(len(r.data) - r.pos)
}

// BitSet reads a length-prefixed BitSet as a VarInt long-count followed by that
// many big-endian longs. The count is bounded for anti-DoS.
func (r *Reader) BitSet() []uint64 {
	n := r.VarInt()
	if r.err != nil {
		return nil
	}
	if n < 0 {
		r.err = ErrNegativeLength
		return nil
	}
	if int(n) > maxBitSetLongs {
		r.err = ErrBitSetTooLong
		return nil
	}
	out := make([]uint64, n)
	for i := range out {
		out[i] = uint64(r.Long())
	}
	if r.err != nil {
		return nil
	}
	return out
}
