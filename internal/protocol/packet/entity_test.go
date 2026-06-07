package packet

import (
	"testing"

	"github.com/Relixik/gomc/internal/protocol/codec"
)

// TestPlayerInfoUpdateEncode checks the add_player + update_listed layout
// against the format captured from a vanilla 26.1.2 server: a 1-byte actions
// EnumSet, a player count, then per player UUID + name + properties + listed.
func TestPlayerInfoUpdateEncode(t *testing.T) {
	uuid := codec.UUID{0x3f, 0x0c, 0xa0, 0xc8}
	p := &PlayerInfoUpdate{Players: []PlayerInfoEntry{
		{UUID: uuid, Name: "Probe", Properties: nil, Listed: true},
	}}
	if p.ID() != 0x46 {
		t.Errorf("id = %#x, want 0x46", p.ID())
	}
	w := codec.NewWriter()
	p.Encode(w)
	r := codec.NewReader(w.Bytes())
	if act := r.UByte(); act != 0x09 { // add_player(0x01) | update_listed(0x08)
		t.Fatalf("actions = %#x, want 0x09", act)
	}
	if n := r.VarInt(); n != 1 {
		t.Fatalf("player count = %d, want 1", n)
	}
	if got := r.UUID(); got != uuid {
		t.Errorf("uuid = %s", got)
	}
	if name := r.String(16); name != "Probe" {
		t.Errorf("name = %q", name)
	}
	if np := r.VarInt(); np != 0 {
		t.Errorf("properties = %d, want 0", np)
	}
	if !r.Bool() {
		t.Error("listed = false, want true")
	}
	if r.Err() != nil || r.Remaining() != 0 {
		t.Errorf("err=%v remaining=%d", r.Err(), r.Remaining())
	}
}

func TestPlayerInfoUpdateWithProperties(t *testing.T) {
	p := &PlayerInfoUpdate{Players: []PlayerInfoEntry{{
		UUID:       codec.UUID{1},
		Name:       "Notch",
		Properties: []LoginProperty{{Name: "textures", Value: "abc", Signature: "sig"}},
		Listed:     true,
	}}}
	w := codec.NewWriter()
	p.Encode(w)
	r := codec.NewReader(w.Bytes())
	r.UByte()    // actions
	r.VarInt()   // count
	r.UUID()     // uuid
	r.String(16) // name
	if np := r.VarInt(); np != 1 {
		t.Fatalf("properties = %d, want 1", np)
	}
	if r.String(0) != "textures" || r.String(0) != "abc" {
		t.Error("property name/value mismatch")
	}
	if !r.Bool() { // has signature
		t.Error("hasSignature = false, want true")
	}
	if r.String(0) != "sig" {
		t.Error("signature mismatch")
	}
	if !r.Bool() { // listed
		t.Error("listed = false")
	}
	if r.Err() != nil || r.Remaining() != 0 {
		t.Errorf("err=%v remaining=%d", r.Err(), r.Remaining())
	}
}

func TestAddEntityEncode(t *testing.T) {
	p := &AddEntity{EntityID: 42, UUID: codec.UUID{9}, Type: PlayerEntityType, X: 1, Y: -60, Z: 2, Yaw: 64, HeadYaw: 64}
	if p.ID() != 0x01 {
		t.Errorf("id = %#x, want 0x01", p.ID())
	}
	w := codec.NewWriter()
	p.Encode(w)
	// Byte-exact vanilla 26.1.2 player spawn: VarInt eid(1) + uuid(16) +
	// VarInt type 155(2) + 3 double(24) + 3 angle(3) + VarInt data(1) + 1 = 48.
	if n := len(w.Bytes()); n != 48 {
		t.Fatalf("encoded length = %d, want 48 (vanilla)", n)
	}
	r := codec.NewReader(w.Bytes())
	if eid := r.VarInt(); eid != 42 {
		t.Errorf("entity id = %d", eid)
	}
	r.UUID()
	if typ := r.VarInt(); typ != PlayerEntityType {
		t.Errorf("type = %d, want %d", typ, PlayerEntityType)
	}
	if x, y, z := r.Double(), r.Double(), r.Double(); x != 1 || y != -60 || z != 2 {
		t.Errorf("pos = %v,%v,%v", x, y, z)
	}
	r.Angle()                        // pitch
	if vel := r.VarInt(); vel != 0 { // optional velocity absent (0x00), between pitch and yaw
		t.Errorf("velocity = %d, want 0", vel)
	}
	if yaw := r.Angle(); yaw != 64 {
		t.Errorf("yaw = %d", yaw)
	}
	if head := r.Angle(); head != 64 {
		t.Errorf("head yaw = %d", head)
	}
	if data := r.VarInt(); data != 0 {
		t.Errorf("data = %d", data)
	}
	if r.Err() != nil || r.Remaining() != 0 {
		t.Fatalf("decode err=%v remaining=%d", r.Err(), r.Remaining())
	}
}

func TestMoveEntityPacketsEncode(t *testing.T) {
	posrot := &MoveEntityPosRot{EntityID: 42, DX: 4096, DY: -2048, DZ: 1, Yaw: 64, Pitch: 200, OnGround: true}
	if posrot.ID() != 0x36 {
		t.Errorf("pos_rot id = %#x, want 0x36", posrot.ID())
	}
	w := codec.NewWriter()
	posrot.Encode(w)
	r := codec.NewReader(w.Bytes())
	if eid := r.VarInt(); eid != 42 {
		t.Errorf("eid = %d", eid)
	}
	if dx, dy, dz := r.Short(), r.Short(), r.Short(); dx != 4096 || dy != -2048 || dz != 1 {
		t.Errorf("delta = %d,%d,%d", dx, dy, dz)
	}
	if yaw, pitch := r.Angle(), r.Angle(); yaw != 64 || pitch != 200 {
		t.Errorf("rot = %d,%d", yaw, pitch)
	}
	if !r.Bool() {
		t.Error("onGround = false")
	}
	if r.Err() != nil || r.Remaining() != 0 {
		t.Errorf("err=%v remaining=%d", r.Err(), r.Remaining())
	}

	pos := &MoveEntityPos{EntityID: 1, DX: 10, DY: 20, DZ: 30}
	if pos.ID() != 0x35 {
		t.Errorf("pos id = %#x, want 0x35", pos.ID())
	}
	rot := &MoveEntityRot{EntityID: 1, Yaw: 128, Pitch: 0}
	if rot.ID() != 0x38 {
		t.Errorf("rot id = %#x, want 0x38", rot.ID())
	}
	head := &RotateHead{EntityID: 7, HeadYaw: 64}
	if head.ID() != 0x53 {
		t.Errorf("head id = %#x, want 0x53", head.ID())
	}
	w = codec.NewWriter()
	head.Encode(w)
	r = codec.NewReader(w.Bytes())
	if eid, hy := r.VarInt(), r.Angle(); eid != 7 || hy != 64 {
		t.Errorf("head = eid %d yaw %d", eid, hy)
	}
}

func TestRemoveEntitiesAndInfoRemove(t *testing.T) {
	re := &RemoveEntities{EntityIDs: []int32{42, 7}}
	if re.ID() != 0x4D {
		t.Errorf("remove entities id = %#x", re.ID())
	}
	w := codec.NewWriter()
	re.Encode(w)
	r := codec.NewReader(w.Bytes())
	if n := r.VarInt(); n != 2 {
		t.Fatalf("count = %d", n)
	}
	if a, b := r.VarInt(), r.VarInt(); a != 42 || b != 7 {
		t.Errorf("ids = %d,%d", a, b)
	}

	pir := &PlayerInfoRemove{UUIDs: []codec.UUID{{1}, {2}}}
	if pir.ID() != 0x45 {
		t.Errorf("info remove id = %#x", pir.ID())
	}
	w = codec.NewWriter()
	pir.Encode(w)
	r = codec.NewReader(w.Bytes())
	if n := r.VarInt(); n != 2 {
		t.Fatalf("uuid count = %d", n)
	}
}
