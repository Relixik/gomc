package loop

import (
	"context"
	"log/slog"
	"sync/atomic"

	"github.com/Relixik/gomc/internal/game/world"
	"github.com/Relixik/gomc/internal/protocol/codec"
	"github.com/Relixik/gomc/internal/protocol/packet"
	"github.com/Relixik/gomc/internal/protocol/text"
)

// Hub is the authoritative registry of online players and the fan-out point for
// presence. All player state lives on the Run goroutine; sessions interact with
// it only through channels (Join/Move/Leave), so there are no locks on the
// registry. Per-connection delivery is a non-blocking enqueue onto the session's
// outbound channel, so one slow client never stalls the hub.
//
// This is the presence core of the eventual 20 TPS game loop (see doc.go); the
// tick itself arrives with world simulation.
type Hub struct {
	nextEID atomic.Int32
	join    chan JoinRequest
	move    chan MoveRequest
	chat    chan chatRequest
	brk     chan blockBreak
	leave   chan int32
	world   *world.World
	logger  *slog.Logger
}

type chatRequest struct {
	eid int32
	msg string
}

type blockBreak struct{ x, y, z int32 }

// JoinRequest registers a player that has entered Play. Out is the session's
// outbound channel onto which the hub enqueues pre-encoded clientbound packets.
type JoinRequest struct {
	EntityID   int32
	UUID       codec.UUID
	Name       string
	Properties []packet.LoginProperty
	X, Y, Z    float64
	Yaw, Pitch float32
	Out        chan<- []byte
}

// MoveRequest updates a player's position/rotation; the hub broadcasts the
// matching delta (or absolute respawn) to the other players.
type MoveRequest struct {
	EntityID   int32
	X, Y, Z    float64
	Yaw, Pitch float32
	OnGround   bool
}

// New creates an unstarted hub backed by w. Call Run in its own goroutine.
func New(w *world.World, logger *slog.Logger) *Hub {
	if logger == nil {
		logger = slog.Default()
	}
	return &Hub{
		join:   make(chan JoinRequest, 32),
		move:   make(chan MoveRequest, 512),
		chat:   make(chan chatRequest, 64),
		brk:    make(chan blockBreak, 64),
		leave:  make(chan int32, 32),
		world:  w,
		logger: logger,
	}
}

// NextEntityID allocates a unique entity id (safe to call from any goroutine).
func (h *Hub) NextEntityID() int32 { return h.nextEID.Add(1) }

// Join registers a player; Leave removes one. Both are rare and may block
// briefly if the hub is busy.
func (h *Hub) Join(r JoinRequest) { h.join <- r }
func (h *Hub) Leave(eid int32)    { h.leave <- eid }

// Move reports a position change; it is dropped if the hub is saturated (the
// next move refreshes the position).
func (h *Hub) Move(r MoveRequest) {
	select {
	case h.move <- r:
	default:
	}
}

// Chat broadcasts a player's chat message to everyone.
func (h *Hub) Chat(eid int32, msg string) { h.chat <- chatRequest{eid: eid, msg: msg} }

// Break sets a block to air in the shared world and tells everyone.
func (h *Hub) Break(x, y, z int32) { h.brk <- blockBreak{x: x, y: y, z: z} }

// Run is the hub's single goroutine; it owns the player registry until ctx ends.
func (h *Hub) Run(ctx context.Context) {
	players := make(map[int32]*player)
	for {
		select {
		case <-ctx.Done():
			return
		case r := <-h.join:
			h.onJoin(players, r)
		case eid := <-h.leave:
			h.onLeave(players, eid)
		case m := <-h.move:
			h.onMove(players, m)
		case c := <-h.chat:
			h.onChat(players, c)
		case b := <-h.brk:
			h.onBreak(players, b)
		}
	}
}

// onBreak removes a block from the world (so it stays broken for future chunk
// loads) and broadcasts the change to every player.
func (h *Hub) onBreak(players map[int32]*player, b blockBreak) {
	if h.world == nil || !h.world.SetBlock(b.x, b.y, b.z, world.Air) {
		return
	}
	body := encode(&packet.BlockUpdate{X: b.x, Y: b.y, Z: b.z, BlockState: int32(world.Air)})
	for _, p := range players {
		enqueue(p.out, body)
	}
}

// onChat broadcasts a player's message to everyone (including the sender) as a
// system message "<name> message".
func (h *Hub) onChat(players map[int32]*player, c chatRequest) {
	p, ok := players[c.eid]
	if !ok {
		return
	}
	body := encode(&packet.SystemChat{Content: text.Plain("<" + p.name + "> " + c.msg)})
	for _, other := range players {
		enqueue(other.out, body)
	}
	h.logger.Info("chat", "name", p.name, "msg", c.msg)
}

type player struct {
	eid     int32
	uuid    codec.UUID
	name    string
	props   []packet.LoginProperty
	x, y, z float64 // last position the clients have been told about
	yaw     byte    // encoded body/head yaw
	pitch   byte
	out     chan<- []byte
}

func (h *Hub) onJoin(players map[int32]*player, r JoinRequest) {
	p := &player{eid: r.EntityID, uuid: r.UUID, name: r.Name, props: r.Properties, x: r.X, y: r.Y, z: r.Z, yaw: degToAngle(r.Yaw), pitch: degToAngle(r.Pitch), out: r.Out}

	// Tell the newcomer about everyone: one player-info list (existing + self, so
	// its own skin renders) plus an entity to render for each existing player.
	infos := make([]packet.PlayerInfoEntry, 0, len(players)+1)
	for _, other := range players {
		infos = append(infos, infoEntry(other))
		enqueue(p.out, encode(spawn(other)))
	}
	infos = append(infos, infoEntry(p))
	enqueue(p.out, encode(&packet.PlayerInfoUpdate{Players: infos}))

	// Tell everyone else about the newcomer.
	newInfo := encode(&packet.PlayerInfoUpdate{Players: []packet.PlayerInfoEntry{infoEntry(p)}})
	newSpawn := encode(spawn(p))
	for _, other := range players {
		enqueue(other.out, newInfo)
		enqueue(other.out, newSpawn)
	}

	players[p.eid] = p
	h.logger.Info("player joined", "name", p.name, "eid", p.eid, "online", len(players))
}

func (h *Hub) onLeave(players map[int32]*player, eid int32) {
	p, ok := players[eid]
	if !ok {
		return
	}
	delete(players, eid)
	remove := encode(&packet.RemoveEntities{EntityIDs: []int32{eid}})
	forget := encode(&packet.PlayerInfoRemove{UUIDs: []codec.UUID{p.uuid}})
	for _, other := range players {
		enqueue(other.out, remove)
		enqueue(other.out, forget)
	}
	h.logger.Info("player left", "name", p.name, "eid", eid, "online", len(players))
}

func infoEntry(p *player) packet.PlayerInfoEntry {
	return packet.PlayerInfoEntry{UUID: p.uuid, Name: p.name, Properties: p.props, Listed: true}
}

func spawn(p *player) *packet.AddEntity {
	return &packet.AddEntity{
		EntityID: p.eid, UUID: p.uuid, Type: packet.PlayerEntityType,
		X: p.x, Y: p.y, Z: p.z,
		Yaw: p.yaw, Pitch: p.pitch, HeadYaw: p.yaw,
	}
}

// onMove broadcasts a player's movement to the others as a position/rotation
// delta. Deltas cover ±8 blocks per axis; a larger jump (a teleport) falls back
// to despawn + respawn so the position stays exact.
func (h *Hub) onMove(players map[int32]*player, m MoveRequest) {
	p, ok := players[m.EntityID]
	if !ok {
		return
	}
	dx := (m.X - p.x) * packet.PositionDeltaUnit
	dy := (m.Y - p.y) * packet.PositionDeltaUnit
	dz := (m.Z - p.z) * packet.PositionDeltaUnit
	posMoved := dx != 0 || dy != 0 || dz != 0
	yaw, pitch := degToAngle(m.Yaw), degToAngle(m.Pitch)
	rotated := yaw != p.yaw || pitch != p.pitch
	if !posMoved && !rotated {
		return
	}

	const limit = 1 << 15 // int16 bound on a delta
	if posMoved && (dx <= -limit || dx >= limit || dy <= -limit || dy >= limit || dz <= -limit || dz >= limit) {
		p.x, p.y, p.z, p.yaw, p.pitch = m.X, m.Y, m.Z, yaw, pitch
		broadcastExcept(players, p.eid, encode(&packet.RemoveEntities{EntityIDs: []int32{p.eid}}))
		broadcastExcept(players, p.eid, encode(spawn(p)))
		return
	}

	ddx, ddy, ddz := int16(dx), int16(dy), int16(dz)
	var body []byte
	switch {
	case posMoved && rotated:
		body = encode(&packet.MoveEntityPosRot{EntityID: p.eid, DX: ddx, DY: ddy, DZ: ddz, Yaw: yaw, Pitch: pitch, OnGround: m.OnGround})
	case posMoved:
		body = encode(&packet.MoveEntityPos{EntityID: p.eid, DX: ddx, DY: ddy, DZ: ddz, OnGround: m.OnGround})
	default:
		body = encode(&packet.MoveEntityRot{EntityID: p.eid, Yaw: yaw, Pitch: pitch, OnGround: m.OnGround})
	}
	// Advance the known position by the rounded delta so it tracks exactly what
	// the clients accumulated (no drift).
	p.x += float64(ddx) / packet.PositionDeltaUnit
	p.y += float64(ddy) / packet.PositionDeltaUnit
	p.z += float64(ddz) / packet.PositionDeltaUnit
	p.yaw, p.pitch = yaw, pitch

	broadcastExcept(players, p.eid, body)
	if rotated {
		broadcastExcept(players, p.eid, encode(&packet.RotateHead{EntityID: p.eid, HeadYaw: yaw}))
	}
}

// broadcastExcept enqueues a packet to every player but one (the mover).
func broadcastExcept(players map[int32]*player, except int32, body []byte) {
	for _, other := range players {
		if other.eid != except {
			enqueue(other.out, body)
		}
	}
}

// degToAngle converts degrees to the protocol's 1-byte angle (1/256 of a turn).
func degToAngle(deg float32) byte { return byte(int32(deg/360.0*256.0) & 0xFF) }

func encode(p packet.Encoder) []byte {
	w := codec.NewWriter()
	w.VarInt(p.ID())
	p.Encode(w)
	return w.Bytes()
}

// enqueue delivers a packet to a session's outbound channel without blocking; if
// the channel is full the packet is dropped (a too-slow client falls behind
// rather than stalling the hub).
func enqueue(out chan<- []byte, body []byte) {
	select {
	case out <- body:
	default:
	}
}
