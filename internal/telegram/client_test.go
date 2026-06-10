package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func newPairingTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()

	client, err := New("secret", "")
	if err != nil {
		t.Fatal(err)
	}
	client.baseURL = server.URL
	client.httpClient = server.Client()
	client.statePath = filepath.Join(t.TempDir(), "state.json")
	client.pairingCode = "ABC123"
	client.pairingExpiresAt = time.Now().Add(time.Minute)
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

func TestNewFromEnvCanStartWithoutChatID(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "secret")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	t.Setenv("TELEGRAM_STATE_PATH", filepath.Join(t.TempDir(), "state.json"))

	client, enabled, err := NewFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Fatal("telegram should be enabled when token is set")
	}
	if _, _, ok := client.PairingCode(); !ok {
		t.Fatal("expected pairing code")
	}
}

func TestHandleUpdatePairsAdminAndWritesState(t *testing.T) {
	var received sendMessageRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(sendMessageResponse{OK: true})
	}))
	defer server.Close()

	client := newPairingTestClient(t, server)
	mon := monitor.New(&config.Config{})
	err := client.handleUpdate(context.Background(), mon, update{
		UpdateID: 1,
		Message: &message{
			Text: "/pair ABC123",
			Chat: chat{ID: 777, Type: "private"},
			From: &user{ID: 777, Username: "admin"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if client.chatID != "777" {
		t.Fatalf("chatID = %q, want 777", client.chatID)
	}
	if !strings.Contains(received.Text, "Pairing завершен") {
		t.Fatalf("unexpected response: %q", received.Text)
	}

	data, err := os.ReadFile(client.statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"admin_chat_id": "777"`) {
		t.Fatalf("state file does not contain admin chat id:\n%s", string(data))
	}
}

func TestHandleUpdateWhoamiBeforePairing(t *testing.T) {
	var received sendMessageRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(sendMessageResponse{OK: true})
	}))
	defer server.Close()

	client := newPairingTestClient(t, server)
	mon := monitor.New(&config.Config{})
	err := client.handleUpdate(context.Background(), mon, update{
		UpdateID: 1,
		Message: &message{
			Text: "/whoami",
			Chat: chat{ID: 888, Type: "private"},
			From: &user{ID: 888, Username: "debug"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(received.Text, "chat_id: 888") || !strings.Contains(received.Text, "paired: false") {
		t.Fatalf("unexpected whoami response: %q", received.Text)
	}
}
