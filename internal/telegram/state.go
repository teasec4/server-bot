package telegram

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultStatePath = "data/state.json"
	pairingTTL       = 15 * time.Minute
	maxPairAttempts  = 5
	pairingBlock     = time.Minute
)

type stateFile struct {
	AdminChatID string `json:"admin_chat_id"`
	AdminUserID string `json:"admin_user_id"`
	Username    string `json:"username,omitempty"`
	PairedAt    string `json:"paired_at"`
}

func statePathFromEnv() string {
	path := strings.TrimSpace(os.Getenv("TELEGRAM_STATE_PATH"))
	if path == "" {
		return defaultStatePath
	}
	return path
}

func (c *Client) loadState() error {
	data, err := os.ReadFile(c.statePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read telegram state: %w", err)
	}

	var state stateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("decode telegram state: %w", err)
	}
	c.chatID = strings.TrimSpace(state.AdminChatID)
	c.adminUserID = strings.TrimSpace(state.AdminUserID)
	return nil
}

func (c *Client) saveState(adminChatID, adminUserID, username string) error {
	state := stateFile{
		AdminChatID: adminChatID,
		AdminUserID: adminUserID,
		Username:    username,
		PairedAt:    time.Now().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode telegram state: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(c.statePath), 0o700); err != nil {
		return fmt.Errorf("create telegram state dir: %w", err)
	}
	if err := os.WriteFile(c.statePath, data, 0o600); err != nil {
		return fmt.Errorf("write telegram state: %w", err)
	}

	c.chatID = adminChatID
	c.adminUserID = adminUserID
	c.pairingCode = ""
	c.pairingExpiresAt = time.Time{}
	c.failedPairAttempts = 0
	c.pairingBlockedUntil = time.Time{}
	return nil
}

func (c *Client) ensurePairingCode() error {
	if c.chatID != "" || c.pairingCode != "" {
		return nil
	}

	code, err := newPairingCode()
	if err != nil {
		return err
	}
	c.pairingCode = code
	c.pairingExpiresAt = time.Now().Add(pairingTTL)
	return nil
}

func (c *Client) PairingCode() (string, time.Time, bool) {
	if c.chatID != "" || c.pairingCode == "" {
		return "", time.Time{}, false
	}
	return c.pairingCode, c.pairingExpiresAt, true
}

func (c *Client) StatePath() string {
	return c.statePath
}

func newPairingCode() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generate pairing code: %w", err)
	}
	return strings.ToUpper(hex.EncodeToString(bytes[:])), nil
}
