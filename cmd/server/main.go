// Command server is the entry point of the gomc Minecraft Java server.
//
// Target: Minecraft Java Edition 26.1.2 (network protocol 775).
// Principle: standard library only — no third-party dependencies.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Relixik/gomc/internal/server"
)

func main() {
	host := flag.String("host", "0.0.0.0", "address the server binds to")
	port := flag.Int("port", 25565, "port the server binds to")
	debug := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	// Graceful shutdown on Ctrl+C / SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := server.Config{Host: *host, Port: *port}

	slog.Info("starting gomc", "protocol", 775, "version", "26.1.2")
	if err := server.Run(ctx, cfg); err != nil {
		slog.Error("server stopped with error", "err", err)
		os.Exit(1)
	}
	slog.Info("server stopped cleanly")
}
