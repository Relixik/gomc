package session

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"sync"
	"time"

	"github.com/Relixik/gomc/internal/game/loop"
	"github.com/Relixik/gomc/internal/game/world"
	"github.com/Relixik/gomc/internal/protocol/auth"
	"github.com/Relixik/gomc/internal/protocol/codec"
	"github.com/Relixik/gomc/internal/protocol/frame"
	"github.com/Relixik/gomc/internal/protocol/packet"
	"github.com/Relixik/gomc/internal/protocol/registry"
	"github.com/Relixik/gomc/internal/protocol/text"
)

// errClose ends the read loop cleanly (e.g. after answering a status ping).
var errClose = errors.New("session: closing")

// viewDistance (in chunks) advertised in Login(play); the join streams a
// matching (2N+1)² grid of chunks around the player, following their movement.
const viewDistance = 8

// keepAlivePeriod is how often a clientbound Keep Alive is sent in Play (the
// client is disconnected by vanilla after ~15s of silence).
const keepAlivePeriod = 10 * time.Second

// Options configures online-mode authentication and compression for a session.
type Options struct {
	// OnlineMode requires the AES/CFB8 encryption handshake plus Mojang session
	// verification before login completes. When false the session runs offline:
	// the UUID is derived from the name and no encryption is negotiated.
	OnlineMode bool

	// KeyPair is the server's RSA key used for the encryption handshake. Required
	// when OnlineMode is true; ignored otherwise (it may be shared across sessions).
	KeyPair *auth.KeyPair

	// CompressionThreshold enables zlib compression for packet bodies of at least
	// this many bytes once login completes; a negative value disables compression.
	CompressionThreshold int

	// Authenticate verifies an online-mode join with the Mojang session server.
	// Defaults to auth.HasJoined; tests override it to avoid network access.
	Authenticate func(ctx context.Context, username, serverHash string) (*auth.Profile, error)

	// Hub is the shared player registry that broadcasts presence between
	// connections. nil runs the session standalone (single player, no presence).
	Hub *loop.Hub

	// World is the shared block world; chunk payloads come from it so block
	// changes persist. nil falls back to the pristine superflat columns.
	World *world.World
}

// Session drives one connection through the protocol state machine. Lifecycle
// states (handshaking/status/login/configuration) are handled inline on the
// read goroutine; Play adds an asynchronous keep-alive sender, so writes are
// serialised through sendMu.
type Session struct {
	conn   *frame.Conn
	state  packet.State
	logger *slog.Logger
	status text.StatusResponse
	opts   Options
	ctx    context.Context // the connection's context, set in Serve

	sendMu sync.Mutex    // serialises clientbound writes
	done   chan struct{} // closed when Serve returns

	// Set during login.
	username    string
	uuid        codec.UUID
	properties  []packet.LoginProperty
	verifyToken []byte // expected echo in the Encryption Response (online mode)
	entityID    int32

	// Play-state chunk streaming (read-goroutine only — no locking needed).
	chunkX, chunkZ int32             // the player's current center chunk
	loaded         map[[2]int32]bool // chunk columns currently sent to the client

	// Play-state presence: hub-originated packets are queued on out and flushed
	// by a dedicated writer goroutine; joined gates the hub leave on disconnect.
	out    chan []byte
	joined bool

	// Last known player position/rotation, tracked across movement packets (each
	// carries only some of these fields) and forwarded to the hub.
	posX, posY, posZ float64
	yaw, pitch       float32
	onGround         bool

	// Hotbar contents (item ids per hotbar slot, -1 = empty) and the selected
	// slot, tracked from creative-mode slot sets and held-item changes so a place
	// action knows which block is in hand. A minimal inventory for block placing.
	hotbar       [9]int32
	selectedSlot int32
}

// New wraps conn in a Session in the Handshaking state. status is the snapshot
// served to server-list pings; opts configures online mode and compression.
func New(conn net.Conn, status text.StatusResponse, opts Options, logger *slog.Logger) *Session {
	if logger == nil {
		logger = slog.Default()
	}
	if opts.Authenticate == nil {
		opts.Authenticate = auth.HasJoined
	}
	return &Session{
		conn:   frame.NewConn(conn),
		state:  packet.StateHandshaking,
		logger: logger,
		status: status,
		opts:   opts,
		done:   make(chan struct{}),
	}
}

// Serve runs the read loop until the connection closes, the context is
// cancelled, or a handler signals completion. It always closes the connection.
func (s *Session) Serve(ctx context.Context) {
	s.ctx = ctx
	defer s.conn.Close()
	defer close(s.done)
	defer s.leaveHub()

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
			s.logger.Info("connection closed by peer", "remote", addr, "state", s.state, "err", err)
			return
		}
		if err := s.handle(body); err != nil {
			if !errors.Is(err, errClose) {
				s.logger.Warn("session error", "remote", addr, "state", s.state, "err", err)
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
		// Tolerate packets we don't handle yet (the client sends many in Play)
		// — ignore rather than disconnect. The whole frame is already consumed.
		s.logger.Debug("ignoring unhandled packet", "state", s.state, "id", fmt.Sprintf("%#x", id))
		return nil
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
	case *packet.EncryptionResponse:
		return s.onEncryptionResponse(p)
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
	case *packet.MovePlayerPos:
		s.posX, s.posY, s.posZ = p.X, p.Y, p.Z
		s.onGround = p.Flags&1 != 0
		return s.handleMove()
	case *packet.MovePlayerPosRot:
		s.posX, s.posY, s.posZ = p.X, p.Y, p.Z
		s.yaw, s.pitch = p.Yaw, p.Pitch
		s.onGround = p.Flags&1 != 0
		return s.handleMove()
	case *packet.MovePlayerRot:
		s.yaw, s.pitch = p.Yaw, p.Pitch
		s.onGround = p.Flags&1 != 0
		return s.handleMove()
	case *packet.ChatMessage:
		if s.joined && s.opts.Hub != nil {
			s.opts.Hub.Chat(s.entityID, p.Message)
		}
		return nil
	case *packet.PlayerAction:
		return s.onPlayerAction(p)
	case *packet.SetCarriedItem:
		if p.Slot >= 0 && p.Slot < int16(len(s.hotbar)) {
			s.selectedSlot = int32(p.Slot)
		}
		return nil
	case *packet.SetCreativeModeSlot:
		s.onCreativeSlot(p)
		return nil
	case *packet.UseItemOn:
		return s.onUseItemOn(p)
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
	s.username = p.Name
	if s.opts.OnlineMode {
		// Online mode: start the encryption handshake; the profile (and real UUID)
		// arrive after the Mojang session check in onEncryptionResponse.
		return s.beginEncryption()
	}
	// Offline mode: derive the UUID from the name; no encryption or Mojang auth.
	// The client-supplied UUID is ignored.
	s.uuid = auth.OfflineUUID(p.Name)
	return s.finishLogin()
}

// beginEncryption sends Encryption Request with a fresh verify token. The client
// replies with the RSA-encrypted shared secret and the echoed token.
func (s *Session) beginEncryption() error {
	token := make([]byte, 4)
	if _, err := rand.Read(token); err != nil {
		return err
	}
	s.verifyToken = token
	return s.send(&packet.EncryptionRequest{
		ServerID:           "", // empty since 1.7, still hashed into the server id
		PublicKey:          s.opts.KeyPair.PublicDER,
		VerifyToken:        token,
		ShouldAuthenticate: true,
	})
}

// onEncryptionResponse decrypts the shared secret and verify token, enables
// encryption (from the next byte in each direction), verifies the join with
// Mojang, and adopts the authenticated profile before finishing login.
func (s *Session) onEncryptionResponse(p *packet.EncryptionResponse) error {
	secret, err := s.opts.KeyPair.Decrypt(p.SharedSecret)
	if err != nil {
		return fmt.Errorf("decrypt shared secret: %w", err)
	}
	if len(secret) != 16 {
		return fmt.Errorf("shared secret is %d bytes, want 16", len(secret))
	}
	token, err := s.opts.KeyPair.Decrypt(p.VerifyToken)
	if err != nil {
		return fmt.Errorf("decrypt verify token: %w", err)
	}
	if !bytes.Equal(token, s.verifyToken) {
		return errors.New("verify token mismatch")
	}
	if err := s.conn.EnableEncryption(secret); err != nil {
		return err
	}

	// The server hash must use the same (empty) server id, secret, and public key
	// the client hashed, or Mojang reports the player as not authenticated.
	hash := auth.ServerHash("", secret, s.opts.KeyPair.PublicDER)
	profile, err := s.opts.Authenticate(s.ctx, s.username, hash)
	if err != nil {
		return fmt.Errorf("authenticate %q: %w", s.username, err)
	}
	uuid, err := profile.UUID()
	if err != nil {
		return fmt.Errorf("parse profile uuid: %w", err)
	}
	s.username = profile.Name
	s.uuid = uuid
	s.properties = make([]packet.LoginProperty, len(profile.Properties))
	for i, pr := range profile.Properties {
		s.properties[i] = packet.LoginProperty{Name: pr.Name, Value: pr.Value, Signature: pr.Signature}
	}
	s.logger.Info("player authenticated (online)", "name", s.username, "uuid", s.uuid)
	return s.finishLogin()
}

// finishLogin optionally enables compression (Set Compression must precede
// Login Success, which becomes the first packet under the new framing) and then
// sends Login Success.
func (s *Session) finishLogin() error {
	if s.opts.CompressionThreshold >= 0 {
		if err := s.send(&packet.SetCompression{Threshold: int32(s.opts.CompressionThreshold)}); err != nil {
			return err
		}
		s.conn.SetCompressionThreshold(s.opts.CompressionThreshold)
	}
	return s.send(&packet.LoginSuccess{UUID: s.uuid, Name: s.username, Properties: s.properties})
}

func (s *Session) onLoginAcknowledged() error {
	s.state = packet.StateConfiguration
	mode := "offline"
	if s.opts.OnlineMode {
		mode = "online"
	}
	s.logger.Info("player logged in", "name", s.username, "uuid", s.uuid, "mode", mode)
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
	// Tags are mandatory: without them the client fails registry loading with
	// "Unbound tags" (enchantment exclusive_set, dialog, timeline, etc.).
	if err := s.send(&packet.UpdateTags{Data: registry.Tags()}); err != nil {
		return err
	}
	return s.send(&packet.FinishConfiguration{})
}

// enterPlay sends the join sequence: Login(play), the "start waiting for
// chunks" game event, a grid of empty chunks around spawn, and the spawn
// teleport. It then starts the keep-alive loop.
func (s *Session) enterPlay() error {
	s.entityID = 1
	if s.opts.Hub != nil {
		s.entityID = s.opts.Hub.NextEntityID()
	}
	// Start with an empty hotbar; creative slot sets fill it as the player picks
	// blocks (slot 0 is selected by default, matching vanilla).
	for i := range s.hotbar {
		s.hotbar[i] = -1
	}
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
	// Load the initial view around spawn chunk (0,0); streaming then follows the
	// player's movement.
	s.loaded = make(map[[2]int32]bool)
	s.chunkX, s.chunkZ = math.MinInt32, math.MinInt32 // force a full initial load
	if err := s.updateChunks(0, 0); err != nil {
		return err
	}
	// Spawn standing on the grass surface (the flat top block is at Y -61).
	s.posX, s.posY, s.posZ = 0, -60, 0
	if err := s.send(&packet.SyncPlayerPosition{TeleportID: 1, X: s.posX, Y: s.posY, Z: s.posZ}); err != nil {
		return err
	}
	go s.keepAliveLoop()

	// Join the shared world: start flushing hub broadcasts, then register so
	// other players see us and we see them.
	if s.opts.Hub != nil {
		s.out = make(chan []byte, 256)
		go s.drainOutbound()
		s.opts.Hub.Join(loop.JoinRequest{
			EntityID:   s.entityID,
			UUID:       s.uuid,
			Name:       s.username,
			Properties: s.properties,
			X:          s.posX, Y: s.posY, Z: s.posZ,
			Out: s.out,
		})
		s.joined = true
	}
	return nil
}

// onPlayerAction handles block digging: on a (creative-instant or survival-
// finished) break it acknowledges the client's prediction so the break commits,
// then asks the hub to remove the block from the shared world and broadcast it.
func (s *Session) onPlayerAction(p *packet.PlayerAction) error {
	if p.Status != packet.DigStart && p.Status != packet.DigFinish {
		return nil // drop-item / swap-hand / etc. are not handled yet
	}
	if err := s.send(&packet.BlockChangedAck{Sequence: p.Sequence}); err != nil {
		return err
	}
	if s.joined && s.opts.Hub != nil {
		s.opts.Hub.Break(p.X, p.Y, p.Z)
	}
	return nil
}

// onCreativeSlot tracks the hotbar contents the player sets in creative mode so a
// later place action knows which block is in hand. Inventory container slots
// 36..44 are the nine hotbar slots; other slots are ignored (no full inventory
// yet).
func (s *Session) onCreativeSlot(p *packet.SetCreativeModeSlot) {
	const hotbarFirst, hotbarLast = 36, 44
	if p.Slot < hotbarFirst || p.Slot > hotbarLast {
		return
	}
	i := p.Slot - hotbarFirst
	if p.HasItem {
		s.hotbar[i] = p.ItemID
	} else {
		s.hotbar[i] = -1
	}
}

// onUseItemOn handles a right-click on a block face — placing the held block. It
// acknowledges the client's prediction (so it commits rather than rolls back),
// resolves the held item to a block state, computes the cell adjacent to the
// clicked face, and asks the hub to set it in the shared world and broadcast it.
func (s *Session) onUseItemOn(p *packet.UseItemOn) error {
	if err := s.send(&packet.BlockChangedAck{Sequence: p.Sequence}); err != nil {
		return err
	}
	if !s.joined || s.opts.Hub == nil {
		return nil
	}
	item := s.hotbar[s.selectedSlot]
	state, placeable := world.BlockStateForItem(item)
	x, y, z := placeAgainst(p.X, p.Y, p.Z, p.Face)
	// TODO(M5): drop to Debug once placing is confirmed in-game.
	s.logger.Info("use_item_on", "slot", s.selectedSlot, "item", item, "state", state,
		"place", item >= 0 && placeable, "x", x, "y", y, "z", z)
	if item < 0 || !placeable {
		return nil // empty hand or a non-block item (tool, food, …)
	}
	s.opts.Hub.Place(x, y, z, state)
	return nil
}

// placeAgainst returns the cell adjacent to the clicked block on the clicked
// face — where a placed block goes.
func placeAgainst(x, y, z, face int32) (int32, int32, int32) {
	switch face {
	case 0: // bottom (-Y)
		y--
	case 1: // top (+Y)
		y++
	case 2: // north (-Z)
		z--
	case 3: // south (+Z)
		z++
	case 4: // west (-X)
		x--
	case 5: // east (+X)
		x++
	}
	return x, y, z
}

// handleMove processes a movement packet after s.pos*/yaw/pitch have been
// updated: it forwards the new state to the hub (so other players see the move)
// and recentres the streamed chunk view if the player crossed a chunk boundary.
func (s *Session) handleMove() error {
	if s.joined && s.opts.Hub != nil {
		s.opts.Hub.Move(loop.MoveRequest{
			EntityID: s.entityID,
			X:        s.posX, Y: s.posY, Z: s.posZ,
			Yaw: s.yaw, Pitch: s.pitch, OnGround: s.onGround,
		})
	}
	cx, cz := chunkOf(s.posX), chunkOf(s.posZ)
	if cx == s.chunkX && cz == s.chunkZ {
		return nil
	}
	return s.updateChunks(cx, cz)
}

// updateChunks recentres the streamed view on chunk (cx,cz): it sends Set Center
// Chunk, streams in every column now within the view distance that the client
// does not have, and unloads every column that fell outside it.
func (s *Session) updateChunks(cx, cz int32) error {
	if err := s.send(&packet.SetCenterChunk{ChunkX: cx, ChunkZ: cz}); err != nil {
		return err
	}
	want := make(map[[2]int32]bool, (2*viewDistance+1)*(2*viewDistance+1))
	for x := cx - viewDistance; x <= cx+viewDistance; x++ {
		for z := cz - viewDistance; z <= cz+viewDistance; z++ {
			key := [2]int32{x, z}
			want[key] = true
			if !s.loaded[key] {
				if err := s.send(&packet.ChunkData{X: x, Z: z, Payload: s.chunkPayload(x, z)}); err != nil {
					return err
				}
				s.loaded[key] = true
			}
		}
	}
	for key := range s.loaded {
		if !want[key] {
			if err := s.send(&packet.UnloadChunk{X: key[0], Z: key[1]}); err != nil {
				return err
			}
			delete(s.loaded, key)
		}
	}
	s.chunkX, s.chunkZ = cx, cz
	return nil
}

// chunkOf maps a block coordinate to its chunk coordinate (floored division by
// the 16-block chunk width, correct for negatives).
func chunkOf(coord float64) int32 {
	return int32(math.Floor(coord / 16))
}

// chunkPayload returns the ChunkData body for (cx,cz) from the shared world
// (reflecting block changes), or the pristine superflat column when standalone.
func (s *Session) chunkPayload(cx, cz int32) []byte {
	if s.opts.World != nil {
		return s.opts.World.ChunkPayload(cx, cz)
	}
	return world.SuperflatPayload()
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

// send encodes a clientbound packet (id + fields) and writes it through the
// frame layer.
func (s *Session) send(p packet.Encoder) error {
	w := codec.NewWriter()
	w.VarInt(p.ID())
	p.Encode(w)
	return s.sendRaw(w.Bytes())
}

// sendRaw writes a pre-encoded packet body. Writes are serialised through sendMu
// so the read loop, keep-alive loop, and outbound drainer can all send safely.
func (s *Session) sendRaw(body []byte) error {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	return s.conn.WritePacket(body)
}

// leaveHub removes the player from the shared registry on disconnect so the
// other players are told to despawn it (the anti-goroutine-leak removal intent).
func (s *Session) leaveHub() {
	if s.joined && s.opts.Hub != nil {
		s.opts.Hub.Leave(s.entityID)
	}
}

// drainOutbound flushes hub-originated packets queued on s.out to the client
// until the session ends.
func (s *Session) drainOutbound() {
	for {
		select {
		case <-s.done:
			return
		case body := <-s.out:
			if err := s.sendRaw(body); err != nil {
				return
			}
		}
	}
}
