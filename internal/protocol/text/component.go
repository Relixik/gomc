package text

import (
	"encoding/json"

	"github.com/Relixik/gomc/internal/protocol/nbt"
)

// Component is a Minecraft text ("chat") component. The zero value with only
// Text set is "plain text". Boolean styles are tri-state pointers so an unset
// style is omitted rather than sent as false.
type Component struct {
	Text          string
	Color         string
	Bold          *bool
	Italic        *bool
	Underlined    *bool
	Strikethrough *bool
	Obfuscated    *bool
	Extra         []Component
}

// Plain returns an unstyled text component.
func Plain(s string) Component { return Component{Text: s} }

// Boolp is a helper for setting tri-state style fields.
func Boolp(b bool) *bool { return &b }

func (c Component) isSimple() bool {
	return c.Color == "" && c.Bold == nil && c.Italic == nil && c.Underlined == nil &&
		c.Strikethrough == nil && c.Obfuscated == nil && len(c.Extra) == 0
}

// NBT renders the component to its on-wire network NBT form: a plain String tag
// when unstyled, otherwise a Compound. This is the form used by Configuration
// and Play packets (chat, disconnect, etc.).
func (c Component) NBT() nbt.Tag {
	if c.isSimple() {
		return nbt.String(c.Text)
	}
	return c.compound()
}

func (c Component) compound() *nbt.Compound {
	comp := nbt.NewCompound()
	comp.Set("text", nbt.String(c.Text))
	if c.Color != "" {
		comp.Set("color", nbt.String(c.Color))
	}
	setBool(comp, "bold", c.Bold)
	setBool(comp, "italic", c.Italic)
	setBool(comp, "underlined", c.Underlined)
	setBool(comp, "strikethrough", c.Strikethrough)
	setBool(comp, "obfuscated", c.Obfuscated)
	if len(c.Extra) > 0 {
		elems := make([]nbt.Tag, len(c.Extra))
		for i, e := range c.Extra {
			elems[i] = e.compound()
		}
		comp.Set("extra", nbt.List{ElemType: nbt.TagCompound, Elems: elems})
	}
	return comp
}

func setBool(c *nbt.Compound, key string, v *bool) {
	if v == nil {
		return
	}
	var b nbt.Byte
	if *v {
		b = 1
	}
	c.Set(key, b)
}

// jsonComponent has the same fields as Component (differing only in struct
// tags, so the two are convertible) and carries the JSON wire names. It exists
// to give Component a custom MarshalJSON without infinite recursion.
type jsonComponent struct {
	Text          string      `json:"text,omitempty"`
	Color         string      `json:"color,omitempty"`
	Bold          *bool       `json:"bold,omitempty"`
	Italic        *bool       `json:"italic,omitempty"`
	Underlined    *bool       `json:"underlined,omitempty"`
	Strikethrough *bool       `json:"strikethrough,omitempty"`
	Obfuscated    *bool       `json:"obfuscated,omitempty"`
	Extra         []Component `json:"extra,omitempty"`
}

// MarshalJSON emits a bare JSON string when the component is unstyled (vanilla
// accepts this), otherwise a full object. This legacy JSON form is used where
// the protocol still expects JSON (e.g. the Status response and the Login-state
// Disconnect packet), as opposed to the NBT form used in Configuration/Play.
func (c Component) MarshalJSON() ([]byte, error) {
	if c.isSimple() {
		return json.Marshal(c.Text)
	}
	return json.Marshal(jsonComponent(c))
}
