package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"server-bot/internal/checks"
	"server-bot/internal/monitor"
)

func TestSendMessagePostsToTelegramAPI(t *testing.T) {
	var received sendMessageRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/botsecret/sendMessage" {
			t.Fatalf("path = %q, want /botsecret/sendMessage", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(sendMessageResponse{OK: true})
	}))
	defer server.Close()

	client, err := New("secret", "12345", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}

	if err := client.SendMessage(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	if received.ChatID != "12345" {
		t.Fatalf("chat id = %q, want 12345", received.ChatID)
	}
	if received.Text != "hello" {
		t.Fatalf("text = %q, want hello", received.Text)
	}
	if !received.DisableWebPagePreview {
		t.Fatal("disable_web_page_preview = false, want true")
	}
}

func TestFormatTargetEventDownIncludesDetails(t *testing.T) {
	message := FormatTargetEvent(monitor.TargetEvent{
		PreviousState: "suspect",
		CurrentState:  "down",
		Target: monitor.TargetState{
			ID:                  "site",
			Name:                "Main site",
			URL:                 "https://example.com",
			State:               "down",
			ConsecutiveFailures: 2,
			FailureThreshold:    2,
			LastResult: &checks.Result{
				CheckedAt:   time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC),
				DurationMS:  120,
				HTTPStatus:  http.StatusInternalServerError,
				Description: "unexpected HTTP status 500, expected 200",
			},
		},
	})

	for _, want := range []string{
		"ALERT: цель недоступна",
		"name: Main site",
		"id: site",
		"state: suspect -> down",
		"http_status: 500",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("message does not contain %q:\n%s", want, message)
		}
	}
}
