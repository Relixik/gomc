// Package nbt implements Minecraft's Named Binary Tag format for tag types
// 0-12, in two modes:
//
//   - NETWORK NBT (the default on the wire since 1.20.2 / protocol 764): the
//     root tag has NO name — just a type byte followed by the payload. The
//     reader must also accept a String tag (type 8) as the root, because text
//     components are transmitted as plain-text NBT strings.
//   - DISK NBT (named root, 2-byte length + name): used only for Anvil
//     persistence (M6).
//
// NBT strings use Java's modified UTF-8 (distinct from the protocol's regular
// UTF-8 String), so a dedicated encoder/decoder is hand-written here.
//
// Stdlib only: encoding/binary, math, errors.
package nbt
