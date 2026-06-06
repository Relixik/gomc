package nbt

import (
	"errors"
	"fmt"

	"github.com/Relixik/gomc/internal/protocol/codec"
)

var (
	errBadMUTF8 = errors.New("nbt: invalid modified UTF-8")
	errNegLen   = errors.New("nbt: negative length")
	errTooDeep  = errors.New("nbt: nesting too deep")
)

// maxDepth bounds recursion to stop a maliciously deeply-nested tag from
// exhausting the stack (anti-DoS).
const maxDepth = 512

// initCap bounds the initial slice allocation for a length-prefixed sequence;
// the slice still grows as needed, but a huge length prefix on truncated data
// can no longer force a large up-front allocation.
const initCap = 1024

// ReadNetwork reads a nameless-root tag (network NBT). A lone TagEnd yields
// (nil, nil).
func ReadNetwork(r *codec.Reader) (Tag, error) {
	typ := r.UByte()
	if r.Err() != nil {
		return nil, r.Err()
	}
	if typ == TagEnd {
		return nil, nil
	}
	t := readPayload(r, typ, 0)
	return t, r.Err()
}

// ReadDisk reads a named-root tag (disk NBT), returning the root name.
func ReadDisk(r *codec.Reader) (string, Tag, error) {
	typ := r.UByte()
	if r.Err() != nil {
		return "", nil, r.Err()
	}
	if typ == TagEnd {
		return "", nil, nil
	}
	name := readString(r)
	t := readPayload(r, typ, 0)
	return name, t, r.Err()
}

func readString(r *codec.Reader) string {
	n := r.UShort()
	b := r.Raw(int(n))
	if r.Err() != nil {
		return ""
	}
	s, err := decodeMUTF8(b)
	if err != nil {
		r.Fail(err)
		return ""
	}
	return s
}

func readPayload(r *codec.Reader, typ byte, depth int) Tag {
	if depth > maxDepth {
		r.Fail(errTooDeep)
		return nil
	}
	switch typ {
	case TagByte:
		return Byte(r.Byte())
	case TagShort:
		return Short(r.Short())
	case TagInt:
		return Int(r.Int())
	case TagLong:
		return Long(r.Long())
	case TagFloat:
		return Float(r.Float())
	case TagDouble:
		return Double(r.Double())
	case TagByteArray:
		n := r.Int()
		if r.Err() != nil {
			return nil
		}
		if n < 0 {
			r.Fail(errNegLen)
			return nil
		}
		return ByteArray(r.Raw(int(n)))
	case TagString:
		return String(readString(r))
	case TagList:
		et := r.UByte()
		n := r.Int()
		if r.Err() != nil {
			return nil
		}
		if n < 0 {
			r.Fail(errNegLen)
			return nil
		}
		elems := make([]Tag, 0, min(int(n), initCap))
		for i := int32(0); i < n; i++ {
			e := readPayload(r, et, depth+1)
			if r.Err() != nil {
				return nil
			}
			elems = append(elems, e)
		}
		return List{ElemType: et, Elems: elems}
	case TagCompound:
		c := NewCompound()
		for {
			ct := r.UByte()
			if r.Err() != nil {
				return nil
			}
			if ct == TagEnd {
				break
			}
			name := readString(r)
			val := readPayload(r, ct, depth+1)
			if r.Err() != nil {
				return nil
			}
			c.Set(name, val)
		}
		return c
	case TagIntArray:
		n := r.Int()
		if r.Err() != nil {
			return nil
		}
		if n < 0 {
			r.Fail(errNegLen)
			return nil
		}
		arr := make([]int32, 0, min(int(n), initCap))
		for i := int32(0); i < n; i++ {
			arr = append(arr, r.Int())
			if r.Err() != nil {
				return nil
			}
		}
		return IntArray(arr)
	case TagLongArray:
		n := r.Int()
		if r.Err() != nil {
			return nil
		}
		if n < 0 {
			r.Fail(errNegLen)
			return nil
		}
		arr := make([]int64, 0, min(int(n), initCap))
		for i := int32(0); i < n; i++ {
			arr = append(arr, r.Long())
			if r.Err() != nil {
				return nil
			}
		}
		return LongArray(arr)
	default:
		r.Fail(fmt.Errorf("nbt: unknown tag type %d", typ))
		return nil
	}
}
