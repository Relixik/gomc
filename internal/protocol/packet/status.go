package packet

import "github.com/Relixik/gomc/internal/protocol/codec"

const (
	idStatusRequest  = 0x00 // serverbound
	idStatusPing     = 0x01 // serverbound
	idStatusResponse = 0x00 // clientbound
	idStatusPong     = 0x01 // clientbound
)

func init() {
	registerServerbound(StateStatus, idStatusRequest, func() Decoder { return &StatusRequest{} })
	registerServerbound(StateStatus, idStatusPing, func() Decoder { return &StatusPing{} })
}

// StatusRequest asks for the server-list response. (Status, serverbound, 0x00.)
type StatusRequest struct{}

func (p *StatusRequest) Decode(*codec.Reader) {}

// StatusPing carries an opaque payload to be echoed in StatusPong. (Status,
// serverbound, 0x01.)
type StatusPing struct{ Payload int64 }

func (p *StatusPing) Decode(r *codec.Reader) { p.Payload = r.Long() }

// StatusResponse is the JSON server-list document. (Status, clientbound, 0x00.)
type StatusResponse struct{ JSON string }

func (p *StatusResponse) ID() int32              { return idStatusResponse }
func (p *StatusResponse) Encode(w *codec.Writer) { w.String(p.JSON) }

// StatusPong echoes the ping payload. (Status, clientbound, 0x01.)
type StatusPong struct{ Payload int64 }

func (p *StatusPong) ID() int32              { return idStatusPong }
func (p *StatusPong) Encode(w *codec.Writer) { w.Long(p.Payload) }
