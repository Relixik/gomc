package frame

import "crypto/cipher"

// cfb8 implements AES in 8-bit cipher-feedback mode — the mode Minecraft uses.
// The standard library's crypto/cipher CFB is a 128-bit-segment CFB and is
// therefore WRONG for the protocol, so this is hand-rolled over the AES block.
//
// Ported from the prior implementation (old/impl/conn/crypto/cfb8.go). The
// feedback register always has the CIPHERTEXT byte appended, for both encrypt
// and decrypt. A sliding buffer avoids shifting the register on every byte.
type cfb8 struct {
	block   cipher.Block
	size    int
	sr      []byte // shift-register backing buffer (4 blocks wide)
	srEnc   []byte // scratch for the encrypted register
	srPos   int
	decrypt bool
}

func newCFB8(block cipher.Block, iv []byte, decrypt bool) cipher.Stream {
	size := block.BlockSize()
	x := &cfb8{
		block:   block,
		size:    size,
		sr:      make([]byte, size*4),
		srEnc:   make([]byte, size),
		decrypt: decrypt,
	}
	copy(x.sr, iv)
	return x
}

func (x *cfb8) XORKeyStream(dst, src []byte) {
	for i := 0; i < len(src); i++ {
		x.block.Encrypt(x.srEnc, x.sr[x.srPos:x.srPos+x.size])

		var c byte
		if x.decrypt {
			c = src[i]
			dst[i] = c ^ x.srEnc[0]
		} else {
			c = src[i] ^ x.srEnc[0]
			dst[i] = c
		}

		x.sr[x.srPos+x.size] = c
		x.srPos++

		if x.srPos+x.size == len(x.sr) {
			copy(x.sr, x.sr[x.srPos:])
			x.srPos = 0
		}
	}
}
