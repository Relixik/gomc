package packet

import "github.com/Relixik/gomc/internal/protocol/codec"

// Handshake NextState intents.
const (
	IntentStatus   = 1
	IntentLogin    = 2
	IntentTransfer = 3
)

const idHandshake = 0x00

func init() {
	registerServerbound(StateHandshaking, idHandshake, func() Decoder { return &Handshake{} })
}

// Handshake is the first packet a client sends; NextState routes the connection
// to Status or Login. (Handshaking, serverbound, 0x00.)
type Handshake struct {
	ProtocolVersion int32
	ServerAddress   string
	ServerPort      uint16
	NextState       int32
}

func (p *Handshake) Decode(r *codec.Reader) {
	p.ProtocolVersion = r.VarInt()
	p.ServerAddress = r.String(255)
	p.ServerPort = r.UShort()
	p.NextState = r.VarInt()
}
