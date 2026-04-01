package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCredentialsSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()

	creds := &Credentials{
		Username:    "testuser",
		Password:    "testpass",
		Token:       "testtoken",
		Session:     map[string]string{"session_key": "session_val"},
		credsPath:   filepath.Join(tmpDir, "creds.m"),
		sessionPath: filepath.Join(tmpDir, "session.m"),
	}

	if err := creds.Save(); err != nil {
		t.Fatal(err)
	}

	// Verify files were created
	if _, err := os.Stat(creds.credsPath); os.IsNotExist(err) {
		t.Fatal("creds file not created")
	}
	if _, err := os.Stat(creds.sessionPath); os.IsNotExist(err) {
		t.Fatal("session file not created")
	}

	// Load into new instance
	creds2 := &Credentials{
		Session:     make(map[string]string),
		credsPath:   creds.credsPath,
		sessionPath: creds.sessionPath,
	}
	creds2.Load()

	if creds2.Username != "testuser" {
		t.Errorf("expected username 'testuser', got %q", creds2.Username)
	}
	if creds2.Password != "testpass" {
		t.Errorf("expected password 'testpass', got %q", creds2.Password)
	}
	if creds2.Token != "testtoken" {
		t.Errorf("expected token 'testtoken', got %q", creds2.Token)
	}
	if creds2.Session["session_key"] != "session_val" {
		t.Errorf("expected session key, got %v", creds2.Session)
	}
}

func TestAreLoaded(t *testing.T) {
	creds := &Credentials{}
	if creds.AreLoaded() {
		t.Error("empty creds should not be loaded")
	}

	creds.Username = "u"
	creds.Password = "p"
	creds.Token = "t"
	if !creds.AreLoaded() {
		t.Error("creds with all fields should be loaded")
	}
}

func TestClean(t *testing.T) {
	tmpDir := t.TempDir()
	credsPath := filepath.Join(tmpDir, "creds.m")
	sessionPath := filepath.Join(tmpDir, "session.m")

	os.WriteFile(credsPath, []byte("data"), 0o600)
	os.WriteFile(sessionPath, []byte("data"), 0o600)

	creds := &Credentials{
		credsPath:   credsPath,
		sessionPath: sessionPath,
	}
	creds.Clean()

	if _, err := os.Stat(credsPath); !os.IsNotExist(err) {
		t.Error("creds file should be deleted")
	}
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Error("session file should be deleted")
	}
}
