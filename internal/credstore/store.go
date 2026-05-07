// Package credstore persists the fine-grained GitHub PAT used by GitFive-Go.
// It supports two backends: the host's OS keyring (preferred) and a
// base64-encoded JSON file under ~/.gitfive_go/. The choice is recorded in
// the file as a "storage" marker so subsequent reads know where to look.
package credstore

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

// Backend identifies which storage strategy is in effect.
type Backend string

const (
	// BackendKeyring stores the token in the OS keyring (Keychain on
	// macOS, Credential Manager on Windows, Secret Service / libsecret
	// on Linux). The on-disk file holds only a marker.
	BackendKeyring Backend = "keyring"

	// BackendFile stores the token inline in the on-disk JSON file.
	BackendFile Backend = "file"
)

// Hard-coded keyring identifiers. Single-PAT design — re-saving overwrites.
const (
	keyringService = "gitfive-go"
	keyringAccount = "fine-grained-pat"
)

// Store persists a single fine-grained PAT.
type Store struct {
	credsPath string
}

// New returns a Store rooted at credsPath (typically ~/.gitfive_go/creds.m).
func New(credsPath string) *Store {
	return &Store{credsPath: credsPath}
}

// CredsPath returns the absolute path to the on-disk marker / fallback file.
// The Store owns this path; callers must not recompute it from environment
// because that path can drift if the environment changes mid-run (e.g. tests
// that rewrite $HOME after construction).
func (s *Store) CredsPath() string { return s.credsPath }

// onDiskState is the JSON shape persisted as base64 inside credsPath.
type onDiskState struct {
	Storage Backend `json:"storage"`
	Token   string  `json:"token,omitempty"` // present only when Storage == BackendFile
}

// Load returns the stored token and the backend in use. An empty token
// (with empty error) means nothing is stored — the caller should treat this
// as "user has not logged in yet".
//
// Legacy v0.1.1 creds.m (which lacks a "storage" field but contains a
// "token" field) is read transparently as BackendFile so existing users
// keep working until their next login.
func (s *Store) Load() (token string, backend Backend, err error) {
	raw, err := os.ReadFile(s.credsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", nil
		}
		return "", "", err
	}
	decoded, err := base64.StdEncoding.DecodeString(string(raw))
	if err != nil {
		return "", "", fmt.Errorf("decode creds: %w", err)
	}

	var state onDiskState
	if err := json.Unmarshal(decoded, &state); err != nil {
		return "", "", fmt.Errorf("parse creds: %w", err)
	}

	switch state.Storage {
	case BackendKeyring:
		tok, err := keyring.Get(keyringService, keyringAccount)
		if err != nil {
			if errors.Is(err, keyring.ErrNotFound) {
				// Marker says keyring, but entry is gone (cleared by
				// the user, OS update, etc.). Treat as "no token saved"
				// rather than as an error.
				return "", BackendKeyring, nil
			}
			return "", BackendKeyring, fmt.Errorf("keyring get: %w", err)
		}
		return tok, BackendKeyring, nil

	case BackendFile:
		return state.Token, BackendFile, nil

	case "":
		// Legacy v0.1.1: no "storage" field. If a "token" key was set we
		// treat it as file storage; otherwise nothing is stored.
		var legacy struct {
			Token string `json:"token"`
		}
		_ = json.Unmarshal(decoded, &legacy)
		if legacy.Token != "" {
			return legacy.Token, BackendFile, nil
		}
		return "", "", nil

	default:
		return "", "", fmt.Errorf("unknown storage backend %q", state.Storage)
	}
}

// Save persists token using the requested backend. If requested is
// BackendKeyring and the keyring is unavailable, the function falls back to
// BackendFile and writes a one-line warning to stderr — keeping zero-config
// installs on D-Bus-less Linux containers and similar environments working.
//
// Stale entries on the other backend are best-effort deleted so a rotated
// PAT does not leave behind a recoverable token on the unused backend.
func (s *Store) Save(token string, requested Backend) (used Backend, err error) {
	if token == "" {
		return "", errors.New("refusing to save empty token")
	}
	if err := os.MkdirAll(filepath.Dir(s.credsPath), 0o700); err != nil {
		return "", fmt.Errorf("ensure creds dir: %w", err)
	}

	switch requested {
	case BackendKeyring:
		if err := keyring.Set(keyringService, keyringAccount, token); err != nil {
			// We surface the underlying error in the warning because users
			// debugging headless / D-Bus-less hosts need to know the cause.
			// `go-keyring` is documented not to embed secrets in its error
			// strings; if that ever changes this message must be sanitized.
			fmt.Fprintf(os.Stderr,
				"[!] OS keyring unavailable (%v); falling back to file storage at %s\n",
				err, s.credsPath)
			// Best-effort: clear any stale keyring entry from a previous
			// successful login. Without this, a transient Set failure
			// (e.g. quota / locked but readable keyring) would leave the
			// old PAT recoverable in the keyring while the new token went
			// to file. Mirrors the cleanup in the BackendFile branch.
			_ = keyring.Delete(keyringService, keyringAccount)
			return s.saveFile(token)
		}
		// keyring.Set succeeded. Persist the marker file. On failure we
		// MUST compensate by deleting the keyring entry; otherwise the
		// token would be orphaned in the keyring with no marker pointing
		// at it, leaving the user unable to recover via the CLI.
		if err := s.writeState(onDiskState{Storage: BackendKeyring}); err != nil {
			_ = keyring.Delete(keyringService, keyringAccount)
			return "", err
		}
		return BackendKeyring, nil

	case BackendFile:
		// Persist the file FIRST so that, if the disk write fails, the
		// previous keyring entry remains usable. Only after the file is
		// safely written do we clear the now-unused keyring entry.
		used, err := s.saveFile(token)
		if err != nil {
			return "", err
		}
		_ = keyring.Delete(keyringService, keyringAccount) // best effort
		return used, nil

	default:
		return "", fmt.Errorf("unknown backend %q", requested)
	}
}

// Clean removes the token from BOTH backends and removes the on-disk file
// entirely. "Not found" errors on either backend are tolerated (idempotent
// reset), but any *other* failure is surfaced so callers cannot mistakenly
// report a successful cleanup when a token is still recoverable on disk
// or in the keyring.
func (s *Store) Clean() error {
	var errs []error
	if err := keyring.Delete(keyringService, keyringAccount); err != nil &&
		!errors.Is(err, keyring.ErrNotFound) {
		errs = append(errs, fmt.Errorf("delete keyring entry: %w", err))
	}
	if err := os.Remove(s.credsPath); err != nil && !os.IsNotExist(err) {
		errs = append(errs, fmt.Errorf("remove creds file: %w", err))
	}
	return errors.Join(errs...)
}

func (s *Store) saveFile(token string) (Backend, error) {
	if err := s.writeState(onDiskState{Storage: BackendFile, Token: token}); err != nil {
		return "", err
	}
	return BackendFile, nil
}

func (s *Store) writeState(state onDiskState) error {
	blob, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString(blob)
	return os.WriteFile(s.credsPath, []byte(encoded), 0o600)
}
