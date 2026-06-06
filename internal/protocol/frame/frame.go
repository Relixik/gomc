package frame

import (
	"bytes"
	"compress/zlib"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/Relixik/gomc/internal/protocol/codec"
)

const (
	// MaxPacketSize caps a packet's length field at 2^21-1, which also bounds
	// the length VarInt to 3 bytes (anti-DoS).
	MaxPacketSize = 2097151
	maxLenBytes   = 3

	// maxUncompressed bounds a decompressed packet to defend against zlib bombs
	// while still allowing large chunk packets.
	maxUncompressed = 16 << 20

	// CompressionDisabled is the threshold value meaning "no compression".
	CompressionDisabled = -1
)

var (
	ErrPacketTooLarge = errors.New("frame: packet length exceeds maximum")
	ErrBadLength      = errors.New("frame: invalid packet length")
	ErrSizeMismatch   = errors.New("frame: decompressed size mismatch")
)

// Conn frames packets over a net.Conn, owning the per-direction transform stack
// (encryption outermost, then compression, then the length-delimited frame).
// It is not safe for concurrent reads, nor for concurrent writes; the session
// layer uses one read goroutine and one write goroutine per connection.
type Conn struct {
	raw       net.Conn
	src       io.Reader
	dst       io.Writer
	threshold int
}

// NewConn wraps c with compression disabled and no encryption.
func NewConn(c net.Conn) *Conn {
	return &Conn{raw: c, src: c, dst: c, threshold: CompressionDisabled}
}

// Raw returns the underlying connection (for addresses, deadlines, close).
func (c *Conn) Raw() net.Conn { return c.raw }

// Close closes the underlying connection.
func (c *Conn) Close() error { return c.raw.Close() }

// SetCompressionThreshold enables compression for packets of at least t bytes
// (t >= 0), or disables it with CompressionDisabled. Must mirror the value sent
// to the client in Set Compression.
func (c *Conn) SetCompressionThreshold(t int) { c.threshold = t }

// EnableEncryption turns on AES-128/CFB8 using the 16-byte shared secret as
// BOTH key and IV, a single continuous stream per direction. Call exactly once,
// right after the Encryption Response is processed; the very next byte in each
// direction is encrypted.
func (c *Conn) EnableEncryption(secret []byte) error {
	if len(secret) != 16 {
		return fmt.Errorf("frame: shared secret must be 16 bytes, got %d", len(secret))
	}
	block, err := aes.NewCipher(secret)
	if err != nil {
		return err
	}
	c.src = cipher.StreamReader{S: newCFB8(block, secret, true), R: c.src}
	c.dst = cipher.StreamWriter{S: newCFB8(block, secret, false), W: c.dst}
	return nil
}

// readLength reads the packet-length VarInt, which the protocol caps at 3 bytes.
func (c *Conn) readLength() (int, error) {
	var result uint32
	var shift uint
	var buf [1]byte
	for i := 0; i < maxLenBytes; i++ {
		if _, err := io.ReadFull(c.src, buf[:]); err != nil {
			return 0, err
		}
		result |= uint32(buf[0]&0x7F) << shift
		if buf[0]&0x80 == 0 {
			return int(result), nil
		}
		shift += 7
	}
	return 0, ErrPacketTooLarge
}

// ReadPacket reads one packet and returns its decompressed, decrypted body
// (packet id + fields). The returned slice is freshly allocated.
func (c *Conn) ReadPacket() ([]byte, error) {
	length, err := c.readLength()
	if err != nil {
		return nil, err
	}
	if length < 1 || length > MaxPacketSize {
		return nil, ErrBadLength
	}
	raw := make([]byte, length)
	if _, err := io.ReadFull(c.src, raw); err != nil {
		return nil, err
	}
	if c.threshold < 0 {
		return raw, nil
	}

	// Compressed framing: DataLength (uncompressed size; 0 = stored) + payload.
	rr := codec.NewReader(raw)
	dataLen := rr.VarInt()
	if rr.Err() != nil {
		return nil, rr.Err()
	}
	payload := raw[rr.Pos():]
	if dataLen == 0 {
		return payload, nil
	}
	if dataLen < 0 || int(dataLen) > maxUncompressed {
		return nil, ErrPacketTooLarge
	}

	zr, err := zlib.NewReader(bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	out := make([]byte, dataLen)
	if _, err := io.ReadFull(zr, out); err != nil {
		return nil, err
	}
	// The stream must contain exactly dataLen bytes.
	var extra [1]byte
	if n, _ := zr.Read(extra[:]); n != 0 {
		return nil, ErrSizeMismatch
	}
	return out, nil
}

// WritePacket frames and writes one packet body (id + fields).
func (c *Conn) WritePacket(body []byte) error {
	if c.threshold < 0 {
		w := codec.NewWriter()
		w.VarInt(int32(len(body)))
		w.Raw(body)
		return c.writeAll(w.Bytes())
	}

	inner := codec.NewWriter()
	if len(body) < c.threshold {
		inner.VarInt(0) // stored (uncompressed)
		inner.Raw(body)
	} else {
		var zb bytes.Buffer
		zw := zlib.NewWriter(&zb)
		if _, err := zw.Write(body); err != nil {
			return err
		}
		if err := zw.Close(); err != nil {
			return err
		}
		inner.VarInt(int32(len(body))) // uncompressed size
		inner.Raw(zb.Bytes())
	}

	out := codec.NewWriter()
	out.VarInt(int32(inner.Len()))
	out.Raw(inner.Bytes())
	return c.writeAll(out.Bytes())
}

func (c *Conn) writeAll(b []byte) error {
	_, err := c.dst.Write(b)
	return err
}
