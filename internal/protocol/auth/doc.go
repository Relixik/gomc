// Package auth implements the cryptography and session checks for login.
//
// It generates the server RSA keypair, exports the X.509/DER public key (the
// byte-for-byte equivalent of Java's PublicKey.getEncoded()), decrypts the
// shared secret and verify token with PKCS#1 v1.5, computes Minecraft's
// "signed-hex" SHA-1 server hash (the 20 bytes read as a two's-complement
// signed big integer, base-16, optional leading '-'), calls the Mojang
// sessionserver hasJoined endpoint (online mode), and derives the offline
// UUIDv3 from MD5("OfflinePlayer:" + name).
//
// Stdlib only: crypto/rsa, crypto/rand, crypto/x509, crypto/sha1, crypto/md5,
// math/big, net/http, encoding/json.
package auth
