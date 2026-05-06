// Package gitcred wires a `GIT_ASKPASS` flow that lets the gitfive-go binary
// authenticate child `git` processes without ever placing the token in argv
// or the spawned environment. The askpass child reloads the token from
// `~/.gitfive_go/creds.m` itself, so the parent only has to set a sentinel
// env var.
package gitcred

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/zackey-heuristics/gitfive-go/internal/auth"
)

// askpassEnvSentinel is the env var the parent sets to tell the child
// "you were invoked by us as a GIT_ASKPASS helper". It carries no secret —
// just a flag.
const askpassEnvSentinel = "GITFIVE_ASKPASS"

// HandleAskPassMode short-circuits startup when this binary was invoked as
// the GIT_ASKPASS helper. It loads credentials from disk, prints the token
// to stdout, and signals the caller to exit. The caller (main) should
// `os.Exit(0)` when this returns true and not proceed to cobra parsing.
//
// Any failure during credential loading is treated as a hard error and
// returns a non-nil error so main can exit non-zero — git will then surface
// an authentication failure rather than appearing to have an empty password.
func HandleAskPassMode() (handled bool, err error) {
	if os.Getenv(askpassEnvSentinel) != "1" {
		return false, nil
	}
	creds, err := auth.NewCredentials()
	if err != nil {
		return true, fmt.Errorf("askpass: locate credentials: %w", err)
	}
	creds.Load()
	if !creds.AreLoaded() {
		// Distinguish "user never logged in" from "creds file is corrupt"
		// — both yield an empty token after Load(), but the user-facing
		// fix is different.
		if _, statErr := os.Stat(creds.CredsPath()); os.IsNotExist(statErr) {
			return true, fmt.Errorf("askpass: credentials file not found at %s — run 'gitfive-go login' first", creds.CredsPath())
		}
		return true, fmt.Errorf("askpass: token could not be read from %s (file may be corrupt)", creds.CredsPath())
	}
	return true, writeToken(os.Stdout, creds)
}

// writeToken is the testable core of askpass mode: given an already-loaded
// *Credentials, emit the token (with trailing newline so git's askpass
// parser is happy). Callers must ensure the token is present; an empty
// token here returns an error rather than silently writing a blank line
// (which git would treat as a "valid" empty password and then 401 on).
func writeToken(w io.Writer, creds *auth.Credentials) error {
	if !creds.AreLoaded() {
		return fmt.Errorf("askpass: no token loaded from %s", creds.CredsPath())
	}
	if _, err := fmt.Fprintln(w, creds.Token); err != nil {
		return fmt.Errorf("askpass: write token: %w", err)
	}
	return nil
}

// CommandWithToken returns an `*exec.Cmd` configured to run `name args...`
// with GIT_ASKPASS pointing at this binary. The token is NOT placed in the
// returned cmd's Args or Env; the askpass child loads it from creds.m.
//
// Callers are expected to additionally set `cmd.Dir` if needed and capture
// stdout/stderr; this function only handles the credential plumbing.
func CommandWithToken(ctx context.Context, name string, args ...string) (*exec.Cmd, error) {
	selfPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locate self for GIT_ASKPASS: %w", err)
	}

	cmd := exec.CommandContext(ctx, name, args...)
	// Build env from a copy of the parent's environment, but drop any prior
	// askpass sentinel so we never accidentally re-enter askpass mode in
	// nested invocations.
	parentEnv := os.Environ()
	env := make([]string, 0, len(parentEnv)+3)
	for _, kv := range parentEnv {
		if isAskpassEnvKey(kv) {
			continue
		}
		env = append(env, kv)
	}
	env = append(env,
		"GIT_ASKPASS="+selfPath,
		askpassEnvSentinel+"=1",
		// Stop git from falling back to a TTY prompt if askpass somehow
		// fails — surface the failure as a non-zero exit instead of
		// blocking indefinitely on stdin.
		"GIT_TERMINAL_PROMPT=0",
	)
	cmd.Env = env
	return cmd, nil
}

// isAskpassEnvKey reports whether `kv` is one of the env vars CommandWithToken
// manages. Used to scrub the parent's environment before re-injection.
//
// Each prefix includes the trailing "=" separator, so an unrelated key like
// `GIT_ASKPASS_HELPER_PATH=...` is NOT matched (its 12th character is `_`,
// not `=`). This avoids accidentally dropping unrelated env vars whose names
// happen to share a prefix.
func isAskpassEnvKey(kv string) bool {
	for _, prefix := range []string{
		askpassEnvSentinel + "=",
		"GIT_ASKPASS=",
		"GIT_TERMINAL_PROMPT=",
	} {
		if len(kv) >= len(prefix) && kv[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}
