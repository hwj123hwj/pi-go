package feishu

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Credentials holds the Feishu app credentials persisted to disk.
type Credentials struct {
	AppID           string `json:"app_id"`
	AppSecret       string `json:"app_secret"`
	UserOpenID      string `json:"user_open_id,omitempty"`
	UserAccessToken string `json:"user_access_token,omitempty"`
	UserRefreshToken string `json:"user_refresh_token,omitempty"`
	BotName         string `json:"bot_name,omitempty"`
	BotOpenID       string `json:"bot_open_id,omitempty"`
	Platform        string `json:"platform,omitempty"` // "feishu" or "lark"
}

// credentialsPath returns the path to the feishu credentials file.
func credentialsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "feishu-credentials.json"
	}
	return filepath.Join(home, ".pi-go", "feishu-credentials.json")
}

// SaveCredentials writes feishu credentials to ~/.pi-go/feishu-credentials.json.
func SaveCredentials(creds Credentials) error {
	dir := filepath.Dir(credentialsPath())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create credentials dir: %w", err)
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	if err := os.WriteFile(credentialsPath(), data, 0o600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}

	return nil
}

// LoadCredentials reads feishu credentials from disk.
// Returns nil if the file does not exist.
func LoadCredentials() (*Credentials, error) {
	data, err := os.ReadFile(credentialsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read credentials: %w", err)
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}

	return &creds, nil
}

// DeleteCredentials removes the credentials file.
func DeleteCredentials() error {
	err := os.Remove(credentialsPath())
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete credentials: %w", err)
	}
	return nil
}

// HasCredentials returns true if credentials file exists and has valid app_id/secret.
func HasCredentials() bool {
	creds, err := LoadCredentials()
	if err != nil || creds == nil {
		return false
	}
	return creds.AppID != "" && creds.AppSecret != ""
}
