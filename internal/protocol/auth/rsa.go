package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
)

// rsaKeyBits is the key size Minecraft uses for the login encryption exchange.
const rsaKeyBits = 1024

// KeyPair is the server's RSA key used during online-mode login.
type KeyPair struct {
	Private *rsa.PrivateKey
	// PublicDER is the X.509/PKIX (SubjectPublicKeyInfo) DER encoding sent in
	// Encryption Request — byte-for-byte what Java's PublicKey.getEncoded()
	// produces, which both sides must agree on for the server hash to match.
	PublicDER []byte
}

// GenerateKeyPair creates a fresh server key pair.
func GenerateKeyPair() (*KeyPair, error) {
	priv, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return nil, err
	}
	return &KeyPair{Private: priv, PublicDER: der}, nil
}

// Decrypt recovers a value the client encrypted to the server's public key with
// PKCS#1 v1.5 (the shared secret and the verify token).
func (kp *KeyPair) Decrypt(ciphertext []byte) ([]byte, error) {
	return rsa.DecryptPKCS1v15(rand.Reader, kp.Private, ciphertext)
}
