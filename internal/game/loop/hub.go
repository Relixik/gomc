package loop

import (
	"context"
	"log/slog"
	"sync/atomic"

	"github.com/Relixik/gomc/internal/protocol/codec"
	"github.com/Relixik/gomc/internal/protocol/packet"
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
	leave   chan int32
	logger  *slog.Logger
}

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

// MoveRequest updates a player's position/rotation (broadcast lands in M4b).
type MoveRequest struct {
	EntityID   int32
	X, Y, Z    float64
	Yaw, Pitch float32
}

// New creates an unstarted hub. Call Run in its own goroutine.
func New(logger *slog.Logger) *Hub {
	if logger == nil {
		logger = slog.Default()
	}
	return &Hub{
		join:   make(chan JoinRequest, 32),
		move:   make(chan MoveRequest, 512),
		leave:  make(chan int32, 32),
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
			if p, ok := players[m.EntityID]; ok {
				p.X, p.Y, p.Z, p.Yaw, p.Pitch = m.X, m.Y, m.Z, m.Yaw, m.Pitch
			}
		}
	}
}

type player struct {
	eid        int32
	uuid       codec.UUID
	name       string
	props      []packet.LoginProperty
	X, Y, Z    float64
	Yaw, Pitch float32
	out        chan<- []byte
}

func (h *Hub) onJoin(players map[int32]*player, r JoinRequest) {
	p := &player{eid: r.EntityID, uuid: r.UUID, name: r.Name, props: r.Properties, X: r.X, Y: r.Y, Z: r.Z, Yaw: r.Yaw, Pitch: r.Pitch, out: r.Out}

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
		X: p.X, Y: p.Y, Z: p.Z,
		Yaw: degToAngle(p.Yaw), Pitch: degToAngle(p.Pitch), HeadYaw: degToAngle(p.Yaw),
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
