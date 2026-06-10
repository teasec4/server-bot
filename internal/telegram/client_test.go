package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"server-bot/internal/config"
	"server-bot/internal/monitor"
)

func newTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()

	client, err := New("secret", "12345")
	if err != nil {
		t.Fatal(err)
	}
	client.baseURL = server.URL
	client.httpClient = server.Client()
	return client
}

func TestSendMessagePostsToTelegram(t *testing.T) {
	var received sendMessageRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/botsecret/sendMessage" {
			t.Fatalf("path = %q, want /botsecret/sendMessage", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(sendMessageResponse{OK: true})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	if err := client.SendMessage(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}

	if received.ChatID != "12345" || received.Text != "hello" {
		t.Fatalf("unexpected message payload: %+v", received)
	}
}

func TestGetUpdatesReadsTelegramResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(getUpdatesResponse{
			OK: true,
			Result: []update{
				{UpdateID: 7, Message: &message{Text: "/status", Chat: chat{ID: 12345}}},
			},
		})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	updates, err := client.getUpdates(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 1 || updates[0].UpdateID != 7 {
		t.Fatalf("unexpected updates: %+v", updates)
	}
}

func TestHandleUpdateSendsReportWithKeyboard(t *testing.T) {
	var received sendMessageRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(sendMessageResponse{OK: true})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	mon := monitor.New(&config.Config{
		Targets: []config.TargetConfig{
			{
				ID:                  "site",
				Name:                "Main site",
				Type:                "http",
				URL:                 "https://example.com",
				ConsecutiveFailures: 2,
			},
		},
	})

	err := client.handleUpdate(context.Background(), mon, update{
		UpdateID: 1,
		Message:  &message{Text: buttonReport, Chat: chat{ID: 12345}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(received.Text, "Отчет server-bot") {
		t.Fatalf("expected report text, got %q", received.Text)
	}
	if received.ReplyMarkup == nil {
		t.Fatal("expected reply keyboard")
	}
}
