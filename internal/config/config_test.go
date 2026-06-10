package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadAppliesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	err := os.WriteFile(path, []byte(`{
		"targets": [
			{"id": "site", "url": "https://example.com"}
		],
		"renewals": [
			{"id": "vps", "due_date": "2026-07-01"}
		]
	}`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	target := cfg.Targets[0]
	if cfg.Listen != ":8080" {
		t.Fatalf("listen = %q, want :8080", cfg.Listen)
	}
	if target.Type != "http" || target.Method != "GET" {
		t.Fatalf("unexpected target defaults: %+v", target)
	}
	if target.Interval.Duration != time.Minute {
		t.Fatalf("interval = %s, want 1m", target.Interval.Duration)
	}
	if target.Timeout.Duration != 5*time.Second {
		t.Fatalf("timeout = %s, want 5s", target.Timeout.Duration)
	}
	if target.ExpectedStatus != 200 || target.ConsecutiveFailures != 2 {
		t.Fatalf("unexpected target thresholds: %+v", target)
	}
	if len(cfg.Renewals[0].WarnDays) == 0 {
		t.Fatal("renewal warn_days defaults were not applied")
	}
}
