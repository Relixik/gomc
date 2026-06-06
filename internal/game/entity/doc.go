// Package entity holds the entity and player model: position, rotation, entity
// ID (EID) allocation, and — from M5 — entity metadata. Players are spawned to
// other clients via Spawn Entity in modern protocol versions.
//
// Mutated only on the tick goroutine (internal/game/loop).
//
// Stdlib only.
package entity
