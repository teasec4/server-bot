package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"server-bot/internal/config"
	"server-bot/internal/handler"
	"server-bot/internal/monitor"
	"server-bot/internal/telegram"
)

func main() {
	// configPath - единственный источник правды для проверок:
	// какие URL проверять, как часто, с каким таймаутом и какие даты оплаты помнить.
	configPath := flag.String("config", defaultConfigPath(), "path to JSON config")

	// once нужен для ручной проверки и будущих cron/systemd timer-сценариев:
	// приложение выполняет все проверки один раз, печатает JSON-статус и завершается.
	once := flag.Bool("once", false, "run all checks once, print status, and exit")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	if err := loadDotEnv(".env"); err != nil {
		slog.Error("failed to load .env", "error", err)
		os.Exit(1)
	}

	// Этот context закрывается при Ctrl+C или SIGTERM от Docker/systemd.
	// Через него останавливаются циклы проверок и HTTP-сервер.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var telegramClient *telegram.Client
	if !*once {
		// Telegram включается только переменными окружения.
		// Без них мониторинг продолжит работать локально через /status.
		client, enabled, err := telegram.NewFromEnv()
		if err != nil {
			slog.Error("failed to configure telegram", "error", err)
			os.Exit(1)
		}
		if enabled {
			slog.Info("telegram notifications enabled")
			telegramClient = client
			if code, expiresAt, ok := telegramClient.PairingCode(); ok {
				slog.Info("telegram pairing required", "code", code, "expires_at", expiresAt.Format(time.RFC3339), "state_path", telegramClient.StatePath())
			}
		}
	}

	mon := monitor.New(cfg)
	if telegramClient != nil {
		mon.SetNotifier(telegramClient)
	}
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
	if telegramClient != nil {
		go func() {
			slog.Info("telegram polling started")
			if err := telegramClient.Run(ctx, mon); err != nil {
				slog.Error("telegram polling stopped", "error", err)
				stop()
			}
		}()
	}

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

func defaultConfigPath() string {
	if _, err := os.Stat("config.json"); err == nil {
		return "config.json"
	}
	if _, err := os.Stat("configs/local.json"); err == nil {
		return "configs/local.json"
	}
	return "config.json"
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return scanner.Err()
}
