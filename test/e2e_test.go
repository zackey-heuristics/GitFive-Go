package test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// binaryPath builds the gitfive-go binary and returns its path.
func binaryPath(t *testing.T) string {
	t.Helper()
	name := "gitfive-go"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(t.TempDir(), name)
	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", binary, "../cmd/gitfive")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return binary
}

func TestBinaryBuilds(t *testing.T) {
	_ = binaryPath(t)
}

func TestVersionFlag(t *testing.T) {
	bin := binaryPath(t)
	out, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("--version failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "gitfive-go version") {
		t.Errorf("expected version output, got: %s", out)
	}
}

func TestHelpOutput(t *testing.T) {
	bin := binaryPath(t)
	out, err := exec.Command(bin, "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("--help failed: %v\n%s", err, out)
	}
	output := string(out)
	for _, cmd := range []string{"login", "user", "email", "emails", "light"} {
		if !strings.Contains(output, cmd) {
			t.Errorf("expected %q in help output", cmd)
		}
	}
}

func TestLoginHelp(t *testing.T) {
	bin := binaryPath(t)
	out, err := exec.Command(bin, "login", "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("login --help failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "--clean") {
		t.Error("expected --clean flag in login help")
	}
}

func TestUserHelp(t *testing.T) {
	bin := binaryPath(t)
	out, err := exec.Command(bin, "user", "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("user --help failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "--json") {
		t.Error("expected --json flag in user help")
	}
}

func TestEmailHelp(t *testing.T) {
	bin := binaryPath(t)
	out, err := exec.Command(bin, "email", "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("email --help failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "email_address") {
		t.Error("expected email_address arg in email help")
	}
}

func TestEmailsHelp(t *testing.T) {
	bin := binaryPath(t)
	out, err := exec.Command(bin, "emails", "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("emails --help failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "-t") {
		t.Error("expected -t flag")
	}
}

func TestLightHelp(t *testing.T) {
	bin := binaryPath(t)
	out, err := exec.Command(bin, "light", "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("light --help failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "username") {
		t.Error("expected username arg in light help")
	}
}

func TestUserMissingArgs(t *testing.T) {
	bin := binaryPath(t)
	cmd := exec.Command(bin, "user")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Error("expected error when username not provided")
	}
	if !strings.Contains(string(out), "accepts 1 arg") {
		t.Errorf("expected arg error message, got: %s", out)
	}
}

func TestUnknownCommand(t *testing.T) {
	bin := binaryPath(t)
	cmd := exec.Command(bin, "nonexistent")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Error("expected error for unknown command")
	}
	if !strings.Contains(string(out), "unknown command") {
		t.Errorf("expected unknown command message, got: %s", out)
	}
}
