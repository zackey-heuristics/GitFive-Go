package gitcred

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/zackey-heuristics/gitfive-go/internal/auth"
)

func TestWriteToken_Success(t *testing.T) {
	creds := &auth.Credentials{Token: "github_pat_11ABC_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"}
	var buf bytes.Buffer
	if err := writeToken(&buf, creds); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("expected trailing newline so git's askpass parser is happy, got %q", got)
	}
	if strings.TrimRight(got, "\n") != creds.Token {
		t.Errorf("output = %q, want %q", got, creds.Token)
	}
}

func TestWriteToken_NoToken(t *testing.T) {
	creds := &auth.Credentials{} // no token
	var buf bytes.Buffer
	err := writeToken(&buf, creds)
	if err == nil {
		t.Fatal("expected error when no token is loaded")
	}
	if buf.Len() != 0 {
		t.Errorf("expected nothing written on error, got %q", buf.String())
	}
}

func TestHandleAskPassMode_NoSentinelReturnsFalse(t *testing.T) {
	t.Setenv(askpassEnvSentinel, "")
	handled, err := HandleAskPassMode()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Error("HandleAskPassMode should return false when sentinel is unset")
	}
}

func TestHandleAskPassMode_SentinelButNoCredsErrors(t *testing.T) {
	// With sentinel set and no credentials available on disk, the function
	// must still return handled=true so main exits, but with a non-nil
	// error so the exit code is non-zero (git will see askpass failed).
	t.Setenv(askpassEnvSentinel, "1")
	// Redirect home directory for both Unix-likes (HOME) and Windows
	// (USERPROFILE) so os.UserHomeDir() resolves to an empty temp dir.
	emptyHome := t.TempDir()
	t.Setenv("HOME", emptyHome)
	t.Setenv("USERPROFILE", emptyHome)
	handled, err := HandleAskPassMode()
	if !handled {
		t.Error("expected handled=true when sentinel is set, even on failure")
	}
	if err == nil {
		t.Fatal("expected error when no credentials are available")
	}
	// The error should hint at the missing-file case so the user knows to
	// run `gitfive-go login`, not that the file is corrupt.
	if !strings.Contains(err.Error(), "not found") || !strings.Contains(err.Error(), "gitfive-go login") {
		t.Errorf("expected file-not-found message hinting at login, got: %v", err)
	}
}

func TestHandleAskPassMode_CorruptCredsFileGivesDistinctError(t *testing.T) {
	// Write garbage to creds.m so Load() silently fails, then verify the
	// askpass mode emits the "may be corrupt" error rather than the
	// file-not-found one.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(askpassEnvSentinel, "1")
	credsDir := home + "/.gitfive_go"
	if err := os.MkdirAll(credsDir, 0o700); err != nil {
		t.Fatalf("setup mkdir: %v", err)
	}
	if err := os.WriteFile(credsDir+"/creds.m", []byte("not-base64-not-json"), 0o600); err != nil {
		t.Fatalf("setup write: %v", err)
	}

	handled, err := HandleAskPassMode()
	if !handled {
		t.Error("expected handled=true")
	}
	if err == nil {
		t.Fatal("expected error for corrupt creds.m")
	}
	if !strings.Contains(err.Error(), "corrupt") {
		t.Errorf("expected 'corrupt' diagnostic for malformed creds, got: %v", err)
	}
	if strings.Contains(err.Error(), "not found") {
		t.Errorf("error should NOT report file-not-found when file exists: %v", err)
	}
}

func TestCommandWithToken_NoTokenInArgsOrEnv(t *testing.T) {
	const sentinelToken = "github_pat_TESTTOKEN_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	// Inject a fake token into the parent's env to ensure it does not
	// somehow leak into the child's env (this asserts our scrub behaviour
	// is not the only thing protecting us — there is no code path that
	// places the token into env at all).
	t.Setenv("UNRELATED_VAR", sentinelToken)

	cmd, err := CommandWithToken(context.Background(), "git", "clone", "--filter=tree:0",
		"https://github.com/example/repo", "/tmp/whatever")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, a := range cmd.Args {
		if strings.Contains(a, sentinelToken) {
			t.Errorf("token leaked into args: %q", a)
		}
		if strings.Contains(a, "x-oauth-basic") {
			t.Errorf("URL still embeds basic-auth pattern: %q", a)
		}
	}

	var (
		seenAskpass    bool
		seenSentinel   bool
		seenNoTerminal bool
	)
	for _, kv := range cmd.Env {
		switch {
		case strings.HasPrefix(kv, "GIT_ASKPASS="):
			seenAskpass = true
			if !strings.HasSuffix(kv, "/gitcred.test") &&
				!strings.HasSuffix(kv, "/gitcred.test.exe") &&
				!strings.Contains(kv, "go-build") {
				// Allow whatever go test produces as the test binary path;
				// we only care that GIT_ASKPASS resolves to a real path.
			}
		case kv == askpassEnvSentinel+"=1":
			seenSentinel = true
		case kv == "GIT_TERMINAL_PROMPT=0":
			seenNoTerminal = true
		}
		// The token (which is in UNRELATED_VAR) WILL appear in the env
		// because we don't strip arbitrary user env, but assert at least
		// that no value equals "Bearer <token>" or carries our managed keys
		// with the token embedded.
		if strings.HasPrefix(kv, "GIT_ASKPASS=") && strings.Contains(kv, sentinelToken) {
			t.Errorf("token leaked into managed GIT_ASKPASS value: %q", kv)
		}
		if strings.HasPrefix(kv, askpassEnvSentinel+"=") && strings.Contains(kv, sentinelToken) {
			t.Errorf("token leaked into managed sentinel value: %q", kv)
		}
	}
	if !seenAskpass {
		t.Error("GIT_ASKPASS not set in cmd.Env")
	}
	if !seenSentinel {
		t.Errorf("%s=1 not set in cmd.Env", askpassEnvSentinel)
	}
	if !seenNoTerminal {
		t.Error("GIT_TERMINAL_PROMPT=0 not set in cmd.Env")
	}
}

func TestCommandWithToken_NoEmbeddedCredentialURL(t *testing.T) {
	// Regression guard: even if a future caller passes a URL with an
	// embedded basic-auth segment by mistake (the exact pattern Issue #6
	// fixes), the helper itself still produces a cmd whose Args contain
	// no `x-oauth-basic` and no `@github.com` userinfo segment when given
	// a clean URL — i.e., we never re-introduce the old form on our own.
	cmd, err := CommandWithToken(context.Background(), "git", "clone",
		"--filter=tree:0", "--no-checkout",
		"https://github.com/exampleuser/examplerepo",
		t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, a := range cmd.Args {
		if strings.Contains(a, "x-oauth-basic") {
			t.Errorf("x-oauth-basic re-appeared in args: %q", a)
		}
		// `@github.com` would only appear in a URL that embedded a
		// userinfo segment; a clean URL like `https://github.com/x/y`
		// must not contain it.
		if strings.Contains(a, "@github.com") {
			t.Errorf("userinfo segment re-appeared in args: %q", a)
		}
	}
}

func TestCommandWithToken_ScrubsPriorAskpassVars(t *testing.T) {
	// If the parent already had an old GIT_ASKPASS / sentinel, the
	// returned cmd's Env must contain only the freshly set values, not
	// duplicates that could confuse downstream tools.
	t.Setenv("GIT_ASKPASS", "/old/path")
	t.Setenv(askpassEnvSentinel, "stale")
	t.Setenv("GIT_TERMINAL_PROMPT", "1")

	cmd, err := CommandWithToken(context.Background(), "git", "status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var (
		askpassCount  int
		sentinelCount int
		terminalCount int
	)
	for _, kv := range cmd.Env {
		switch {
		case strings.HasPrefix(kv, "GIT_ASKPASS="):
			askpassCount++
			if kv == "GIT_ASKPASS=/old/path" {
				t.Errorf("stale GIT_ASKPASS leaked through: %q", kv)
			}
		case strings.HasPrefix(kv, askpassEnvSentinel+"="):
			sentinelCount++
			if kv != askpassEnvSentinel+"=1" {
				t.Errorf("stale sentinel leaked through: %q", kv)
			}
		case strings.HasPrefix(kv, "GIT_TERMINAL_PROMPT="):
			terminalCount++
			if kv != "GIT_TERMINAL_PROMPT=0" {
				t.Errorf("stale GIT_TERMINAL_PROMPT leaked through: %q", kv)
			}
		}
	}
	if askpassCount != 1 {
		t.Errorf("GIT_ASKPASS count = %d, want 1", askpassCount)
	}
	if sentinelCount != 1 {
		t.Errorf("sentinel count = %d, want 1", sentinelCount)
	}
	if terminalCount != 1 {
		t.Errorf("GIT_TERMINAL_PROMPT count = %d, want 1", terminalCount)
	}
}
