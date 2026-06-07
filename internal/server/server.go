package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/Relixik/gomc/internal/game/loop"
	"github.com/Relixik/gomc/internal/net/session"
	"github.com/Relixik/gomc/internal/protocol/auth"
	"github.com/Relixik/gomc/internal/protocol/packet"
	"github.com/Relixik/gomc/internal/protocol/text"
)

// Run starts the TCP listener and serves each connection through a Session
// until ctx is cancelled. Each connection runs its own goroutine.
func Run(ctx context.Context, cfg Config) error {
	// One shared hub broadcasts player presence between all connections.
	hub := loop.New(slog.Default())
	go hub.Run(ctx)

	opts := session.Options{
		OnlineMode:           cfg.OnlineMode,
		CompressionThreshold: cfg.compressionThreshold(),
		Hub:                  hub,
	}
	if cfg.OnlineMode {
		// One RSA key pair is generated at startup and shared across connections,
		// exactly as the vanilla server does.
		kp, err := auth.GenerateKeyPair()
		if err != nil {
			return fmt.Errorf("generate server key pair: %w", err)
		}
		opts.KeyPair = kp
	}

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", cfg.Addr())
	if err != nil {
		return err
	}
	slog.Info("listening", "addr", cfg.Addr(), "protocol", packet.ProtocolVersion, "version", packet.GameVersion,
		"online", cfg.OnlineMode, "compression", opts.CompressionThreshold)

	// Close the listener when the context is cancelled so Accept unblocks.
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	status := cfg.status()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil // clean shutdown
			}
			slog.Warn("accept error", "err", err)
			continue
		}
		go session.New(conn, status, opts, slog.Default()).Serve(ctx)
	}
}

// status builds the server-list response from the config, applying defaults.
func (cfg Config) status() text.StatusResponse {
	maxPlayers := cfg.MaxPlayers
	if maxPlayers <= 0 {
		maxPlayers = 20
	}
	motd := cfg.MOTD
	if motd == "" {
		motd = "A gomc server"
	}
	return text.StatusResponse{
		Version:     text.StatusVersion{Name: packet.GameVersion, Protocol: packet.ProtocolVersion},
		Players:     text.StatusPlayers{Max: maxPlayers, Online: 0},
		Description: text.Plain(motd),
	}
}
