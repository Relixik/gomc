// Package session owns a single connection's lifecycle and state machine.
//
// It holds the FrameConn, the current protocol State, the GameProfile, and runs
// two goroutines per connection: a blocking READ loop, and a WRITE loop that
// drains a bounded outbound channel (so a slow socket never blocks the
// authoritative tick loop — overflow => drop + disconnect).
//
// Handlers for Handshaking / Status / Login / Configuration run INLINE on the
// connection goroutine (pure request/response, no shared world state). On the
// transition to Play, the session registers a Player with the central game loop
// (internal/game/loop) and from then on converts inbound Play packets into
// intents pushed onto the loop's inbound channel — it never mutates world state
// directly.
//
// Connection lifetime is owned by a context.Context: on read error/close it
// cancels the context, drains+closes the outbound channel, and posts one
// removal intent to the game loop (the clean-teardown lesson carried over from
// the old code's goroutine-leak fix).
//
// Stdlib only: net, context, log/slog, time.
package session
