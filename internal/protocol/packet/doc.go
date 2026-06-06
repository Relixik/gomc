// Package packet defines every protocol packet and the registry that maps a
// (State, Direction, ID) triple to a packet factory.
//
// The same ID byte means different packets depending on connection state and
// direction, so dispatch MUST key on all three — never on the ID alone.
//
// Layout: one file per state per direction (clientbound_play.go,
// serverbound_login.go, ...). Each packet is a struct implementing
// Encode(*codec.Writer) and/or Decode(*codec.Reader). Packet IDs are declared
// as named consts at the top of each file, each with a comment citing the
// protocol version it was verified against, e.g.:
//
//	const (
//	    // Verified vs deobf 26.1.2 server jar (2026-xx-xx).
//	    idKeepAlive = 0x1C // serverbound, Play
//	)
//
// This keeps the per-version ID churn (protocol 775 shifted all Play IDs) a
// localized, test-guarded edit instead of a regeneration. A golden-bytes test
// table pins the wire format of each packet.
//
// WARNING: Play-state IDs for 775 are NOT authoritatively documented yet
// (minecraft.wiki shows 773; minecraft-data's 26.1.2 Play tables are known
// buggy). Byte-verify every Play ID against a real 26.1.2 artifact. See PLAN.md
// risk #1.
//
// Stdlib only; builds on codec, nbt, text.
package packet
