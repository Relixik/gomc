package codec

import (
	"bytes"
	"errors"
)

// Limits and protocol constants.
const (
	// MaxStringLen is the protocol's default maximum String length, in characters.
	MaxStringLen = 32767

	maxVarIntBytes  = 5
	maxVarLongBytes = 10

	// maxBitSetLongs bounds a BitSet allocation to keep a malicious length
	// VarInt from forcing a huge allocation (anti-DoS).
	maxBitSetLongs = 1 << 16
)

// Sticky errors set by Reader. Once set, every subsequent read is a no-op that
// returns the zero value, so a Decode method can read many fields and check
// Err() exactly once at the end — and a malformed packet can never panic the
// connection goroutine (see CONVENTIONS.md §3).
var (
	ErrTruncated      = errors.New("codec: unexpected end of data")
	ErrVarIntTooLong  = errors.New("codec: VarInt exceeds 5 bytes")
	ErrVarLongTooLong = errors.New("codec: VarLong exceeds 10 bytes")
	ErrStringTooLong  = errors.New("codec: string exceeds maximum length")
	ErrStringInvalid  = errors.New("codec: string is not valid UTF-8")
	ErrNegativeLength = errors.New("codec: negative length prefix")
	ErrBitSetTooLong  = errors.New("codec: BitSet exceeds maximum length")
)

// Reader decodes protocol primitives from a single, already decrypted and
// decompressed frame body. It uses a sticky-error model: see the package var
// block above.
type Reader struct {
	data []byte
	pos  int
	err  error
}

// NewReader returns a Reader over data. The slice is not copied; the caller must
// not mutate it while the Reader is in use.
func NewReader(data []byte) *Reader { return &Reader{data: data} }

// Err returns the first error encountered, or nil.
func (r *Reader) Err() error { return r.err }

// Remaining reports how many unread bytes are left (0 once an error is set).
func (r *Reader) Remaining() int {
	if r.err != nil {
		return 0
	}
	return len(r.data) - r.pos
}

// Pos reports the current read offset.
func (r *Reader) Pos() int { return r.pos }

// take returns the next n bytes as a subslice of the underlying buffer (no
// copy), or nil with the sticky error set if fewer than n bytes remain.
// Callers that retain the result beyond the Reader's lifetime must copy it.
func (r *Reader) take(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 {
		r.err = ErrNegativeLength
		return nil
	}
	if r.pos+n > len(r.data) {
		r.err = ErrTruncated
		return nil
	}
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	return b
}

// Writer encodes protocol primitives into an in-memory buffer. Writing to a
// bytes.Buffer never fails, so Writer methods do not return errors.
type Writer struct {
	buf bytes.Buffer
}

// NewWriter returns an empty Writer.
func NewWriter() *Writer { return &Writer{} }

// Bytes returns the accumulated bytes. The slice aliases the internal buffer
// and is invalidated by the next write; copy it if it must outlive the Writer.
func (w *Writer) Bytes() []byte { return w.buf.Bytes() }

// Len returns the number of bytes written so far.
func (w *Writer) Len() int { return w.buf.Len() }

// Reset clears the buffer for reuse.
func (w *Writer) Reset() { w.buf.Reset() }
