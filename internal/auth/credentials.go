package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zackey-heuristics/gitfive-go/internal/util"
)

// Credentials holds the fine-grained PAT and the resolved GitHub username.
type Credentials struct {
	Username string `json:"username"`
	Token    string `json:"token"`

	credsPath string
}

// NewCredentials initializes credential paths.
func NewCredentials() (*Credentials, error) {
	dir, err := util.GitfiveDir()
	if err != nil {
		return nil, fmt.Errorf("credentials: %w", err)
	}
	return &Credentials{
		credsPath: filepath.Join(dir, "creds.m"),
	}, nil
}

// CredsPath returns the path to the credentials file.
func (c *Credentials) CredsPath() string { return c.credsPath }

// Load reads credentials from disk.
func (c *Credentials) Load() {
	if creds := parseFile(c.credsPath); creds != nil {
		if v, ok := creds["username"].(string); ok {
			c.Username = v
		}
		if v, ok := creds["token"].(string); ok {
			c.Token = v
		}
	}
}

// Save writes credentials to disk.
func (c *Credentials) Save() error {
	return saveFile(c.credsPath, map[string]string{
		"username": c.Username,
		"token":    c.Token,
	})
}

// AreLoaded returns true if a token is present. Username is populated by
// CheckToken on first validation, so it is not part of the readiness check.
func (c *Credentials) AreLoaded() bool {
	return c.Token != ""
}

// Clean removes the credentials file from disk.
func (c *Credentials) Clean() error {
	if err := os.Remove(c.credsPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func parseFile(path string) map[string]interface{} {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return nil
	}
	var result map[string]interface{}
	if err := json.Unmarshal(decoded, &result); err != nil {
		return nil
	}
	return result
}

func saveFile(path string, data interface{}) error {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString(jsonBytes)
	return os.WriteFile(path, []byte(encoded), 0o600)
}
