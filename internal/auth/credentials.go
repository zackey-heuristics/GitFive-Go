package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zackey-heuristics/gitfive-go/internal/util"
)

// Credentials holds GitHub authentication data.
type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Token    string `json:"token"`

	Session map[string]string `json:"-"`

	credsPath   string
	sessionPath string
}

// NewCredentials initializes credential paths.
func NewCredentials() (*Credentials, error) {
	dir, err := util.GitfiveDir()
	if err != nil {
		return nil, fmt.Errorf("credentials: %w", err)
	}
	return &Credentials{
		Session:     make(map[string]string),
		credsPath:   filepath.Join(dir, "creds.m"),
		sessionPath: filepath.Join(dir, "session.m"),
	}, nil
}

// CredsPath returns the path to the credentials file.
func (c *Credentials) CredsPath() string { return c.credsPath }

// SessionPath returns the path to the session file.
func (c *Credentials) SessionPath() string { return c.sessionPath }

// Load reads credentials and session from disk.
func (c *Credentials) Load() {
	if creds := parseFile(c.credsPath); creds != nil {
		if v, ok := creds["username"].(string); ok {
			c.Username = v
		}
		if v, ok := creds["password"].(string); ok {
			c.Password = v
		}
		if v, ok := creds["token"].(string); ok {
			c.Token = v
		}
	}
	if session := parseFile(c.sessionPath); session != nil {
		c.Session = make(map[string]string)
		for k, v := range session {
			if s, ok := v.(string); ok {
				c.Session[k] = s
			}
		}
	}
}

// Save writes credentials and session to disk.
func (c *Credentials) Save() error {
	if err := saveFile(c.credsPath, map[string]string{
		"username": c.Username,
		"password": c.Password,
		"token":    c.Token,
	}); err != nil {
		return err
	}
	return saveFile(c.sessionPath, c.Session)
}

// AreLoaded returns true if username, password, and token are all set.
func (c *Credentials) AreLoaded() bool {
	return c.Username != "" && c.Password != "" && c.Token != ""
}

// Clean removes credential and session files from disk.
func (c *Credentials) Clean() error {
	var firstErr error

	if err := os.Remove(c.credsPath); err != nil && !os.IsNotExist(err) && firstErr == nil {
		firstErr = err
	}
	if err := os.Remove(c.sessionPath); err != nil && !os.IsNotExist(err) && firstErr == nil {
		firstErr = err
	}

	return firstErr
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
