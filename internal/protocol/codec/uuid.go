package codec

import (
	"encoding/hex"
	"fmt"
)

// UUID is a 128-bit identifier in the canonical big-endian byte order used on
// the wire (the same 16 bytes the protocol transmits as two big-endian u64s).
type UUID [16]byte

// String formats the UUID in canonical 8-4-4-4-12 lowercase hex form.
func (u UUID) String() string {
	var buf [36]byte
	hex.Encode(buf[0:8], u[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], u[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], u[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], u[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], u[10:16])
	return string(buf[:])
}

// IsZero reports whether the UUID is all-zero.
func (u UUID) IsZero() bool {
	return u == UUID{}
}

// ParseUUID parses a UUID in canonical hyphenated form or as 32 raw hex digits.
func ParseUUID(s string) (UUID, error) {
	var u UUID
	clean := make([]byte, 0, 32)
	for i := 0; i < len(s); i++ {
		if s[i] != '-' {
			clean = append(clean, s[i])
		}
	}
	if len(clean) != 32 {
		return u, fmt.Errorf("codec: invalid UUID %q: expected 32 hex digits, got %d", s, len(clean))
	}
	if _, err := hex.Decode(u[:], clean); err != nil {
		return u, fmt.Errorf("codec: invalid UUID %q: %w", s, err)
	}
	return u, nil
}
