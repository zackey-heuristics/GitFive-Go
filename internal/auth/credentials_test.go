package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"
)

// redirectHome redirects HOME (and USERPROFILE for Windows) so
// NewCredentials anchors the store under a per-test temp directory and
// resets the keyring mock so prior tests don't bleed in.
func redirectHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	keyring.MockInit()
	return home
}

func TestCredentialsSaveLoad(t *testing.T) {
	home := redirectHome(t)

	creds, err := NewCredentials()
	if err != nil {
		t.Fatal(err)
	}
	creds.Username = "testuser" // not persisted, just validates Save tolerates it
	creds.Token = "github_pat_testtoken"

	if err := creds.Save(); err != nil {
		t.Fatal(err)
	}

	credsPath := filepath.Join(home, ".gitfive_go", "creds.m")
	if _, err := os.Stat(credsPath); os.IsNotExist(err) {
		t.Fatal("creds file not created")
	}

	creds2, err := NewCredentials()
	if err != nil {
		t.Fatal(err)
	}
	creds2.Load()
	if creds2.Token != "github_pat_testtoken" {
		t.Errorf("expected token 'github_pat_testtoken', got %q", creds2.Token)
	}
	// Username is intentionally NOT persisted; CheckToken repopulates it
	// from `GET /user` on every login. Confirm that.
	if creds2.Username != "" {
		t.Errorf("Username must not be persisted, got %q", creds2.Username)
	}
}

func TestAreLoaded(t *testing.T) {
	creds := &Credentials{}
	if creds.AreLoaded() {
		t.Error("empty creds should not be loaded")
	}

	creds.Token = "github_pat_xxx"
	if !creds.AreLoaded() {
		t.Error("creds with a token should be loaded")
	}
}

func TestClean(t *testing.T) {
	home := redirectHome(t)

	creds, err := NewCredentials()
	if err != nil {
		t.Fatal(err)
	}
	creds.Token = "github_pat_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	if err := creds.Save(); err != nil {
		t.Fatal(err)
	}

	if err := creds.Clean(); err != nil {
		t.Fatal(err)
	}

	credsPath := filepath.Join(home, ".gitfive_go", "creds.m")
	if _, err := os.Stat(credsPath); !os.IsNotExist(err) {
		t.Error("creds file should be deleted by Clean")
	}
}

func TestCleanIgnoresMissingFile(t *testing.T) {
	redirectHome(t)
	creds, err := NewCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if err := creds.Clean(); err != nil {
		t.Errorf("Clean on missing file should be a no-op, got %v", err)
	}
}
