package auth

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/zackey-heuristics/gitfive-go/internal/credstore"
	"github.com/zackey-heuristics/gitfive-go/internal/util"
)

// Credentials holds the fine-grained PAT and the resolved GitHub username.
//
// Username is in-memory only — it is rederived from `GET /user` by
// CheckToken on every Login flow, so persisting it is unnecessary. The
// token is the only secret persisted, and it is delegated to credstore
// (OS keyring or fallback file).
type Credentials struct {
	Username string
	Token    string

	store *credstore.Store

	// PreferredBackend selects the credstore backend used by Save. When
	// unset, Save uses BackendKeyring (auto-falling back to file inside
	// credstore on keyring failure).
	PreferredBackend credstore.Backend

	// ActiveBackend records which backend Save actually used after the
	// last successful Save. Useful for user-facing messages such as
	// "Token stored in OS keyring" vs "Token stored in file at ...".
	ActiveBackend credstore.Backend
}

// NewCredentials returns Credentials wired to the standard on-disk path.
func NewCredentials() (*Credentials, error) {
	dir, err := util.GitfiveDir()
	if err != nil {
		return nil, fmt.Errorf("credentials: %w", err)
	}
	return &Credentials{
		store: credstore.New(filepath.Join(dir, "creds.m")),
	}, nil
}

// CredsPath returns the path to the credentials marker / fallback file.
// Useful for diagnostics and the askpass error messages.
func (c *Credentials) CredsPath() string {
	if c.store == nil {
		return ""
	}
	return c.store.CredsPath()
}

// Load reads credentials from disk (and the OS keyring if the marker says so).
// Errors are silently ignored to preserve prior "best-effort" behaviour for
// callers that only check AreLoaded(); explicit error reporting belongs at
// login time.
func (c *Credentials) Load() {
	if c.store == nil {
		return
	}
	tok, _, err := c.store.Load()
	if err != nil {
		// Surface a one-line warning so a corrupt creds.m is not silently
		// invisible. Token stays empty; AreLoaded() will return false and
		// callers will route the user to `gitfive-go login`.
		fmt.Fprintf(os.Stderr, "[!] failed to load credentials: %v\n", err)
		return
	}
	c.Token = tok
}

// Save writes the token via credstore using the configured PreferredBackend
// (defaulting to BackendKeyring). Records the backend actually used in
// ActiveBackend (which may differ from PreferredBackend if credstore fell
// back from keyring to file). Username is in-memory only and is NOT
// persisted.
func (c *Credentials) Save() error {
	if c.store == nil {
		return fmt.Errorf("credentials store not initialized")
	}
	backend := c.PreferredBackend
	if backend == "" {
		backend = credstore.BackendKeyring
	}
	used, err := c.store.Save(c.Token, backend)
	if err != nil {
		return err
	}
	c.ActiveBackend = used
	return nil
}

// AreLoaded returns true if a token is present. Username is populated by
// CheckToken on first validation, so it is not part of the readiness check.
func (c *Credentials) AreLoaded() bool {
	return c.Token != ""
}

// Clean removes the token from BOTH backends (keyring + file). After a
// successful Clean, no GitFive-Go credentials remain on the host.
func (c *Credentials) Clean() error {
	if c.store == nil {
		return nil
	}
	return c.store.Clean()
}
