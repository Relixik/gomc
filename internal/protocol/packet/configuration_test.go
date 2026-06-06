package packet

import (
	"bytes"
	"testing"

	"github.com/Relixik/gomc/internal/protocol/codec"
)

func TestClientInformationDecode(t *testing.T) {
	w := codec.NewWriter()
	w.String("en_us")
	w.Byte(12)
	w.VarInt(0) // chat mode: enabled
	w.Bool(true)
	w.UByte(0x7F)
	w.VarInt(1) // main hand: right
	w.Bool(false)
	w.Bool(true)
	w.VarInt(0) // particle status: all

	r := codec.NewReader(w.Bytes())
	var ci ClientInformation
	ci.Decode(r)
	if r.Err() != nil {
		t.Fatalf("decode: %v", r.Err())
	}
	if ci.Locale != "en_us" || ci.ViewDistance != 12 || ci.MainHand != 1 ||
		!ci.ChatColors || ci.DisplayedSkinParts != 0x7F || !ci.AllowServerListings {
		t.Errorf("decoded %+v", ci)
	}
	if r.Remaining() != 0 {
		t.Errorf("trailing bytes: %d", r.Remaining())
	}
}

func TestKnownPacksRoundTrip(t *testing.T) {
	out := &ClientboundKnownPacks{Packs: []KnownPack{{"minecraft", "core", "26.1.2"}}}
	w := codec.NewWriter()
	out.Encode(w)

	// Decode with the serverbound form (same wire layout).
	r := codec.NewReader(w.Bytes())
	var in KnownPacksServerbound
	in.Decode(r)
	if r.Err() != nil {
		t.Fatalf("decode: %v", r.Err())
	}
	if len(in.Packs) != 1 || in.Packs[0] != (KnownPack{"minecraft", "core", "26.1.2"}) {
		t.Errorf("packs = %+v", in.Packs)
	}
}

func TestRegistryDataEncodeNoNBT(t *testing.T) {
	rd := &RegistryData{
		Registry: "minecraft:dimension_type",
		Entries: []RegistryEntry{
			{ID: "minecraft:overworld"},  // has_data=false
			{ID: "minecraft:the_nether"}, // has_data=false
		},
	}
	w := codec.NewWriter()
	rd.Encode(w)

	r := codec.NewReader(w.Bytes())
	if reg := r.Identifier(); reg != "minecraft:dimension_type" {
		t.Errorf("registry = %q", reg)
	}
	if n := r.VarInt(); n != 2 {
		t.Fatalf("entry count = %d", n)
	}
	if id := r.Identifier(); id != "minecraft:overworld" {
		t.Errorf("entry 0 id = %q", id)
	}
	if has := r.Bool(); has {
		t.Error("entry 0 should have has_data=false")
	}
	if id := r.Identifier(); id != "minecraft:the_nether" {
		t.Errorf("entry 1 id = %q", id)
	}
	if has := r.Bool(); has {
		t.Error("entry 1 should have has_data=false")
	}
	if r.Err() != nil || r.Remaining() != 0 {
		t.Errorf("err=%v remaining=%d", r.Err(), r.Remaining())
	}
}

func TestFeatureFlagsEncode(t *testing.T) {
	w := codec.NewWriter()
	(&FeatureFlags{Flags: []string{"minecraft:vanilla"}}).Encode(w)
	r := codec.NewReader(w.Bytes())
	if n := r.VarInt(); n != 1 {
		t.Fatalf("flag count = %d", n)
	}
	if f := r.Identifier(); f != "minecraft:vanilla" {
		t.Errorf("flag = %q", f)
	}
}

func TestConfigRegistryLookup(t *testing.T) {
	for _, id := range []int32{idCfgClientInformation, idCfgPluginMessageSb, idCfgAckFinish, idCfgKeepAliveSb, idCfgKnownPacksSb} {
		if _, ok := NewServerbound(StateConfiguration, id); !ok {
			t.Errorf("configuration serverbound %#x not registered", id)
		}
	}
}

func TestPluginMessageServerboundDecode(t *testing.T) {
	w := codec.NewWriter()
	w.Identifier("minecraft:brand")
	w.Raw([]byte{0x05, 'v', 'a', 'n', 'i', 'l'})
	r := codec.NewReader(w.Bytes())
	var pm PluginMessageServerbound
	pm.Decode(r)
	if pm.Channel != "minecraft:brand" || !bytes.Equal(pm.Data, []byte{0x05, 'v', 'a', 'n', 'i', 'l'}) {
		t.Errorf("decoded channel=%q data=% x", pm.Channel, pm.Data)
	}
}
