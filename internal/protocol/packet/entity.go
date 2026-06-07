package packet

import "github.com/Relixik/gomc/internal/protocol/codec"

// PlayerEntityType is the entity-type registry id of minecraft:player in 26.1.2
// (from the data generator). Players are spawned with the generic Add Entity
// packet using this type since the dedicated Spawn Player packet was removed.
const PlayerEntityType = 155

// Player Info Update action bits — an 8-action EnumSet serialised as one byte in
// 26.1.2 (verified against a capture; list_priority and hat are the newer bits).
const (
	piAddPlayer    = 0x01
	piUpdateListed = 0x08
)

// PlayerInfoEntry is one player added to the client's player list.
type PlayerInfoEntry struct {
	UUID       codec.UUID
	Name       string
	Properties []LoginProperty // notably "textures" — drives the skin
	Listed     bool            // shown in the tab list
}

// PlayerInfoUpdate adds players to the client's player list with their profile
// (so skins render and the tab list is populated). It sends only the add_player
// and update_listed actions. (Play, cb, 0x46.)
type PlayerInfoUpdate struct {
	Players []PlayerInfoEntry
}

func (p *PlayerInfoUpdate) ID() int32 { return idPlayInfoUpdate }

func (p *PlayerInfoUpdate) Encode(w *codec.Writer) {
	w.UByte(piAddPlayer | piUpdateListed) // actions EnumSet
	w.VarInt(int32(len(p.Players)))
	for _, e := range p.Players {
		w.UUID(e.UUID)
		// add_player: name + properties.
		w.String(e.Name)
		w.VarInt(int32(len(e.Properties)))
		for _, prop := range e.Properties {
			w.String(prop.Name)
			w.String(prop.Value)
			if prop.Signature != "" {
				w.Bool(true)
				w.String(prop.Signature)
			} else {
				w.Bool(false)
			}
		}
		// update_listed: tab-list visibility.
		w.Bool(e.Listed)
	}
}

// PlayerInfoRemove drops players from the client's player list. (Play, cb, 0x45.)
type PlayerInfoRemove struct {
	UUIDs []codec.UUID
}

func (p *PlayerInfoRemove) ID() int32 { return idPlayInfoRemove }

func (p *PlayerInfoRemove) Encode(w *codec.Writer) {
	w.VarInt(int32(len(p.UUIDs)))
	for _, u := range p.UUIDs {
		w.UUID(u)
	}
}

// AddEntity spawns an entity. For another player, Type is PlayerEntityType and
// the client renders it using the profile sent earlier in PlayerInfoUpdate.
// Angles are 1 byte = 1/256 turn. (Play, cb, 0x01.)
//
// The 26.1.2 layout was nailed against vanilla captures (a stationary player at
// 5 trailing bytes, and moving slimes at 10) plus client crash reports: pitch,
// yaw, then an OPTIONAL velocity (a low-precision Vec3 "LpVec3" — absent is a
// single 0x00 byte, present is a flag byte plus a packed uint), then head yaw,
// then data. gomc only spawns motionless players, so the velocity is always the
// absent marker 0x00 (right AFTER yaw); a stationary spawn is byte-for-byte
// vanilla. The earlier crashes came from emitting that 0x00 before yaw, so a
// rotated player's non-zero yaw landed where the client expects the optional
// marker and it tried to read a 4-byte velocity past the packet end.
type AddEntity struct {
	EntityID            int32
	UUID                codec.UUID
	Type                int32
	X, Y, Z             float64
	Pitch, Yaw, HeadYaw byte
	Data                int32
}

func (p *AddEntity) ID() int32 { return idPlayAddEntity }

func (p *AddEntity) Encode(w *codec.Writer) {
	w.VarInt(p.EntityID)
	w.UUID(p.UUID)
	w.VarInt(p.Type)
	w.Double(p.X)
	w.Double(p.Y)
	w.Double(p.Z)
	w.Angle(p.Pitch)
	w.Angle(p.Yaw)
	w.VarInt(0) // velocity: Optional<LpVec3> absent (0x00), after yaw
	w.Angle(p.HeadYaw)
	w.VarInt(p.Data)
}

// RemoveEntities despawns entities by id. (Play, cb, 0x4D.)
type RemoveEntities struct {
	EntityIDs []int32
}

func (p *RemoveEntities) ID() int32 { return idPlayRemoveEntities }

func (p *RemoveEntities) Encode(w *codec.Writer) {
	w.VarInt(int32(len(p.EntityIDs)))
	for _, id := range p.EntityIDs {
		w.VarInt(id)
	}
}

// Position deltas are encoded as (newBlock - oldBlock) * 4096 in a Short, so a
// single update covers at most ±8 blocks per axis.
const PositionDeltaUnit = 4096

// MoveEntityPos moves an entity by a position delta (no rotation change).
// (Play, cb, 0x35.)
type MoveEntityPos struct {
	EntityID   int32
	DX, DY, DZ int16
	OnGround   bool
}

func (p *MoveEntityPos) ID() int32 { return idPlayMoveEntityPos }

func (p *MoveEntityPos) Encode(w *codec.Writer) {
	w.VarInt(p.EntityID)
	w.Short(p.DX)
	w.Short(p.DY)
	w.Short(p.DZ)
	w.Bool(p.OnGround)
}

// MoveEntityPosRot moves an entity by a position delta and sets its body
// rotation. (Play, cb, 0x36.)
type MoveEntityPosRot struct {
	EntityID   int32
	DX, DY, DZ int16
	Yaw, Pitch byte
	OnGround   bool
}

func (p *MoveEntityPosRot) ID() int32 { return idPlayMoveEntityPosRot }

func (p *MoveEntityPosRot) Encode(w *codec.Writer) {
	w.VarInt(p.EntityID)
	w.Short(p.DX)
	w.Short(p.DY)
	w.Short(p.DZ)
	w.Angle(p.Yaw)
	w.Angle(p.Pitch)
	w.Bool(p.OnGround)
}

// MoveEntityRot sets an entity's body rotation (no position change).
// (Play, cb, 0x38.)
type MoveEntityRot struct {
	EntityID   int32
	Yaw, Pitch byte
	OnGround   bool
}

func (p *MoveEntityRot) ID() int32 { return idPlayMoveEntityRot }

func (p *MoveEntityRot) Encode(w *codec.Writer) {
	w.VarInt(p.EntityID)
	w.Angle(p.Yaw)
	w.Angle(p.Pitch)
	w.Bool(p.OnGround)
}

// RotateHead sets an entity's head yaw (independent of body rotation).
// (Play, cb, 0x53.)
type RotateHead struct {
	EntityID int32
	HeadYaw  byte
}

func (p *RotateHead) ID() int32 { return idPlayRotateHead }

func (p *RotateHead) Encode(w *codec.Writer) {
	w.VarInt(p.EntityID)
	w.Angle(p.HeadYaw)
}
