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
			{
				"id": "site",
				"url": "https://example.com"
			}
		],
		"renewals": [
			{
				"id": "vps",
				"due_date": "2026-07-01"
			}
		]
	}`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Listen != ":8080" {
		t.Fatalf("listen = %q, want :8080", cfg.Listen)
	}
	target := cfg.Targets[0]
	if target.Name != "site" || target.Type != "http" || target.Method != "GET" {
		t.Fatalf("unexpected target defaults: %+v", target)
	}
	if target.Interval.Duration != time.Minute {
		t.Fatalf("interval = %s, want 1m", target.Interval.Duration)
	}
	if target.Timeout.Duration != 5*time.Second {
		t.Fatalf("timeout = %s, want 5s", target.Timeout.Duration)
	}
	if target.ExpectedStatus != 200 {
		t.Fatalf("expected status = %d, want 200", target.ExpectedStatus)
	}
	if target.ConsecutiveFailures != 2 {
		t.Fatalf("failure threshold = %d, want 2", target.ConsecutiveFailures)
	}

	renewal := cfg.Renewals[0]
	if renewal.Name != "vps" {
		t.Fatalf("renewal name = %q, want vps", renewal.Name)
	}
	if len(renewal.WarnDays) != 4 {
		t.Fatalf("warn days = %+v, want defaults", renewal.WarnDays)
	}
}

func TestLoadRejectsDuplicateTargets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	err := os.WriteFile(path, []byte(`{
		"targets": [
			{"id": "site", "url": "https://example.com"},
			{"id": "site", "url": "https://example.org"}
		]
	}`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected duplicate target error")
	}
}
