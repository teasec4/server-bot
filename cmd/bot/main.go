package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"server-bot/internal/config"
	"server-bot/internal/handler"
	"server-bot/internal/monitor"
)

func main() {
	configPath := flag.String("config", "config.json", "path to JSON config")
	once := flag.Bool("once", false, "run all checks once, print status, and exit")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mon := monitor.New(cfg)
	if *once {
		mon.CheckAll(ctx)
		if err := json.NewEncoder(os.Stdout).Encode(mon.Snapshot(time.Now())); err != nil {
			slog.Error("failed to write status", "error", err)
			os.Exit(1)
		}
		return
	}

	mon.Start(ctx)

	server := &http.Server{
		Addr:              cfg.Listen,
		Handler:           handler.New(mon),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("server-bot started", "listen", cfg.Listen)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "shutdown failed: %v\n", err)
		os.Exit(1)
	}
}
