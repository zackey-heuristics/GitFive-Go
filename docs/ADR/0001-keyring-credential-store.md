# ADR-0001: Use OS keyring for fine-grained PAT storage with file-based fallback

## Status

Proposed (will move to **Accepted** upon merge of the PR that closes Issue #5).

## Context

GitFive-Go authenticates to the GitHub REST API and to `git` operations using a
fine-grained Personal Access Token (PAT). After PR #14 the PAT is the *only*
secret the application stores — username, password, and 2FA flow have been
removed.

Through v0.1.1, the PAT was persisted as base64-encoded JSON at
`~/.gitfive_go/creds.m`, with file mode `0600`. This was inherited from the
Python upstream and was never meaningful protection: any same-user process can
read the file, any backup tool that includes dotfiles captures the secret in
plaintext-equivalent form, and the obfuscation discourages neither casual
inspection nor automated credential scrapers.

Issue #5 asks for a real OS-level secret store. The fine-grained PAT migration
makes the upgrade more valuable: the token now has `Administration: write` on
all repositories owned by the user, so a compromise has higher blast radius
than the previous classic PAT did.

Constraints we must honor:

- **Pure Go, no CGO.** The project ships static binaries with `CGO_ENABLED=0`
  for cross-compilation; any keyring library that pulls in CGO is disqualified.
- **Cross-platform.** The release matrix covers macOS (arm64, amd64), Linux
  (arm64, amd64), and Windows (arm64, amd64). The same code path must work on
  all six.
- **Headless / CI compatibility.** The tool must remain usable on hosts where
  no OS keyring is available — Docker containers without a D-Bus session,
  SSH-only servers, sandboxed CI runners. Forcing keyring everywhere would
  break those use cases.
- **Existing v0.1.1 users must not be locked out.** Their on-disk creds files
  must continue to read until the user re-authenticates.

## Decision

Adopt **`github.com/zalando/go-keyring`** as the primary credential backend,
with a base64-encoded file as the fallback / opt-in alternative.

**Why `zalando/go-keyring`:**

- Pure Go on all three platforms (`/usr/bin/security` shell-out on macOS,
  `godbus/dbus` pure-Go D-Bus on Linux, `syscall` on Windows). Compatible with
  `CGO_ENABLED=0`.
- Single small API surface (`Set` / `Get` / `Delete`).
- Has a built-in mock (`keyring.MockInit`, `keyring.MockInitWithError`) that
  allows deterministic unit tests without touching the real OS store.
- Actively maintained at the time of writing.

### Storage schema

The on-disk file (`~/.gitfive_go/creds.m`, base64-encoded JSON) carries a
`storage` marker telling readers where the token actually lives:

```json
// keyring backend (default)
{"storage": "keyring"}

// file backend (opt-in via --use-file-storage, or auto-fallback)
{"storage": "file", "token": "github_pat_..."}
```

Legacy v0.1.1 files (no `storage` field, but a `token` field) are read
transparently as the file backend; a subsequent `Save` rewrites them in the new
format.

The username field that previous versions persisted is dropped: it is rederived
from `GET /user` on every `CheckToken` call, so persisting it added no value
and produced stale data after token rotation.

### Backend selection

- **Default**: keyring. The login flow attempts `keyring.Set`. On success the
  token is in the OS keyring and the file holds only a marker.
- **Auto-fallback**: if `keyring.Set` returns an error (no D-Bus session,
  Wincred unavailable, etc.) the login flow falls back to the file backend
  with a one-line stderr warning so the user is not left silently confused on
  a host that lacks keyring support.
- **Explicit opt-in**: `gitfive-go login --use-file-storage` skips the keyring
  attempt entirely and always writes file-backed credentials. Useful for
  hosts where the keyring exists but the user prefers file storage (e.g.
  for portability or to back up creds.m alongside other dotfiles).

### Token rotation / cleanup

- Every `Save` overwrites the entry on whichever backend is selected. The
  *other* backend is best-effort cleared in the same call, so a user who
  switches between keyring and file modes never leaves a recoverable token
  behind on the unused backend.
- `gitfive-go login --clean` removes both the keyring entry and the on-disk
  file regardless of which was last used.
- The existing 30-day expiry warning (PR #14) still prompts the user to
  rotate before the PAT expires. The new `Save` flow guarantees the rotation
  replaces the old token rather than leaving it adjacent.

### Keyring identifiers

Service: `gitfive-go`. Account: `fine-grained-pat`. Single fixed key — the
tool does not support multiple PATs per host.

## Consequences

### Positive

- The PAT is no longer trivially recoverable from a stolen home-directory
  backup or a same-user process casually inspecting `~/.gitfive_go/`.
- Aligns with mainstream macOS / Windows / Linux desktop expectations: the
  Keychain / Credential Manager / Secret Service is where third-party CLIs
  are expected to store credentials.
- Token rotation is now safer: stale tokens are explicitly purged from the
  unused backend on every `Save`, so a leaked token cannot survive in a
  forgotten file after the user re-logs in.
- Username is no longer persisted, eliminating one stale-data class.

### Negative / risks

- An additional dependency (`zalando/go-keyring` plus its transitive deps:
  `godbus/dbus/v5`, `danieljoos/wincred`). All pure Go and static-build
  compatible, but the supply-chain surface grows.
- Two storage backends mean two code paths. We keep this manageable by hiding
  the choice behind `internal/credstore.Store` and a single `storage` marker.
- macOS keychain access via `/usr/bin/security` shells out for every read
  and write. This is fine for our access pattern (tens of operations per
  invocation) but is observably slower than a CGO-based binding would be.
- Same-user attackers who can already execute code as the GitFive-Go user
  retain a path to the token (they can call `keyring.Get` themselves). The
  keyring raises the bar against backup-and-extract scenarios but does not
  defend against an attacker who is already running on the box as our user.
  This is consistent with how other CLI tools that use the OS keyring frame
  their threat model.

### Follow-ups

- If usage patterns warrant, swap the macOS `/usr/bin/security` shell-out for
  a direct Keychain Services binding (would require CGO; out of scope here).
- Document the OS-specific GUI surface (Keychain Access, `seahorse`, Windows
  Credential Manager) in a future user-facing README section so users can
  inspect / revoke the entry by hand.

## References

- Issue: [#5](https://github.com/zackey-heuristics/GitFive-Go/issues/5)
- Library: <https://github.com/zalando/go-keyring>
- Prior work removing username/password and adopting fine-grained PATs: PR #14
- Prior work routing git auth through `GIT_ASKPASS` (so this ADR's secret
  never touches argv): PR #15
