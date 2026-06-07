package packet

import (
	"github.com/Relixik/gomc/internal/protocol/codec"
	"github.com/Relixik/gomc/internal/protocol/nbt"
	"github.com/Relixik/gomc/internal/protocol/text"
)

// SystemChat sends an unsigned system message. The content is a text component
// in network-NBT form; Overlay false shows it in the chat box (true = action
// bar). Broadcasting player chat as system messages avoids the signed-chat
// machinery while still delivering global chat. (Play, cb, 0x79.)
type SystemChat struct {
	Content text.Component
	Overlay bool
}

func (p *SystemChat) ID() int32 { return idPlaySystemChat }

func (p *SystemChat) Encode(w *codec.Writer) {
	nbt.WriteNetwork(w, p.Content.NBT())
	w.Bool(p.Overlay)
}
