package server

import (
	"net"
	"strconv"
)

// Config holds the top-level server settings.
//
// It will grow (server.properties-like, parsed with bufio — stdlib only) as
// milestones land. For M0 it is just the bind address.
type Config struct {
	Host string
	Port int

	// OnlineMode toggles Mojang session authentication + encryption.
	// M1 runs offline (false); online mode is wired up in M2.
	OnlineMode bool

	// CompressionThreshold sets the zlib threshold sent in Set Compression:
	// a value > 0 compresses packet bodies of at least that many bytes, a
	// negative value disables compression, and 0 selects the vanilla default.
	CompressionThreshold int

	// MaxPlayers is advertised in the status (server list) response.
	MaxPlayers int

	// MOTD is the "message of the day" shown in the multiplayer list.
	MOTD string

	// ViewDistance in chunks (advertised in Login(play)).
	ViewDistance int
}

// defaultCompressionThreshold matches the vanilla network-compression-threshold.
const defaultCompressionThreshold = 256

// Addr returns the host:port string for net.Listen.
func (c Config) Addr() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

// compressionThreshold resolves the configured threshold, mapping the zero value
// to the vanilla default so a zero-value Config still compresses sensibly.
func (c Config) compressionThreshold() int {
	if c.CompressionThreshold == 0 {
		return defaultCompressionThreshold
	}
	return c.CompressionThreshold
}
