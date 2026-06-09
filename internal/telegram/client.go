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
	defaultBaseURL = "https://api.telegram.org"
	defaultTimeout = 10 * time.Second
)

type Client struct {
	token      string
	chatID     string
	baseURL    string
	httpClient *http.Client
}

type Option func(*Client)

func WithBaseURL(baseURL string) Option {
	return func(client *Client) {
		client.baseURL = strings.TrimRight(baseURL, "/")
	}
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		client.httpClient = httpClient
	}
}

// New создает Telegram-клиент поверх обычного Bot API.
// token и chatID приходят из переменных окружения, а не из config.json, чтобы не хранить секреты в git.
func New(token, chatID string, options ...Option) (*Client, error) {
	token = strings.TrimSpace(token)
	chatID = strings.TrimSpace(chatID)
	if token == "" {
		return nil, errors.New("telegram bot token is required")
	}
	if chatID == "" {
		return nil, errors.New("telegram chat id is required")
	}

	client := &Client{
		token:   token,
		chatID:  chatID,
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
	for _, option := range options {
		option(client)
	}
	if client.baseURL == "" {
		return nil, errors.New("telegram base url is required")
	}
	if client.httpClient == nil {
		client.httpClient = &http.Client{Timeout: defaultTimeout}
	}

	return client, nil
}

// NewFromEnv включает Telegram только если заданы обе переменные.
// Если переменные отсутствуют, enabled=false и приложение продолжает работать без уведомлений.
func NewFromEnv() (*Client, bool, error) {
	token := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	chatID := strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID"))
	if token == "" && chatID == "" {
		return nil, false, nil
	}
	if token == "" || chatID == "" {
		return nil, false, errors.New("set both TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID")
	}

	client, err := New(token, chatID)
	if err != nil {
		return nil, false, err
	}
	return client, true, nil
}

// NotifyTargetEvent превращает событие monitor в обычный текст и отправляет его в Telegram.
func (c *Client) NotifyTargetEvent(ctx context.Context, event monitor.TargetEvent) error {
	return c.SendMessage(ctx, FormatTargetEvent(event))
}

func (c *Client) SendMessage(ctx context.Context, text string) error {
	payload := sendMessageRequest{
		ChatID:                c.chatID,
		Text:                  text,
		DisableWebPagePreview: true,
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

// FormatTargetEvent держит формат сообщений в одном месте, чтобы алерты были предсказуемыми.
func FormatTargetEvent(event monitor.TargetEvent) string {
	target := event.Target
	result := target.LastResult

	var builder strings.Builder
	if event.CurrentState == "down" {
		builder.WriteString("ALERT: цель недоступна\n")
	} else {
		builder.WriteString("RECOVERY: цель восстановилась\n")
	}

	builder.WriteString("name: ")
	builder.WriteString(target.Name)
	builder.WriteString("\n")
	builder.WriteString("id: ")
	builder.WriteString(target.ID)
	builder.WriteString("\n")
	builder.WriteString("state: ")
	builder.WriteString(event.PreviousState)
	builder.WriteString(" -> ")
	builder.WriteString(event.CurrentState)
	builder.WriteString("\n")
	builder.WriteString("url: ")
	builder.WriteString(target.URL)
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("failures: %d/%d\n", target.ConsecutiveFailures, target.FailureThreshold))

	if result == nil {
		return strings.TrimSpace(builder.String())
	}

	builder.WriteString(fmt.Sprintf("duration: %dms\n", result.DurationMS))
	if result.HTTPStatus != 0 {
		builder.WriteString(fmt.Sprintf("http_status: %d\n", result.HTTPStatus))
	}
	if result.Error != "" {
		builder.WriteString("error: ")
		builder.WriteString(result.Error)
		builder.WriteString("\n")
	} else if result.Description != "" {
		builder.WriteString("description: ")
		builder.WriteString(result.Description)
		builder.WriteString("\n")
	}
	builder.WriteString("checked_at: ")
	builder.WriteString(result.CheckedAt.Format(time.RFC3339))

	return strings.TrimSpace(builder.String())
}

type sendMessageRequest struct {
	ChatID                string `json:"chat_id"`
	Text                  string `json:"text"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview"`
}

type sendMessageResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}
