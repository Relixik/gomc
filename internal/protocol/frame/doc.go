// Package frame owns the wire framing and the per-direction transform stack —
// the single trickiest layer of the protocol.
//
// A FrameConn wraps a net.Conn and applies, outermost-first, encryption over
// compression over the raw [length, id, data] frame. Two framing modes coexist
// on ONE connection:
//
//   - Pre-Set-Compression:  Length(VarInt) + PacketID(VarInt) + Data
//   - Post-Set-Compression: PacketLength + DataLength(0 = below threshold,
//     uncompressed) + zlib(PacketID + Data)
//
// Encryption is AES-128 in CFB8 (8-bit segment) mode, with the shared secret
// used as BOTH the key and the IV, one continuous stream per direction that is
// never reset. Go's crypto/cipher CFB is 128-bit-segment and therefore WRONG
// for Minecraft — a 1-byte-segment cipher.Stream is hand-rolled over
// crypto/aes block.Encrypt (port of old/impl/conn/crypto/cfb8.go).
//
// The layer also enforces anti-DoS caps (3-byte length VarInt, 2097151-byte
// packet max), switches transform modes at exact byte boundaries, and peeks the
// first byte for the legacy 0xFE server-list ping.
//
// Stdlib only: compress/zlib, crypto/aes, crypto/cipher, net, bufio.
package frame
