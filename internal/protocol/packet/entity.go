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
type AddEntity struct {
	EntityID            int32
	UUID                codec.UUID
	Type                int32
	X, Y, Z             float64
	Pitch, Yaw, HeadYaw byte
	Data                int32
	VelX, VelY, VelZ    int16
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
	w.Angle(p.HeadYaw)
	w.VarInt(p.Data)
	w.Short(p.VelX)
	w.Short(p.VelY)
	w.Short(p.VelZ)
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
