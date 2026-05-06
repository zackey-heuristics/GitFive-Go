package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCredentialsSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()

	creds := &Credentials{
		Username:  "testuser",
		Token:     "github_pat_testtoken",
		credsPath: filepath.Join(tmpDir, "creds.m"),
	}

	if err := creds.Save(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(creds.credsPath); os.IsNotExist(err) {
		t.Fatal("creds file not created")
	}

	creds2 := &Credentials{
		credsPath: creds.credsPath,
	}
	creds2.Load()

	if creds2.Username != "testuser" {
		t.Errorf("expected username 'testuser', got %q", creds2.Username)
	}
	if creds2.Token != "github_pat_testtoken" {
		t.Errorf("expected token 'github_pat_testtoken', got %q", creds2.Token)
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
	tmpDir := t.TempDir()
	credsPath := filepath.Join(tmpDir, "creds.m")

	if err := os.WriteFile(credsPath, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	creds := &Credentials{
		credsPath: credsPath,
	}
	if err := creds.Clean(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(credsPath); !os.IsNotExist(err) {
		t.Error("creds file should be deleted")
	}
}

func TestCleanIgnoresMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	creds := &Credentials{
		credsPath: filepath.Join(tmpDir, "missing.m"),
	}
	if err := creds.Clean(); err != nil {
		t.Errorf("Clean on missing file should be a no-op, got %v", err)
	}
}
