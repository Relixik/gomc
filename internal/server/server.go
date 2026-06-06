package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
)

// Run starts the TCP listener and accepts connections until ctx is cancelled.
//
// M0 status: this is a placeholder accept loop that logs each connection and
// closes it. The real per-connection lifecycle — Handshaking -> (Status|Login)
// -> Configuration -> Play — lands in M1 via internal/net/session, and shared
// world mutation goes through the single authoritative tick loop in
// internal/game/loop. See PLAN.md for the full roadmap.
func Run(ctx context.Context, cfg Config) error {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", cfg.Addr())
	if err != nil {
		return err
	}
	slog.Info("listening", "addr", cfg.Addr())

	// Close the listener when the context is cancelled so Accept unblocks.
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil // clean shutdown
			}
			slog.Warn("accept error", "err", err)
			continue
		}
		go handleConn(ctx, conn)
	}
}

// handleConn is the per-connection goroutine. In M1 this becomes
// session.New(conn).Serve(ctx) which owns the read loop and a dedicated
// outbound write goroutine.
func handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	slog.Info("connection accepted", "remote", conn.RemoteAddr().String())
	// M0: no protocol handling yet.
}
