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
	userID := userIDFromMessage(update.Message)
	username := usernameFromMessage(update.Message)
	rawText := strings.TrimSpace(update.Message.Text)
	text := strings.ToLower(rawText)

	if c.chatID == "" {
		return c.handleUnpairedMessage(ctx, update.Message, chatID, userID, username, rawText, text)
	}

	if chatID != c.chatID {
		slog.Warn("ignoring telegram message from unauthorized chat", "chat_id", chatID)
		return nil
	}
	if text == "/pair" || strings.HasPrefix(text, "/pair ") {
		return c.sendMessage(ctx, chatID, "Бот уже привязан к этому чату.", true)
	}

	switch text {
	case "/start", "/help", "help":
		return c.sendMessage(ctx, chatID, helpMessage(), true)
	case "/whoami", "whoami":
		return c.sendMessage(ctx, chatID, whoamiMessage(chatID, userID, username, true), true)
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

func (c *Client) handleUnpairedMessage(ctx context.Context, msg *message, chatID, userID, username, rawText, text string) error {
	switch {
	case text == "/start" || text == "/help" || text == "help":
		return c.sendMessage(ctx, chatID, unpairedHelpMessage(), false)
	case text == "/whoami" || text == "whoami":
		return c.sendMessage(ctx, chatID, whoamiMessage(chatID, userID, username, false), false)
	case strings.HasPrefix(text, "/pair "):
		return c.tryPair(ctx, msg, chatID, userID, username, rawText)
	default:
		return c.sendMessage(ctx, chatID, "Бот еще не привязан.\n\n"+unpairedHelpMessage(), false)
	}
}

func (c *Client) tryPair(ctx context.Context, msg *message, chatID, userID, username, rawText string) error {
	if msg.Chat.Type != "" && msg.Chat.Type != "private" {
		return c.sendMessage(ctx, chatID, "Pairing разрешен только в личном чате с ботом.", false)
	}

	now := time.Now()
	if now.Before(c.pairingBlockedUntil) {
		return c.sendMessage(ctx, chatID, "Слишком много неверных попыток. Попробуй позже.", false)
	}
	if c.pairingCode == "" || now.After(c.pairingExpiresAt) {
		c.pairingCode = ""
		if err := c.ensurePairingCode(); err != nil {
			return err
		}
		slog.Info("telegram pairing code regenerated", "code", c.pairingCode, "expires_at", c.pairingExpiresAt.Format(time.RFC3339), "state_path", c.statePath)
		return c.sendMessage(ctx, chatID, "Pairing code истек. Новый code записан в server logs.", false)
	}

	fields := strings.Fields(rawText)
	if len(fields) != 2 {
		return c.sendMessage(ctx, chatID, "Используй формат: /pair CODE", false)
	}
	code := fields[1]
	if !strings.EqualFold(code, c.pairingCode) {
		c.failedPairAttempts++
		if c.failedPairAttempts >= maxPairAttempts {
			c.pairingBlockedUntil = now.Add(pairingBlock)
			c.failedPairAttempts = 0
		}
		return c.sendMessage(ctx, chatID, "Неверный pairing code.", false)
	}

	if err := c.saveState(chatID, userID, username); err != nil {
		return err
	}
	return c.sendMessage(ctx, chatID, "Pairing завершен. Теперь этот чат управляет server-bot.", true)
}

func userIDFromMessage(msg *message) string {
	if msg.From == nil {
		return ""
	}
	return strconv.FormatInt(msg.From.ID, 10)
}

func usernameFromMessage(msg *message) string {
	if msg.From == nil {
		return ""
	}
	return strings.TrimSpace(msg.From.Username)
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
