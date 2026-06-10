package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"server-bot/internal/monitor"
)

// Run читает входящие сообщения через long polling.
// Для Docker это проще webhook: серверу не нужен публичный HTTPS endpoint.
func (c *Client) Run(ctx context.Context, mon *monitor.Monitor) error {
	offset := 0

	for {
		updates, err := c.getUpdates(ctx, offset)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			slog.Warn("failed to poll telegram updates", "error", err)
			if !sleepOrDone(ctx, 2*time.Second) {
				return nil
			}
			continue
		}

		for _, update := range updates {
			if update.UpdateID >= offset {
				offset = update.UpdateID + 1
			}
			if err := c.handleUpdate(ctx, mon, update); err != nil {
				slog.Warn("failed to handle telegram update", "update_id", update.UpdateID, "error", err)
			}
		}
	}
}

func (c *Client) getUpdates(ctx context.Context, offset int) ([]update, error) {
	payload := getUpdatesRequest{
		Offset:         offset,
		Timeout:        pollTimeoutSeconds,
		AllowedUpdates: []string{"message"},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode telegram updates request: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/getUpdates", c.baseURL, c.token)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build telegram updates request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("poll telegram updates: %w", err)
	}
	defer response.Body.Close()

	var result getUpdatesResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode telegram updates response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || !result.OK {
		if result.Description == "" {
			result.Description = response.Status
		}
		return nil, fmt.Errorf("telegram api rejected updates request: %s", result.Description)
	}
	return result.Result, nil
}

func (c *Client) handleUpdate(ctx context.Context, mon *monitor.Monitor, update update) error {
	if update.Message == nil {
		return nil
	}

	chatID := strconv.FormatInt(update.Message.Chat.ID, 10)
	if chatID != c.chatID {
		slog.Warn("ignoring telegram message from unauthorized chat", "chat_id", chatID)
		return nil
	}

	text := strings.ToLower(strings.TrimSpace(update.Message.Text))
	switch text {
	case "/start", "/help", "help":
		return c.sendMessage(ctx, chatID, helpMessage(), true)
	case "/status", "/report", "status", "report", strings.ToLower(buttonReport):
		return c.sendMessage(ctx, chatID, FormatSnapshot(mon.Snapshot(time.Now())), true)
	case "/check", strings.ToLower(buttonCheckNow):
		mon.CheckAll(ctx)
		return c.sendMessage(ctx, chatID, FormatSnapshot(mon.Snapshot(time.Now())), true)
	case "/ping", strings.ToLower(buttonPing):
		return c.sendMessage(ctx, chatID, "Связь есть. Telegram polling работает, monitor process жив.", true)
	default:
		return c.sendMessage(ctx, chatID, "Не понял команду.\n\n"+helpMessage(), true)
	}
}

func sleepOrDone(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
