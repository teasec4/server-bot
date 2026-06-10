package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvSetsMissingValues(t *testing.T) {
	t.Setenv("EXISTING_VALUE", "from-env")

	path := filepath.Join(t.TempDir(), ".env")
	err := os.WriteFile(path, []byte(`
TELEGRAM_BOT_TOKEN=secret
EXISTING_VALUE=from-file
# ignored comment
QUOTED_VALUE="hello"
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	if err := loadDotEnv(path); err != nil {
		t.Fatal(err)
	}

	if got := os.Getenv("TELEGRAM_BOT_TOKEN"); got != "secret" {
		t.Fatalf("TELEGRAM_BOT_TOKEN = %q, want secret", got)
	}
	if got := os.Getenv("EXISTING_VALUE"); got != "from-env" {
		t.Fatalf("EXISTING_VALUE = %q, want from-env", got)
	}
	if got := os.Getenv("QUOTED_VALUE"); got != "hello" {
		t.Fatalf("QUOTED_VALUE = %q, want hello", got)
	}
}
