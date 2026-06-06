package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/Relixik/gomc/internal/protocol/auth"
	"github.com/Relixik/gomc/internal/protocol/codec"
	"github.com/Relixik/gomc/internal/protocol/frame"
	"github.com/Relixik/gomc/internal/protocol/packet"
	"github.com/Relixik/gomc/internal/protocol/registry"
	"github.com/Relixik/gomc/internal/protocol/text"
)

// errClose ends the read loop cleanly (e.g. after answering a status ping).
var errClose = errors.New("session: closing")

// viewDistance (in chunks) advertised in Login(play); the join sends a matching
// (2N+1)² grid of chunks around spawn so the client finishes loading terrain.
const viewDistance = 2

// keepAlivePeriod is how often a clientbound Keep Alive is sent in Play (the
// client is disconnected by vanilla after ~15s of silence).
const keepAlivePeriod = 10 * time.Second

// Session drives one connection through the protocol state machine. Lifecycle
// states (handshaking/status/login/configuration) are handled inline on the
// read goroutine; Play adds an asynchronous keep-alive sender, so writes are
// serialised through sendMu.
type Session struct {
	conn   *frame.Conn
	state  packet.State
	logger *slog.Logger
	status text.StatusResponse

	sendMu sync.Mutex    // serialises clientbound writes
	done   chan struct{} // closed when Serve returns

	// Set during login.
	username string
	uuid     codec.UUID
	entityID int32
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
		done:   make(chan struct{}),
	}
}

// Serve runs the read loop until the connection closes, the context is
// cancelled, or a handler signals completion. It always closes the connection.
func (s *Session) Serve(ctx context.Context) {
	defer s.conn.Close()
	defer close(s.done)

	// Unblock the blocking ReadPacket when the context is cancelled; the done
	// channel keeps this watcher (and the keep-alive loop) from leaking.
	go func() {
		select {
		case <-ctx.Done():
			_ = s.conn.Close()
		case <-s.done:
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
	case *packet.ClientInformation:
		s.logger.Debug("client information", "locale", p.Locale, "view", p.ViewDistance)
		return nil
	case *packet.PluginMessageServerbound:
		s.logger.Debug("plugin message", "channel", p.Channel)
		return nil
	case *packet.KnownPacksServerbound:
		return s.onKnownPacks(p)
	case *packet.KeepAliveServerbound:
		return nil
	case *packet.AckFinishConfiguration:
		s.state = packet.StatePlay
		s.logger.Info("entering play", "name", s.username)
		return s.enterPlay()
	case *packet.ConfirmTeleport:
		s.logger.Debug("confirm teleport", "id", p.TeleportID)
		return nil
	case *packet.PlayerLoaded:
		s.logger.Info("player spawned", "name", s.username)
		return nil
	case *packet.MovePlayerPos, *packet.MovePlayerPosRot, *packet.MovePlayerRot:
		return nil // movement accepted (no world simulation yet)
	case *packet.ChatMessage:
		s.logger.Info("chat", "name", s.username, "msg", p.Message)
		return nil
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
	return s.enterConfiguration()
}

// enterConfiguration sends the initial configuration packets: the server brand,
// feature flags, and the known-packs advertisement (the vanilla core pack, so
// registry entries may be sent without inline NBT).
func (s *Session) enterConfiguration() error {
	brand := codec.NewWriter()
	brand.String("gomc")
	if err := s.send(&packet.PluginMessageClientbound{Channel: "minecraft:brand", Data: brand.Bytes()}); err != nil {
		return err
	}
	if err := s.send(&packet.FeatureFlags{Flags: []string{"minecraft:vanilla"}}); err != nil {
		return err
	}
	return s.send(&packet.ClientboundKnownPacks{
		Packs: []packet.KnownPack{{Namespace: "minecraft", ID: "core", Version: packet.GameVersion}},
	})
}

func (s *Session) onKnownPacks(p *packet.KnownPacksServerbound) error {
	s.logger.Debug("client known packs", "count", len(p.Packs))
	// Send the synchronised registries (captured vanilla set/order, all
	// has_data=false since we share the core pack), then finish configuration.
	for _, rd := range registry.SyncedData() {
		if err := s.send(rd); err != nil {
			return err
		}
	}
	return s.send(&packet.FinishConfiguration{})
}

// enterPlay sends the join sequence: Login(play), the "start waiting for
// chunks" game event, a grid of empty chunks around spawn, and the spawn
// teleport. It then starts the keep-alive loop.
func (s *Session) enterPlay() error {
	s.entityID = 1
	if err := s.send(&packet.LoginPlay{
		EntityID:            s.entityID,
		DimensionNames:      []string{"minecraft:overworld", "minecraft:the_nether", "minecraft:the_end"},
		MaxPlayers:          20,
		ViewDistance:        viewDistance,
		SimulationDistance:  viewDistance,
		EnableRespawnScreen: true,
		DimensionType:       0, // overworld (id 0 in our dimension_type registry)
		DimensionName:       "minecraft:overworld",
		GameMode:            1, // creative: the player can fly and won't take fall damage in the void
		PreviousGameMode:    -1,
		IsFlat:              true,
		SeaLevel:            63,
	}); err != nil {
		return err
	}
	if err := s.send(&packet.GameEvent{Event: 13}); err != nil { // start waiting for level chunks
		return err
	}
	if err := s.send(&packet.SetCenterChunk{ChunkX: 0, ChunkZ: 0}); err != nil {
		return err
	}
	for x := int32(-viewDistance); x <= viewDistance; x++ {
		for z := int32(-viewDistance); z <= viewDistance; z++ {
			if err := s.send(&packet.ChunkData{X: x, Z: z}); err != nil {
				return err
			}
		}
	}
	if err := s.send(&packet.SyncPlayerPosition{TeleportID: 1, X: 0, Y: 64, Z: 0}); err != nil {
		return err
	}
	go s.keepAliveLoop()
	return nil
}

// keepAliveLoop periodically sends a clientbound Keep Alive while in Play, until
// the session ends.
func (s *Session) keepAliveLoop() {
	t := time.NewTicker(keepAlivePeriod)
	defer t.Stop()
	var id int64 = 1
	for {
		select {
		case <-s.done:
			return
		case <-t.C:
			if err := s.send(&packet.KeepAlivePlayClientbound{KeepAliveID: id}); err != nil {
				return
			}
			id++
		}
	}
}

// send writes a clientbound packet (id + fields) through the frame layer. Writes
// are serialised so the read loop and keep-alive loop can both send safely.
func (s *Session) send(p packet.Encoder) error {
	w := codec.NewWriter()
	w.VarInt(p.ID())
	p.Encode(w)
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	return s.conn.WritePacket(w.Bytes())
}
