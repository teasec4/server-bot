package monitor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"server-bot/internal/config"
)

type recordingNotifier struct {
	mu     sync.Mutex
	events []TargetEvent
}

func (n *recordingNotifier) NotifyTargetEvent(ctx context.Context, event TargetEvent) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.events = append(n.events, event)
	return nil
}

func (n *recordingNotifier) Events() []TargetEvent {
	n.mu.Lock()
	defer n.mu.Unlock()
	events := make([]TargetEvent, len(n.events))
	copy(events, n.events)
	return events
}

func TestMonitorNotifiesOnDownAndRecovery(t *testing.T) {
	status := http.StatusInternalServerError
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}))
	defer server.Close()

	notifier := &recordingNotifier{}
	monitor := New(&config.Config{
		Targets: []config.TargetConfig{
			{
				ID:                  "site",
				Name:                "Site",
				Type:                "http",
				URL:                 server.URL,
				Method:              "GET",
				Interval:            config.Duration{Duration: time.Second},
				Timeout:             config.Duration{Duration: time.Second},
				ExpectedStatus:      http.StatusOK,
				ConsecutiveFailures: 1,
			},
		},
	}, WithNotifier(notifier))

	monitor.CheckAll(context.Background())
	status = http.StatusOK
	monitor.CheckAll(context.Background())

	events := notifier.Events()
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2: %+v", len(events), events)
	}
	if events[0].PreviousState != "pending" || events[0].CurrentState != "down" {
		t.Fatalf("first event = %+v, want pending -> down", events[0])
	}
	if events[1].PreviousState != "down" || events[1].CurrentState != "up" {
		t.Fatalf("second event = %+v, want down -> up", events[1])
	}
}
