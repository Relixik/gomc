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

	// MaxPlayers is advertised in the status (server list) response.
	MaxPlayers int

	// MOTD is the "message of the day" shown in the multiplayer list.
	MOTD string

	// ViewDistance in chunks (advertised in Login(play)).
	ViewDistance int
}

// Addr returns the host:port string for net.Listen.
func (c Config) Addr() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}
