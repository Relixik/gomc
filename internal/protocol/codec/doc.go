// Package codec implements the primitive wire types of the Minecraft Java
// protocol: VarInt/VarLong (7-bit groups, capped at 5/10 bytes, plain
// two's-complement — NOT zigzag — and rejecting overlong/overflow encodings),
// String (VarInt byte-length + regular UTF-8, with a UTF-16 code-unit cap),
// Identifier, big-endian numerics, Boolean, Angle (1 byte = 1/256 turn),
// Position (packed x:26,z:26,y:12), UUID (two big-endian u64), BitSet and
// Fixed BitSet, Prefixed Optional/Array, and the registry ID-or-X / ID-Set
// helpers.
//
// A Reader reads from a single (already decrypted + decompressed) frame body;
// a Writer accumulates into a bytes.Buffer. This package has NO knowledge of
// packets — it is the bottom layer everything else builds on.
//
// Stdlib only: encoding/binary, math, bufio, bytes.
package codec
