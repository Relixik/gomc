package nbt

// Tag type IDs.
const (
	TagEnd       byte = 0
	TagByte      byte = 1
	TagShort     byte = 2
	TagInt       byte = 3
	TagLong      byte = 4
	TagFloat     byte = 5
	TagDouble    byte = 6
	TagByteArray byte = 7
	TagString    byte = 8
	TagList      byte = 9
	TagCompound  byte = 10
	TagIntArray  byte = 11
	TagLongArray byte = 12
)

// Tag is any NBT value.
type Tag interface {
	// TypeID returns the NBT tag type byte.
	TypeID() byte
}

// Scalar and array tag types map onto Go primitives.
type (
	Byte      int8
	Short     int16
	Int       int32
	Long      int64
	Float     float32
	Double    float64
	ByteArray []byte
	String    string
	IntArray  []int32
	LongArray []int64
)

func (Byte) TypeID() byte      { return TagByte }
func (Short) TypeID() byte     { return TagShort }
func (Int) TypeID() byte       { return TagInt }
func (Long) TypeID() byte      { return TagLong }
func (Float) TypeID() byte     { return TagFloat }
func (Double) TypeID() byte    { return TagDouble }
func (ByteArray) TypeID() byte { return TagByteArray }
func (String) TypeID() byte    { return TagString }
func (IntArray) TypeID() byte  { return TagIntArray }
func (LongArray) TypeID() byte { return TagLongArray }

// List is a homogeneous sequence of tags. When empty, the element type is
// written as TagEnd (per the NBT spec).
type List struct {
	ElemType byte
	Elems    []Tag
}

func (List) TypeID() byte { return TagList }

// Compound is an ordered set of named tags. Order is preserved so that output
// is deterministic (useful for golden tests and capture-diffing against
// vanilla); NBT itself does not assign meaning to compound key order.
type Compound struct {
	keys []string
	vals map[string]Tag
}

// NewCompound returns an empty Compound.
func NewCompound() *Compound {
	return &Compound{vals: make(map[string]Tag)}
}

func (*Compound) TypeID() byte { return TagCompound }

// Set inserts or replaces a named tag, preserving first-insertion order, and
// returns the compound for chaining.
func (c *Compound) Set(name string, t Tag) *Compound {
	if _, ok := c.vals[name]; !ok {
		c.keys = append(c.keys, name)
	}
	c.vals[name] = t
	return c
}

// Get returns the tag stored under name.
func (c *Compound) Get(name string) (Tag, bool) {
	t, ok := c.vals[name]
	return t, ok
}

// Keys returns the names in insertion order.
func (c *Compound) Keys() []string { return c.keys }

// Len returns the number of entries.
func (c *Compound) Len() int { return len(c.keys) }
