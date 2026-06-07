package packet

import "github.com/Relixik/gomc/internal/protocol/codec"

// Play-state packet ids — VERIFIED against the 26.1.2 (protocol 775) data
// generator packets.json. NOTE: protocol 775 shifted all Play ids vs 773.
const (
	// Clientbound
	idPlayLogin            = 0x31
	idPlayGameEvent        = 0x26
	idPlaySetCenterChunk   = 0x5E
	idPlayChunkData        = 0x2D
	idPlayUnloadChunk      = 0x25
	idPlaySyncPosition     = 0x48
	idPlayKeepAliveCb      = 0x2C
	idPlayAddEntity        = 0x01
	idPlayRemoveEntities   = 0x4D
	idPlayInfoUpdate       = 0x46
	idPlayInfoRemove       = 0x45
	idPlayMoveEntityPos    = 0x35
	idPlayMoveEntityPosRot = 0x36
	idPlayMoveEntityRot    = 0x38
	idPlayRotateHead       = 0x53
	idPlaySystemChat       = 0x79
	idPlayBlockUpdate      = 0x08
	idPlayBlockChangedAck  = 0x04
	// Serverbound
	idPlayConfirmTeleport = 0x00
	idPlayChat            = 0x09
	idPlayClientInfo      = 0x0E
	idPlayKeepAliveSb     = 0x1C
	idPlayMovePos         = 0x1E
	idPlayMovePosRot      = 0x1F
	idPlayMoveRot         = 0x20
	idPlayPlayerLoaded    = 0x2C
	idPlayPlayerAction    = 0x29
	idPlaySetCarriedItem  = 0x35
	idPlaySetCreativeSlot = 0x38
	idPlayUseItemOn       = 0x42
)

func init() {
	registerServerbound(StatePlay, idPlayPlayerAction, func() Decoder { return &PlayerAction{} })
	registerServerbound(StatePlay, idPlayUseItemOn, func() Decoder { return &UseItemOn{} })
	registerServerbound(StatePlay, idPlaySetCreativeSlot, func() Decoder { return &SetCreativeModeSlot{} })
	registerServerbound(StatePlay, idPlaySetCarriedItem, func() Decoder { return &SetCarriedItem{} })
}

func init() {
	registerServerbound(StatePlay, idPlayConfirmTeleport, func() Decoder { return &ConfirmTeleport{} })
	registerServerbound(StatePlay, idPlayClientInfo, func() Decoder { return &ClientInformation{} })
	registerServerbound(StatePlay, idPlayKeepAliveSb, func() Decoder { return &KeepAliveServerbound{} })
	registerServerbound(StatePlay, idPlayMovePos, func() Decoder { return &MovePlayerPos{} })
	registerServerbound(StatePlay, idPlayMovePosRot, func() Decoder { return &MovePlayerPosRot{} })
	registerServerbound(StatePlay, idPlayMoveRot, func() Decoder { return &MovePlayerRot{} })
	registerServerbound(StatePlay, idPlayPlayerLoaded, func() Decoder { return &PlayerLoaded{} })
	registerServerbound(StatePlay, idPlayChat, func() Decoder { return &ChatMessage{} })
}

// ---- Clientbound ----

// LoginPlay (the "Login"/Join Game packet). Field layout verified byte-for-byte
// against a captured vanilla 26.1.2 Login(play). (Play, cb, 0x31.)
type LoginPlay struct {
	EntityID            int32
	Hardcore            bool
	DimensionNames      []string
	MaxPlayers          int32
	ViewDistance        int32
	SimulationDistance  int32
	ReducedDebug        bool
	EnableRespawnScreen bool
	DoLimitedCrafting   bool
	DimensionType       int32 // id into the dimension_type registry
	DimensionName       string
	HashedSeed          int64
	GameMode            byte
	PreviousGameMode    int8
	IsDebug             bool
	IsFlat              bool
	HasDeathLocation    bool
	PortalCooldown      int32
	SeaLevel            int32
	EnforcesSecureChat  bool
}

func (p *LoginPlay) ID() int32 { return idPlayLogin }
func (p *LoginPlay) Encode(w *codec.Writer) {
	w.Int(p.EntityID)
	w.Bool(p.Hardcore)
	w.VarInt(int32(len(p.DimensionNames)))
	for _, d := range p.DimensionNames {
		w.Identifier(d)
	}
	w.VarInt(p.MaxPlayers)
	w.VarInt(p.ViewDistance)
	w.VarInt(p.SimulationDistance)
	w.Bool(p.ReducedDebug)
	w.Bool(p.EnableRespawnScreen)
	w.Bool(p.DoLimitedCrafting)
	w.VarInt(p.DimensionType)
	w.Identifier(p.DimensionName)
	w.Long(p.HashedSeed)
	w.UByte(p.GameMode)
	w.Byte(p.PreviousGameMode)
	w.Bool(p.IsDebug)
	w.Bool(p.IsFlat)
	w.Bool(p.HasDeathLocation) // false => no death dimension/location fields
	w.VarInt(p.PortalCooldown)
	w.VarInt(p.SeaLevel)
	w.Bool(p.EnforcesSecureChat)
}

// GameEvent signals a game event (e.g. event 13 = "start waiting for level
// chunks"). (Play, cb, 0x26.)
type GameEvent struct {
	Event byte
	Value float32
}

func (p *GameEvent) ID() int32 { return idPlayGameEvent }
func (p *GameEvent) Encode(w *codec.Writer) {
	w.UByte(p.Event)
	w.Float(p.Value)
}

// SetCenterChunk tells the client which chunk the player is in. (Play, cb, 0x5E.)
type SetCenterChunk struct{ ChunkX, ChunkZ int32 }

func (p *SetCenterChunk) ID() int32 { return idPlaySetCenterChunk }
func (p *SetCenterChunk) Encode(w *codec.Writer) {
	w.VarInt(p.ChunkX)
	w.VarInt(p.ChunkZ)
}

// SyncPlayerPosition teleports the player. Field layout verified against a
// captured vanilla packet (Teleport ID, position, velocity, yaw, pitch, flags).
// (Play, cb, 0x48.)
type SyncPlayerPosition struct {
	TeleportID       int32
	X, Y, Z          float64
	VelX, VelY, VelZ float64
	Yaw, Pitch       float32
	Flags            int32
}

func (p *SyncPlayerPosition) ID() int32 { return idPlaySyncPosition }
func (p *SyncPlayerPosition) Encode(w *codec.Writer) {
	w.VarInt(p.TeleportID)
	w.Double(p.X)
	w.Double(p.Y)
	w.Double(p.Z)
	w.Double(p.VelX)
	w.Double(p.VelY)
	w.Double(p.VelZ)
	w.Float(p.Yaw)
	w.Float(p.Pitch)
	w.Int(p.Flags)
}

// KeepAlivePlayClientbound carries an id the client must echo. (Play, cb, 0x2C.)
type KeepAlivePlayClientbound struct{ KeepAliveID int64 }

func (p *KeepAlivePlayClientbound) ID() int32              { return idPlayKeepAliveCb }
func (p *KeepAlivePlayClientbound) Encode(w *codec.Writer) { w.Long(p.KeepAliveID) }

// ---- Serverbound ----

// ConfirmTeleport acknowledges a SyncPlayerPosition. (Play, sb, 0x00.)
type ConfirmTeleport struct{ TeleportID int32 }

func (p *ConfirmTeleport) Decode(r *codec.Reader) { p.TeleportID = r.VarInt() }

// PlayerLoaded signals the client has finished loading into the world. (Play,
// sb, 0x2C.)
type PlayerLoaded struct{}

func (p *PlayerLoaded) Decode(*codec.Reader) {}

// MovePlayerPos is a position-only movement update. (Play, sb, 0x1E.)
type MovePlayerPos struct {
	X, Y, Z float64
	Flags   int8 // on-ground / against-wall bit flags
}

func (p *MovePlayerPos) Decode(r *codec.Reader) {
	p.X, p.Y, p.Z = r.Double(), r.Double(), r.Double()
	p.Flags = r.Byte()
}

// MovePlayerPosRot is a position + rotation movement update. (Play, sb, 0x1F.)
type MovePlayerPosRot struct {
	X, Y, Z    float64
	Yaw, Pitch float32
	Flags      int8
}

func (p *MovePlayerPosRot) Decode(r *codec.Reader) {
	p.X, p.Y, p.Z = r.Double(), r.Double(), r.Double()
	p.Yaw, p.Pitch = r.Float(), r.Float()
	p.Flags = r.Byte()
}

// MovePlayerRot is a rotation-only movement update. (Play, sb, 0x20.)
type MovePlayerRot struct {
	Yaw, Pitch float32
	Flags      int8
}

func (p *MovePlayerRot) Decode(r *codec.Reader) {
	p.Yaw, p.Pitch = r.Float(), r.Float()
	p.Flags = r.Byte()
}

// ChatMessage is a player chat message (signed fields are read but ignored for
// now). (Play, sb, 0x09.)
type ChatMessage struct {
	Message string
}

func (p *ChatMessage) Decode(r *codec.Reader) {
	p.Message = r.String(256)
	// Remaining signed-chat fields (timestamp, salt, signature, etc.) are left
	// unread; the session does not rely on them yet.
}
