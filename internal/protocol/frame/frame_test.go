package frame

import (
	"bytes"
	"crypto/aes"
	"net"
	"testing"
)

func TestCFB8RoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef")
	block, _ := aes.NewCipher(key)
	plain := []byte("the quick brown fox jumps over the lazy dog, and then again!!")

	ct := make([]byte, len(plain))
	newCFB8(block, key, false).XORKeyStream(ct, plain)
	if bytes.Equal(ct, plain) {
		t.Fatal("ciphertext equals plaintext")
	}
	got := make([]byte, len(ct))
	newCFB8(block, key, true).XORKeyStream(got, ct)
	if !bytes.Equal(got, plain) {
		t.Errorf("round trip = %q", got)
	}
}

// TestCFB8MatchesSpec checks the first two output bytes against the CFB8
// definition computed directly from the AES block, proving it is genuine 8-bit
// CFB and not the standard library's 128-bit-segment variant.
func TestCFB8MatchesSpec(t *testing.T) {
	key := []byte("an example 16 by")
	block, _ := aes.NewCipher(key)
	iv := key
	plain := []byte{0xAA, 0x55}

	var sr [16]byte
	copy(sr[:], iv)
	var ks [16]byte
	block.Encrypt(ks[:], sr[:])
	c0 := plain[0] ^ ks[0]
	copy(sr[:], sr[1:]) // shift left one byte
	sr[15] = c0         // append ciphertext byte
	block.Encrypt(ks[:], sr[:])
	c1 := plain[1] ^ ks[0]

	out := make([]byte, 2)
	newCFB8(block, iv, false).XORKeyStream(out, plain)
	if out[0] != c0 || out[1] != c1 {
		t.Errorf("CFB8 = % x, want % x", out, []byte{c0, c1})
	}
}

func TestCFB8StreamingEquivalence(t *testing.T) {
	key := []byte("0123456789abcdef")
	block, _ := aes.NewCipher(key)
	plain := bytes.Repeat([]byte{0x01, 0x02, 0x03}, 50) // > one block window

	all := make([]byte, len(plain))
	newCFB8(block, key, false).XORKeyStream(all, plain)

	s := newCFB8(block, key, false)
	bb := make([]byte, len(plain))
	for i := range plain {
		s.XORKeyStream(bb[i:i+1], plain[i:i+1])
	}
	if !bytes.Equal(all, bb) {
		t.Error("byte-by-byte output differs from bulk output")
	}
}

func pipePair(t *testing.T) (*Conn, *Conn) {
	t.Helper()
	a, b := net.Pipe()
	t.Cleanup(func() { _ = a.Close(); _ = b.Close() })
	return NewConn(a), NewConn(b)
}

func roundTrip(t *testing.T, ca, cb *Conn, body []byte) {
	t.Helper()
	errc := make(chan error, 1)
	go func() { errc <- ca.WritePacket(body) }()
	got, err := cb.ReadPacket()
	if err != nil {
		t.Fatalf("ReadPacket: %v", err)
	}
	if werr := <-errc; werr != nil {
		t.Fatalf("WritePacket: %v", werr)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("round trip = % x, want % x", got, body)
	}
}

func TestFrameUncompressed(t *testing.T) {
	ca, cb := pipePair(t)
	roundTrip(t, ca, cb, []byte{0x00, 0xDE, 0xAD, 0xBE, 0xEF})
}

func TestFrameCompressedStored(t *testing.T) {
	ca, cb := pipePair(t)
	ca.SetCompressionThreshold(64)
	cb.SetCompressionThreshold(64)
	roundTrip(t, ca, cb, []byte{0x01, 0x02, 0x03}) // below threshold -> stored
}

func TestFrameCompressedDeflated(t *testing.T) {
	ca, cb := pipePair(t)
	ca.SetCompressionThreshold(16)
	cb.SetCompressionThreshold(16)
	roundTrip(t, ca, cb, bytes.Repeat([]byte{0xAB, 0xCD}, 300)) // above threshold -> zlib
}

func TestFrameEncrypted(t *testing.T) {
	ca, cb := pipePair(t)
	secret := []byte("0123456789abcdef")
	if err := ca.EnableEncryption(secret); err != nil {
		t.Fatal(err)
	}
	if err := cb.EnableEncryption(secret); err != nil {
		t.Fatal(err)
	}
	roundTrip(t, ca, cb, []byte{0x2A, 0x01, 0x02, 0x03, 0x04, 0x05})
}

func TestFrameEncryptedAndCompressed(t *testing.T) {
	ca, cb := pipePair(t)
	secret := []byte("fedcba9876543210")
	if err := ca.EnableEncryption(secret); err != nil {
		t.Fatal(err)
	}
	if err := cb.EnableEncryption(secret); err != nil {
		t.Fatal(err)
	}
	ca.SetCompressionThreshold(8)
	cb.SetCompressionThreshold(8)
	roundTrip(t, ca, cb, bytes.Repeat([]byte{0x07, 0x08}, 100))
}

func TestEnableEncryptionBadSecret(t *testing.T) {
	ca, _ := pipePair(t)
	if err := ca.EnableEncryption([]byte("too short")); err == nil {
		t.Error("expected error for non-16-byte secret")
	}
}

func TestFrameRejectsOversizedLength(t *testing.T) {
	a, b := net.Pipe()
	t.Cleanup(func() { _ = a.Close(); _ = b.Close() })
	cb := NewConn(b)
	go func() { _, _ = a.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF}) }() // 4-byte length VarInt
	if _, err := cb.ReadPacket(); err == nil {
		t.Error("expected error on oversized length VarInt")
	}
}
