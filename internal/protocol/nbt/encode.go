package nbt

import "github.com/Relixik/gomc/internal/protocol/codec"

// WriteNetwork writes a tag with a NAMELESS root, the network NBT form used on
// the wire since 1.20.2 (protocol 764). A nil root is written as a lone TagEnd.
func WriteNetwork(w *codec.Writer, root Tag) {
	if root == nil {
		w.UByte(TagEnd)
		return
	}
	w.UByte(root.TypeID())
	writePayload(w, root)
}

// WriteDisk writes a tag with a NAMED root (2-byte length + modified-UTF-8
// name), the form used for Anvil/level.dat persistence.
func WriteDisk(w *codec.Writer, name string, root Tag) {
	if root == nil {
		w.UByte(TagEnd)
		return
	}
	w.UByte(root.TypeID())
	writeString(w, name)
	writePayload(w, root)
}

func writeString(w *codec.Writer, s string) {
	b := encodeMUTF8(s)
	w.UShort(uint16(len(b)))
	w.Raw(b)
}

func writePayload(w *codec.Writer, t Tag) {
	switch v := t.(type) {
	case Byte:
		w.Byte(int8(v))
	case Short:
		w.Short(int16(v))
	case Int:
		w.Int(int32(v))
	case Long:
		w.Long(int64(v))
	case Float:
		w.Float(float32(v))
	case Double:
		w.Double(float64(v))
	case ByteArray:
		w.Int(int32(len(v)))
		w.Raw(v)
	case String:
		writeString(w, string(v))
	case List:
		et := v.ElemType
		if len(v.Elems) == 0 {
			et = TagEnd
		}
		w.UByte(et)
		w.Int(int32(len(v.Elems)))
		for _, e := range v.Elems {
			writePayload(w, e)
		}
	case *Compound:
		for _, k := range v.keys {
			child := v.vals[k]
			w.UByte(child.TypeID())
			writeString(w, k)
			writePayload(w, child)
		}
		w.UByte(TagEnd)
	case IntArray:
		w.Int(int32(len(v)))
		for _, x := range v {
			w.Int(x)
		}
	case LongArray:
		w.Int(int32(len(v)))
		for _, x := range v {
			w.Long(x)
		}
	}
}
