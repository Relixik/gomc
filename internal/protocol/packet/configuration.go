package packet

import "github.com/Relixik/gomc/internal/protocol/codec"

// Configuration-state packet ids — VERIFIED against the 26.1.2 (protocol 775)
// data generator's packets.json (vanilla/Packet Report). select_known_packs is
// the wire name for "known packs"; update_enabled_features for "feature flags".
const (
	// Serverbound
	idCfgClientInformation = 0x00
	idCfgPluginMessageSb   = 0x02
	idCfgAckFinish         = 0x03
	idCfgKeepAliveSb       = 0x04
	idCfgKnownPacksSb      = 0x07
	// Clientbound
	idCfgPluginMessageCb = 0x01
	idCfgFinish          = 0x03
	idCfgKeepAliveCb     = 0x04
	idCfgRegistryData    = 0x07
	idCfgFeatureFlags    = 0x0C
	idCfgUpdateTags      = 0x0D
	idCfgKnownPacksCb    = 0x0E
)

func init() {
	registerServerbound(StateConfiguration, idCfgClientInformation, func() Decoder { return &ClientInformation{} })
	registerServerbound(StateConfiguration, idCfgPluginMessageSb, func() Decoder { return &PluginMessageServerbound{} })
	registerServerbound(StateConfiguration, idCfgAckFinish, func() Decoder { return &AckFinishConfiguration{} })
	registerServerbound(StateConfiguration, idCfgKeepAliveSb, func() Decoder { return &KeepAliveServerbound{} })
	registerServerbound(StateConfiguration, idCfgKnownPacksSb, func() Decoder { return &KnownPacksServerbound{} })
}

// KnownPack identifies a data/resource pack both sides may share. When a pack is
// known to both, Registry Data entries from it can be sent without inline NBT.
type KnownPack struct {
	Namespace string
	ID        string
	Version   string
}

// ---- Serverbound ----

// ClientInformation carries the client's settings. (Configuration, sb, 0x00.)
type ClientInformation struct {
	Locale              string
	ViewDistance        int8
	ChatMode            int32
	ChatColors          bool
	DisplayedSkinParts  byte
	MainHand            int32
	EnableTextFiltering bool
	AllowServerListings bool
	ParticleStatus      int32 // added 1.21.2
}

func (p *ClientInformation) Decode(r *codec.Reader) {
	p.Locale = r.String(16)
	p.ViewDistance = r.Byte()
	p.ChatMode = r.VarInt()
	p.ChatColors = r.Bool()
	p.DisplayedSkinParts = r.UByte()
	p.MainHand = r.VarInt()
	p.EnableTextFiltering = r.Bool()
	p.AllowServerListings = r.Bool()
	p.ParticleStatus = r.VarInt()
}

// PluginMessageServerbound is a custom-channel message from the client (e.g.
// "minecraft:brand"). (Configuration, sb, 0x02.)
type PluginMessageServerbound struct {
	Channel string
	Data    []byte
}

func (p *PluginMessageServerbound) Decode(r *codec.Reader) {
	p.Channel = r.Identifier()
	p.Data = r.RemainingBytes()
}

// AckFinishConfiguration moves the connection from Configuration to Play.
// (Configuration, sb, 0x03.)
type AckFinishConfiguration struct{}

func (p *AckFinishConfiguration) Decode(*codec.Reader) {}

// KeepAliveServerbound echoes a clientbound keep-alive id. (Configuration, sb,
// 0x04 — also used in Play with a different id.)
type KeepAliveServerbound struct{ ID int64 }

func (p *KeepAliveServerbound) Decode(r *codec.Reader) { p.ID = r.Long() }

// KnownPacksServerbound is the client's list of known packs. (Configuration, sb,
// 0x07.)
type KnownPacksServerbound struct{ Packs []KnownPack }

func (p *KnownPacksServerbound) Decode(r *codec.Reader) {
	n := r.VarInt()
	if r.Err() != nil || n < 0 {
		return
	}
	p.Packs = make([]KnownPack, 0, min(int(n), 64))
	for i := int32(0); i < n; i++ {
		p.Packs = append(p.Packs, KnownPack{
			Namespace: r.Identifier(),
			ID:        r.String(0),
			Version:   r.String(0),
		})
		if r.Err() != nil {
			return
		}
	}
}

// ---- Clientbound ----

// PluginMessageClientbound sends a custom-channel message (e.g. the server
// brand). (Configuration, cb, 0x01.)
type PluginMessageClientbound struct {
	Channel string
	Data    []byte
}

func (p *PluginMessageClientbound) ID() int32 { return idCfgPluginMessageCb }
func (p *PluginMessageClientbound) Encode(w *codec.Writer) {
	w.Identifier(p.Channel)
	w.Raw(p.Data)
}

// FinishConfiguration tells the client configuration is done. (Configuration,
// cb, 0x03.)
type FinishConfiguration struct{}

func (p *FinishConfiguration) ID() int32            { return idCfgFinish }
func (p *FinishConfiguration) Encode(*codec.Writer) {}

// KeepAliveClientbound carries an id the client must echo. (Configuration, cb,
// 0x04.)
type KeepAliveClientbound struct{ KeepAliveID int64 }

func (p *KeepAliveClientbound) ID() int32              { return idCfgKeepAliveCb }
func (p *KeepAliveClientbound) Encode(w *codec.Writer) { w.Long(p.KeepAliveID) }

// FeatureFlags enables datapack feature flags (at minimum "minecraft:vanilla").
// (Configuration, cb, 0x0C.)
type FeatureFlags struct{ Flags []string }

func (p *FeatureFlags) ID() int32 { return idCfgFeatureFlags }
func (p *FeatureFlags) Encode(w *codec.Writer) {
	w.VarInt(int32(len(p.Flags)))
	for _, f := range p.Flags {
		w.Identifier(f)
	}
}

// ClientboundKnownPacks advertises the packs the server knows. (Configuration,
// cb, 0x0E.)
type ClientboundKnownPacks struct{ Packs []KnownPack }

func (p *ClientboundKnownPacks) ID() int32 { return idCfgKnownPacksCb }
func (p *ClientboundKnownPacks) Encode(w *codec.Writer) {
	w.VarInt(int32(len(p.Packs)))
	for _, pk := range p.Packs {
		w.String(pk.Namespace)
		w.String(pk.ID)
		w.String(pk.Version)
	}
}

// RegistryEntry is one entry of a Registry Data packet. When Data is nil the
// entry is sent without inline NBT (has_data=false), relying on a shared known
// pack; otherwise Data is the entry's network NBT (TODO: nbt.Tag wiring).
type RegistryEntry struct {
	ID   string
	Data []byte // pre-encoded network NBT, or nil for has_data=false
}

// RegistryData sends one registry and its ordered entries. (Configuration, cb,
// 0x07.) Entry order defines the numeric ids referenced by later packets.
type RegistryData struct {
	Registry string
	Entries  []RegistryEntry
}

func (p *RegistryData) ID() int32 { return idCfgRegistryData }
func (p *RegistryData) Encode(w *codec.Writer) {
	w.Identifier(p.Registry)
	w.VarInt(int32(len(p.Entries)))
	for _, e := range p.Entries {
		w.Identifier(e.ID)
		if e.Data != nil {
			w.Bool(true)
			w.Raw(e.Data)
		} else {
			w.Bool(false)
		}
	}
}
