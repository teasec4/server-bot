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
	// configPath - единственный источник правды для проверок:
	// какие URL проверять, как часто, с каким таймаутом и какие даты оплаты помнить.
	configPath := flag.String("config", "config.json", "path to JSON config")

	// once нужен для ручной проверки и будущих cron/systemd timer-сценариев:
	// приложение выполняет все проверки один раз, печатает JSON-статус и завершается.
	once := flag.Bool("once", false, "run all checks once, print status, and exit")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Этот context закрывается при Ctrl+C или SIGTERM от Docker/systemd.
	// Через него останавливаются циклы проверок и HTTP-сервер.
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

	// В обычном режиме монитор сам запускает отдельный цикл проверок для каждой цели.
	mon.Start(ctx)

	// HTTP API пока локальное и простое:
	// /health показывает, что сам бот жив,
	// /status отдает последний известный статус всех проверок.
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

	// Даем серверу до 10 секунд на аккуратное завершение активных запросов.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "shutdown failed: %v\n", err)
		os.Exit(1)
	}
}
