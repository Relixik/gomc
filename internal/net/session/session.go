package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/Relixik/gomc/internal/protocol/auth"
	"github.com/Relixik/gomc/internal/protocol/codec"
	"github.com/Relixik/gomc/internal/protocol/frame"
	"github.com/Relixik/gomc/internal/protocol/packet"
	"github.com/Relixik/gomc/internal/protocol/text"
)

// errClose ends the read loop cleanly (e.g. after answering a status ping).
var errClose = errors.New("session: closing")

// Session drives one connection through the protocol state machine. Lifecycle
// states (handshaking/status/login/configuration) are handled inline on the
// read goroutine; on the transition to Play the player will be handed to the
// game loop (later milestone).
type Session struct {
	conn   *frame.Conn
	state  packet.State
	logger *slog.Logger
	status text.StatusResponse

	// Set during login.
	username string
	uuid     codec.UUID
}

// New wraps conn in a Session in the Handshaking state. status is the snapshot
// served to server-list pings.
func New(conn net.Conn, status text.StatusResponse, logger *slog.Logger) *Session {
	if logger == nil {
		logger = slog.Default()
	}
	return &Session{
		conn:   frame.NewConn(conn),
		state:  packet.StateHandshaking,
		logger: logger,
		status: status,
	}
}

// Serve runs the read loop until the connection closes, the context is
// cancelled, or a handler signals completion. It always closes the connection.
func (s *Session) Serve(ctx context.Context) {
	defer s.conn.Close()

	// Unblock the blocking ReadPacket when the context is cancelled. The done
	// channel stops this goroutine from leaking once Serve returns.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = s.conn.Close()
		case <-done:
		}
	}()

	addr := s.conn.Raw().RemoteAddr()
	for {
		body, err := s.conn.ReadPacket()
		if err != nil {
			return // EOF / closed / read error ends the session
		}
		if err := s.handle(body); err != nil {
			if !errors.Is(err, errClose) {
				s.logger.Debug("session ended", "remote", addr, "state", s.state, "err", err)
			}
			return
		}
	}
}

func (s *Session) handle(body []byte) error {
	r := codec.NewReader(body)
	id := r.VarInt()
	if r.Err() != nil {
		return r.Err()
	}
	dec, ok := packet.NewServerbound(s.state, id)
	if !ok {
		return fmt.Errorf("unknown packet: state=%s id=%#x", s.state, id)
	}
	dec.Decode(r)
	if r.Err() != nil {
		return fmt.Errorf("decode %T: %w", dec, r.Err())
	}

	switch p := dec.(type) {
	case *packet.Handshake:
		return s.onHandshake(p)
	case *packet.StatusRequest:
		return s.onStatusRequest()
	case *packet.StatusPing:
		return s.onStatusPing(p)
	case *packet.LoginStart:
		return s.onLoginStart(p)
	case *packet.LoginAcknowledged:
		return s.onLoginAcknowledged()
	default:
		return fmt.Errorf("no handler for %T in state %s", p, s.state)
	}
}

func (s *Session) onHandshake(p *packet.Handshake) error {
	switch p.NextState {
	case packet.IntentStatus:
		s.state = packet.StateStatus
	case packet.IntentLogin, packet.IntentTransfer:
		s.state = packet.StateLogin
	default:
		return fmt.Errorf("invalid next state %d", p.NextState)
	}
	return nil
}

func (s *Session) onStatusRequest() error {
	j, err := json.Marshal(s.status)
	if err != nil {
		return err
	}
	return s.send(&packet.StatusResponse{JSON: string(j)})
}

func (s *Session) onStatusPing(p *packet.StatusPing) error {
	if err := s.send(&packet.StatusPong{Payload: p.Payload}); err != nil {
		return err
	}
	return errClose // the client closes the connection after the pong
}

func (s *Session) onLoginStart(p *packet.LoginStart) error {
	// Offline mode (M1): derive the UUID from the name; no encryption or Mojang
	// auth yet (that lands in M2). The client-supplied UUID is ignored.
	s.username = p.Name
	s.uuid = auth.OfflineUUID(p.Name)
	return s.send(&packet.LoginSuccess{UUID: s.uuid, Name: s.username})
}

func (s *Session) onLoginAcknowledged() error {
	s.state = packet.StateConfiguration
	s.logger.Info("player logged in (offline)", "name", s.username, "uuid", s.uuid)
	// Configuration handlers land next; until then the client's first
	// configuration packet will close the session.
	return nil
}

// send writes a clientbound packet (id + fields) through the frame layer.
func (s *Session) send(p packet.Encoder) error {
	w := codec.NewWriter()
	w.VarInt(p.ID())
	p.Encode(w)
	return s.conn.WritePacket(w.Bytes())
}
