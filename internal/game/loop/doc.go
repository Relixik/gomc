// Package loop is the single authoritative game loop, running a fixed 20 TPS
// tick (a 50ms ticker with a lag accumulator — never trust per-tick
// wall-clock).
//
// It is the ONLY goroutine that mutates world state, so cross-goroutine access
// is via channels rather than locks: connections push (PlayerID, intent) onto a
// single bounded inbound channel; the loop pushes per-player outbound packets
// by enqueueing onto each Player's outbound channel. Broadcasts iterate players
// on the tick goroutine. The loop also drives Keep-Alive (in Configuration and
// Play), the synchronous in-process event bus (handlers run on the tick
// goroutine, can cancel events), and command dispatch.
//
// Stdlib only: time, context, sync (only at the player-registration boundary).
package loop
