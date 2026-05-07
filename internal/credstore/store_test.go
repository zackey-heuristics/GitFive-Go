package credstore

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

// resetMockKeyring switches go-keyring to its in-memory mock, wiping any
// state from prior tests in the same process. Tests that mutate keyring
// state must not call t.Parallel() because the mock is global.
func resetMockKeyring(t *testing.T) {
	t.Helper()
	keyring.MockInit()
}

func newTempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	return New(filepath.Join(dir, "creds.m"))
}

func TestLoad_NoFileReturnsEmpty(t *testing.T) {
	resetMockKeyring(t)
	s := newTempStore(t)
	tok, backend, err := s.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "" || backend != "" {
		t.Errorf("expected empty/empty, got token=%q backend=%q", tok, backend)
	}
}

func TestSaveLoad_FileBackend(t *testing.T) {
	resetMockKeyring(t)
	s := newTempStore(t)
	used, err := s.Save("github_pat_TEST_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", BackendFile)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if used != BackendFile {
		t.Errorf("used = %q, want file", used)
	}

	tok, backend, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if backend != BackendFile {
		t.Errorf("backend = %q, want file", backend)
	}
	if !strings.HasPrefix(tok, "github_pat_") {
		t.Errorf("unexpected token returned: %q", tok)
	}
}

func TestSaveLoad_KeyringBackend(t *testing.T) {
	resetMockKeyring(t)
	s := newTempStore(t)
	used, err := s.Save("github_pat_KEYRING_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", BackendKeyring)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if used != BackendKeyring {
		t.Errorf("used = %q, want keyring", used)
	}

	// File should be a marker only — token must NOT be present in the
	// on-disk JSON.
	raw, err := os.ReadFile(s.credsPath)
	if err != nil {
		t.Fatalf("read creds.m: %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(string(raw))
	if err != nil {
		t.Fatalf("decode creds.m: %v", err)
	}
	var state map[string]any
	if err := json.Unmarshal(decoded, &state); err != nil {
		t.Fatalf("unmarshal creds.m: %v", err)
	}
	if state["storage"] != "keyring" {
		t.Errorf("storage marker = %v, want keyring", state["storage"])
	}
	if _, hasToken := state["token"]; hasToken {
		t.Errorf("token must not be present on disk in keyring mode: %v", state)
	}

	tok, backend, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if backend != BackendKeyring {
		t.Errorf("backend = %q, want keyring", backend)
	}
	if !strings.HasPrefix(tok, "github_pat_KEYRING_") {
		t.Errorf("unexpected token: %q", tok)
	}
}

func TestSaveKeyring_FallsBackToFileOnError(t *testing.T) {
	keyring.MockInitWithError(errors.New("simulated keyring failure"))
	t.Cleanup(func() { keyring.MockInit() })

	s := newTempStore(t)
	used, err := s.Save("github_pat_FALLBACK_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", BackendKeyring)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if used != BackendFile {
		t.Errorf("used = %q, want file (fallback)", used)
	}

	// File backend should now hold the token inline.
	tok, backend, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if backend != BackendFile {
		t.Errorf("backend after fallback = %q, want file", backend)
	}
	if !strings.HasPrefix(tok, "github_pat_FALLBACK_") {
		t.Errorf("unexpected token after fallback: %q", tok)
	}
}

func TestSwitchKeyringToFile_DeletesKeyringEntry(t *testing.T) {
	resetMockKeyring(t)
	s := newTempStore(t)

	if _, err := s.Save("github_pat_FIRST_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", BackendKeyring); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	if _, err := keyring.Get(keyringService, keyringAccount); err != nil {
		t.Fatalf("expected entry in keyring after first save, got error: %v", err)
	}

	if _, err := s.Save("github_pat_SECOND_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", BackendFile); err != nil {
		t.Fatalf("second Save: %v", err)
	}
	if _, err := keyring.Get(keyringService, keyringAccount); !errors.Is(err, keyring.ErrNotFound) {
		t.Errorf("expected keyring entry to be deleted on switch to file, got err=%v", err)
	}
}

func TestSwitchFileToKeyring_RemovesTokenFromDisk(t *testing.T) {
	resetMockKeyring(t)
	s := newTempStore(t)

	if _, err := s.Save("github_pat_INFILE_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", BackendFile); err != nil {
		t.Fatalf("file save: %v", err)
	}
	if _, err := s.Save("github_pat_INRING_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", BackendKeyring); err != nil {
		t.Fatalf("keyring save: %v", err)
	}

	raw, _ := os.ReadFile(s.credsPath)
	decoded, _ := base64.StdEncoding.DecodeString(string(raw))
	if strings.Contains(string(decoded), "github_pat_INFILE_") {
		t.Errorf("old file-mode token still present on disk after switch to keyring: %s", string(decoded))
	}
}

func TestLegacyV011_MigratesToKeyringOnNextSave(t *testing.T) {
	resetMockKeyring(t)
	s := newTempStore(t)
	if err := os.MkdirAll(filepath.Dir(s.credsPath), 0o700); err != nil {
		t.Fatal(err)
	}
	// Pre-existing v0.1.1 file with inline token.
	legacy := `{"username":"alice","token":"github_pat_OLDLEGACY_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"}`
	encoded := base64.StdEncoding.EncodeToString([]byte(legacy))
	if err := os.WriteFile(s.credsPath, []byte(encoded), 0o600); err != nil {
		t.Fatal(err)
	}

	// First Save under the new code path moves the token to the keyring
	// and rewrites the file as a marker only. The OLD inline token must
	// not survive on disk after migration.
	if _, err := s.Save("github_pat_NEWFRESH_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", BackendKeyring); err != nil {
		t.Fatalf("migrating Save: %v", err)
	}

	raw, _ := os.ReadFile(s.credsPath)
	decoded, _ := base64.StdEncoding.DecodeString(string(raw))
	if strings.Contains(string(decoded), "github_pat_OLDLEGACY_") {
		t.Errorf("legacy token still on disk after migration: %s", string(decoded))
	}
	if strings.Contains(string(decoded), "github_pat_NEWFRESH_") {
		t.Errorf("new token must NOT be on disk in keyring mode, got: %s", string(decoded))
	}

	tok, backend, err := s.Load()
	if err != nil {
		t.Fatalf("Load after migration: %v", err)
	}
	if backend != BackendKeyring {
		t.Errorf("backend after migration = %q, want keyring", backend)
	}
	if !strings.HasPrefix(tok, "github_pat_NEWFRESH_") {
		t.Errorf("token after migration: %q", tok)
	}
}

func TestLoad_LegacyV011Schema(t *testing.T) {
	resetMockKeyring(t)
	s := newTempStore(t)
	if err := os.MkdirAll(filepath.Dir(s.credsPath), 0o700); err != nil {
		t.Fatal(err)
	}
	// Pre-existing v0.1.1 schema: no "storage" field, but has "username"
	// and "token". Should be read as file backend.
	legacy := `{"username":"alice","token":"github_pat_LEGACY_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"}`
	encoded := base64.StdEncoding.EncodeToString([]byte(legacy))
	if err := os.WriteFile(s.credsPath, []byte(encoded), 0o600); err != nil {
		t.Fatal(err)
	}

	tok, backend, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if backend != BackendFile {
		t.Errorf("legacy schema should be read as file backend, got %q", backend)
	}
	if !strings.HasPrefix(tok, "github_pat_LEGACY_") {
		t.Errorf("legacy token mis-read: %q", tok)
	}
}

func TestLoad_KeyringMarkerWithoutEntryReturnsEmpty(t *testing.T) {
	resetMockKeyring(t)
	s := newTempStore(t)
	if err := os.MkdirAll(filepath.Dir(s.credsPath), 0o700); err != nil {
		t.Fatal(err)
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(`{"storage":"keyring"}`))
	if err := os.WriteFile(s.credsPath, []byte(encoded), 0o600); err != nil {
		t.Fatal(err)
	}

	tok, backend, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if tok != "" {
		t.Errorf("token should be empty when keyring has no entry, got %q", tok)
	}
	if backend != BackendKeyring {
		t.Errorf("backend should still be keyring, got %q", backend)
	}
}

func TestSave_RejectsEmptyToken(t *testing.T) {
	resetMockKeyring(t)
	s := newTempStore(t)
	if _, err := s.Save("", BackendFile); err == nil {
		t.Error("expected Save to reject empty token")
	}
	if _, err := s.Save("", BackendKeyring); err == nil {
		t.Error("expected Save to reject empty token in keyring mode")
	}
}

func TestClean_RemovesBothBackends(t *testing.T) {
	resetMockKeyring(t)
	s := newTempStore(t)
	if _, err := s.Save("github_pat_CLEAN1_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", BackendKeyring); err != nil {
		t.Fatal(err)
	}

	if err := s.Clean(); err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if _, err := os.Stat(s.credsPath); !os.IsNotExist(err) {
		t.Errorf("creds.m should be deleted, got stat err=%v", err)
	}
	if _, err := keyring.Get(keyringService, keyringAccount); !errors.Is(err, keyring.ErrNotFound) {
		t.Errorf("keyring entry should be deleted, got err=%v", err)
	}
}

func TestClean_NoOpWhenNothingStored(t *testing.T) {
	resetMockKeyring(t)
	s := newTempStore(t)
	if err := s.Clean(); err != nil {
		t.Errorf("Clean on empty store should not error, got %v", err)
	}
}

func TestLoad_UnknownStorageReturnsError(t *testing.T) {
	resetMockKeyring(t)
	s := newTempStore(t)
	if err := os.MkdirAll(filepath.Dir(s.credsPath), 0o700); err != nil {
		t.Fatal(err)
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(`{"storage":"satellite"}`))
	if err := os.WriteFile(s.credsPath, []byte(encoded), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Load(); err == nil {
		t.Error("expected error on unknown storage backend")
	}
}
