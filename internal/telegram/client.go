package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"server-bot/internal/monitor"
)

const (
	telegramAPIURL      = "https://api.telegram.org"
	telegramHTTPTimeout = 35 * time.Second
	pollTimeoutSeconds  = 25

	buttonReport   = "Отчет"
	buttonCheckNow = "Проверить сейчас"
	buttonPing     = "Проверить соединение"
)

type Client struct {
	token               string
	chatID              string
	adminUserID         string
	statePath           string
	pairingCode         string
	pairingExpiresAt    time.Time
	failedPairAttempts  int
	pairingBlockedUntil time.Time
	baseURL             string
	httpClient          *http.Client
}

// New создает клиента Telegram Bot API.
// chatID может быть пустым: тогда admin будет привязан через /pair CODE.
func New(token, chatID string) (*Client, error) {
	token = strings.TrimSpace(token)
	chatID = strings.TrimSpace(chatID)
	if token == "" {
		return nil, errors.New("telegram bot token is required")
	}

	return &Client{
		token:     token,
		chatID:    chatID,
		statePath: statePathFromEnv(),
		baseURL:   telegramAPIURL,
		httpClient: &http.Client{
			Timeout: telegramHTTPTimeout,
		},
	}, nil
}

func NewFromEnv() (*Client, bool, error) {
	token := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	chatID := strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID"))
	if token == "" && chatID == "" {
		return nil, false, nil
	}
	if token == "" {
		return nil, false, errors.New("TELEGRAM_BOT_TOKEN is required when Telegram is enabled")
	}

	client, err := New(token, chatID)
	if err != nil {
		return nil, false, err
	}
	if client.chatID == "" {
		if err := client.loadState(); err != nil {
			return nil, false, err
		}
	}
	if client.chatID == "" {
		if err := client.ensurePairingCode(); err != nil {
			return nil, false, err
		}
	}
	return client, true, nil
}

func (c *Client) NotifyTargetEvent(ctx context.Context, event monitor.TargetEvent) error {
	if c.chatID == "" {
		return nil
	}
	return c.SendMessage(ctx, FormatTargetEvent(event))
}

func (c *Client) SendMessage(ctx context.Context, text string) error {
	if c.chatID == "" {
		return nil
	}
	return c.sendMessage(ctx, c.chatID, text, false)
}

func (c *Client) sendMessage(ctx context.Context, chatID, text string, withKeyboard bool) error {
	if chatID == "" {
		chatID = c.chatID
	}

	payload := sendMessageRequest{
		ChatID:                chatID,
		Text:                  text,
		DisableWebPagePreview: true,
	}
	if withKeyboard {
		payload.ReplyMarkup = defaultKeyboard()
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode telegram message: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", c.baseURL, c.token)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build telegram request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("send telegram message: %w", err)
	}
	defer response.Body.Close()

	var result sendMessageResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode telegram response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || !result.OK {
		if result.Description == "" {
			result.Description = response.Status
		}
		return fmt.Errorf("telegram api rejected message: %s", result.Description)
	}
	return nil
}
