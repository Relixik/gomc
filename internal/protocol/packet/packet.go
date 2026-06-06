package packet

import "github.com/Relixik/gomc/internal/protocol/codec"

// The protocol/version this server speaks.
const (
	ProtocolVersion = 775
	GameVersion     = "26.1.2"
)

// State is the connection state; the same packet id means different packets in
// different states, so dispatch keys on (State, Direction, id).
type State byte

const (
	StateHandshaking State = iota
	StateStatus
	StateLogin
	StateConfiguration
	StatePlay
)

func (s State) String() string {
	switch s {
	case StateHandshaking:
		return "handshaking"
	case StateStatus:
		return "status"
	case StateLogin:
		return "login"
	case StateConfiguration:
		return "configuration"
	case StatePlay:
		return "play"
	default:
		return "unknown"
	}
}

// Direction is the travel direction of a packet.
type Direction byte

const (
	Serverbound Direction = iota // client -> server
	Clientbound                  // server -> client
)

// Decoder is a serverbound packet: it reads its fields from the frame body
// (after the id has been consumed by the dispatcher).
type Decoder interface {
	Decode(r *codec.Reader)
}

// Encoder is a clientbound packet: it knows its id and writes its fields.
type Encoder interface {
	ID() int32
	Encode(w *codec.Writer)
}

// serverbound maps a state and packet id to a factory for the matching packet.
// Populated by init() in each packet file so the id lives next to the packet.
var serverbound = map[State]map[int32]func() Decoder{}

func registerServerbound(state State, id int32, f func() Decoder) {
	if serverbound[state] == nil {
		serverbound[state] = map[int32]func() Decoder{}
	}
	if _, dup := serverbound[state][id]; dup {
		// Programmer error caught at startup, not a client input — panic is apt.
		panic("packet: duplicate serverbound registration for " + state.String())
	}
	serverbound[state][id] = f
}

// NewServerbound constructs the registered serverbound packet for (state, id),
// or reports false if none is registered.
func NewServerbound(state State, id int32) (Decoder, bool) {
	m := serverbound[state]
	if m == nil {
		return nil, false
	}
	f, ok := m[id]
	if !ok {
		return nil, false
	}
	return f(), true
}
