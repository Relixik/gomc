package auth

import (
	"crypto/md5"  //nolint:gosec // MD5 is protocol-mandated for offline UUIDs, not used for security.
	"crypto/sha1" //nolint:gosec // SHA-1 is protocol-mandated for the server hash, not used for security.
	"math/big"

	"github.com/Relixik/gomc/internal/protocol/codec"
)

// ServerHash computes the Minecraft "server id hash" = SHA-1(serverID ++
// sharedSecret ++ publicKeyDER), rendered in Minecraft's signed-hex form. It is
// passed to the Mojang session server's hasJoined check.
func ServerHash(serverID string, sharedSecret, publicKeyDER []byte) string {
	h := sha1.New() //nolint:gosec // see import note
	h.Write([]byte(serverID))
	h.Write(sharedSecret)
	h.Write(publicKeyDER)
	return minecraftDigest(h.Sum(nil))
}

// minecraftDigest renders a hash as Java's BigInteger(sum).toString(16) would:
// the bytes are interpreted as a two's-complement signed big-endian integer
// (so a set high bit yields a negative, '-'-prefixed result) and printed in
// base 16 with no leading zeros.
func minecraftDigest(sum []byte) string {
	n := new(big.Int).SetBytes(sum)
	if len(sum) > 0 && sum[0]&0x80 != 0 {
		n.Sub(n, new(big.Int).Lsh(big.NewInt(1), uint(len(sum))*8))
	}
	return n.Text(16)
}

// OfflineUUID derives the offline-mode player UUID as Java's
// UUID.nameUUIDFromBytes does: a version-3 (MD5) UUID over "OfflinePlayer:"+name.
func OfflineUUID(name string) codec.UUID {
	sum := md5.Sum([]byte("OfflinePlayer:" + name)) //nolint:gosec // see import note
	var u codec.UUID
	copy(u[:], sum[:])
	u[6] = (u[6] & 0x0f) | 0x30 // version 3
	u[8] = (u[8] & 0x3f) | 0x80 // RFC 4122 variant
	return u
}
