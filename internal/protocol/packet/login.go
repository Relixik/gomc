package packet

import "github.com/Relixik/gomc/internal/protocol/codec"

const (
	idLoginStart        = 0x00 // serverbound
	idLoginAcknowledged = 0x03 // serverbound
	idLoginSuccess      = 0x02 // clientbound
)

func init() {
	registerServerbound(StateLogin, idLoginStart, func() Decoder { return &LoginStart{} })
	registerServerbound(StateLogin, idLoginAcknowledged, func() Decoder { return &LoginAcknowledged{} })
}

// LoginStart begins login: the desired name and the client's UUID (which the
// server replaces with the offline UUID in offline mode). (Login, sb, 0x00.)
type LoginStart struct {
	Name string
	UUID codec.UUID
}

func (p *LoginStart) Decode(r *codec.Reader) {
	p.Name = r.String(16)
	p.UUID = r.UUID()
}

// LoginAcknowledged is sent by the client after LoginSuccess; it moves the
// connection into the Configuration state. (Login, sb, 0x03.)
type LoginAcknowledged struct{}

func (p *LoginAcknowledged) Decode(*codec.Reader) {}

// LoginProperty is a signed profile property (e.g. "textures").
type LoginProperty struct {
	Name      string
	Value     string
	Signature string // empty => unsigned
}

// LoginSuccess confirms login and carries the authoritative profile. (Login,
// cb, 0x02.)
type LoginSuccess struct {
	UUID       codec.UUID
	Name       string
	Properties []LoginProperty
}

func (p *LoginSuccess) ID() int32 { return idLoginSuccess }

func (p *LoginSuccess) Encode(w *codec.Writer) {
	w.UUID(p.UUID)
	w.String(p.Name)
	w.VarInt(int32(len(p.Properties)))
	for _, prop := range p.Properties {
		w.String(prop.Name)
		w.String(prop.Value)
		if prop.Signature != "" {
			w.Bool(true)
			w.String(prop.Signature)
		} else {
			w.Bool(false)
		}
	}
}
