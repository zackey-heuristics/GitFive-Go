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

- Go 1.25+ (minimum version per go.mod)
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
scripts/                      — Release helper script
.github/workflows/ci.yaml    — CI: lint, test, build, cross-compile on push/PR
.github/workflows/release.yaml — Release: GoReleaser on v* tag push
.goreleaser.yaml              — GoReleaser configuration
```

## CLI Commands

```
gitfive-go login              — Authenticate to GitHub
gitfive-go user <username>    — Full reconnaissance (--json for export)
gitfive-go email <address>    — Reverse email lookup
gitfive-go emails <file> -t   — Batch email processing
gitfive-go light <username>   — Quick email discovery from commits
```

## Cross-compilation & Releases

Releases are automated via GitHub Actions + GoReleaser:

```bash
# Tag a release (triggers CI → GoReleaser → GitHub Release)
git tag v0.1.0
git push origin v0.1.0

# Manual single platform build
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o gitfive-go-linux-arm64 ./cmd/gitfive
```

Target platforms: linux/amd64, linux/arm64, darwin/arm64, windows/amd64, windows/arm64

## CI/CD

- **CI** (`ci.yaml`): Runs on every push and PR — lint (golangci-lint v2), test, build, cross-compile
- **Release** (`release.yaml`): Runs on `v*` tag push — lint, test, then GoReleaser builds and publishes GitHub Release

## Open Issues

- #4 Tighten DeleteTmpDir error handling
- #5 Use OS keyring for credentials
- #6 Avoid token exposure in git clone argv
- #7 ExtractDomain should strip port numbers

## Do NOT

- Use CGO (breaks static cross-compilation)
- Add Python code or dependencies
