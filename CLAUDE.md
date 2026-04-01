# CLAUDE.md — GitFive-Go

## Project Overview

GitFive-Go is a Go rewrite of [GitFive](https://github.com/mxrch/GitFive), an OSINT tool for tracking GitHub users. Single static binary, cross-platform, zero runtime dependencies.

## Build & Test

```bash
make build                      # Build binary → bin/gitfive-go
make test                       # go test -race ./...
make clean                      # Remove build artifacts
CGO_ENABLED=0 go build -o bin/gitfive-go ./cmd/gitfive   # Manual build
```

## Code Conventions

- Go 1.23+ (minimum required by goquery dependency)
- `internal/` for all non-main packages (not importable externally)
- `context.Context` as first parameter on all functions that do I/O or concurrency
- Errors returned up the stack; only `internal/commands/` prints and exits
- `errgroup` + `semaphore.Weighted` for bounded concurrency
- `goquery` for HTML parsing
- `cobra` for CLI
- `CGO_ENABLED=0` for static binaries — all dependencies must be pure Go
- Table-driven tests, unit tests colocated with source (`_test.go`), E2E tests in `test/`

## Project Structure

```
cmd/gitfive/main.go          — Entry point, cobra root command
internal/
  config/                     — Constants (headers, GH Pages IPs, email domains)
  version/                    — Version string (ldflags injected at build)
  auth/                       — Credentials, GitHub login (2FA), session management
  api/                        — Multi-client GitHub API with rate-limit auto-switching
  httpclient/                 — Shared HTTP client (cookie jar, default headers)
  models/                     — Target, ContribEntry, StringSet, RepoDetails
  runner/                     — GitfiveRunner coordinator (semaphore limiters)
  scraper/                    — GitHub HTML scrapers (repos, social, commits, orgs, PEA)
  analysis/                   — xray, metamon, emails_gen, close_friends, domain_finder
  commands/                   — CLI handlers (login, user, email, emails, light)
  ui/                         — TMPrinter, banner, progress bar
  util/                       — String, domain, file, levenshtein, chunks helpers
test/                         — E2E tests (binary build + CLI smoke tests)
scripts/                      — Release script
```

## CLI Commands

```
gitfive-go login              — Authenticate to GitHub
gitfive-go user <username>    — Full reconnaissance (--json for export)
gitfive-go email <address>    — Reverse email lookup
gitfive-go emails <file> -t   — Batch email processing
gitfive-go light <username>   — Quick email discovery from commits
```

## Cross-compilation

```bash
# All platforms (via release script)
./scripts/release.sh v0.1.0

# Manual single platform
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o gitfive-go-linux-arm64 ./cmd/gitfive
```

## Open Issues

- #3 Update CLAUDE.md Go version reference
- #4 Tighten DeleteTmpDir error handling
- #5 Use OS keyring for credentials
- #6 Avoid token exposure in git clone argv
- #7 ExtractDomain should strip port numbers
- #8 CI/CD for cross-compilation and automated releases

## Do NOT

- Use CGO (breaks static cross-compilation)
- Add Python code or dependencies
