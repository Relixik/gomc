package nbt

import "unicode/utf16"

// NBT strings use Java's "modified UTF-8", which differs from standard UTF-8 in
// two ways: the null character U+0000 is encoded as two bytes (0xC0 0x80), and
// characters above U+FFFF are encoded as a UTF-16 surrogate pair (two 3-byte
// sequences, CESU-8 style) rather than a single 4-byte sequence. For ordinary
// BMP text without nulls it is identical to standard UTF-8.

// encodeMUTF8 encodes s to modified UTF-8.
func encodeMUTF8(s string) []byte {
	units := utf16.Encode([]rune(s))
	out := make([]byte, 0, len(units))
	for _, c := range units {
		switch {
		case c >= 0x0001 && c <= 0x007F:
			out = append(out, byte(c))
		case c == 0 || c <= 0x07FF:
			out = append(out, byte(0xC0|(c>>6)), byte(0x80|(c&0x3F)))
		default: // 0x0800..0xFFFF, including surrogate code units
			out = append(out, byte(0xE0|(c>>12)), byte(0x80|((c>>6)&0x3F)), byte(0x80|(c&0x3F)))
		}
	}
	return out
}

// decodeMUTF8 decodes modified UTF-8 bytes to a Go string.
func decodeMUTF8(b []byte) (string, error) {
	units := make([]uint16, 0, len(b))
	for i := 0; i < len(b); {
		a := b[i]
		switch {
		case a&0x80 == 0:
			units = append(units, uint16(a))
			i++
		case a&0xE0 == 0xC0:
			if i+1 >= len(b) || b[i+1]&0xC0 != 0x80 {
				return "", errBadMUTF8
			}
			units = append(units, uint16(a&0x1F)<<6|uint16(b[i+1]&0x3F))
			i += 2
		case a&0xF0 == 0xE0:
			if i+2 >= len(b) || b[i+1]&0xC0 != 0x80 || b[i+2]&0xC0 != 0x80 {
				return "", errBadMUTF8
			}
			units = append(units, uint16(a&0x0F)<<12|uint16(b[i+1]&0x3F)<<6|uint16(b[i+2]&0x3F))
			i += 3
		default:
			return "", errBadMUTF8
		}
	}
	return string(utf16.Decode(units)), nil
}
