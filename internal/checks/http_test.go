package checks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"server-bot/internal/config"
)

func TestRunHTTPChecksExpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	result := RunHTTP(context.Background(), config.TargetConfig{
		ID:             "site",
		Name:           "Site",
		Type:           "http",
		URL:            server.URL,
		Method:         "GET",
		Timeout:        config.Duration{Duration: time.Second},
		ExpectedStatus: http.StatusNoContent,
	})

	if !result.OK {
		t.Fatalf("result should be ok: %+v", result)
	}
	if result.HTTPStatus != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", result.HTTPStatus, http.StatusNoContent)
	}
}

func TestRunHTTPFailsOnUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	result := RunHTTP(context.Background(), config.TargetConfig{
		ID:             "site",
		Name:           "Site",
		Type:           "http",
		URL:            server.URL,
		Method:         "GET",
		Timeout:        config.Duration{Duration: time.Second},
		ExpectedStatus: http.StatusOK,
	})

	if result.OK {
		t.Fatalf("result should fail on unexpected status: %+v", result)
	}
}
