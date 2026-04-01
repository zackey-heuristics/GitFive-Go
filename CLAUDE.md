# CLAUDE.md — GitFive Go Port

## Project Overview

GitFive is an OSINT tool for tracking GitHub users, originally written in Python. This repo is porting it to Go for single-binary cross-platform distribution. Tracked in #1.

## Workflow

- **Claude Code**: Planning, architecture decisions, code review orchestration
- **Codex**: Implementation, review, adversarial-review iterations
- Each implementation phase is delegated to Codex with full context (Python source + Go target path + dependencies from prior phases)

## Branch

- Working branch: `feature/1-port-to-go`

## Build & Test

```bash
go build ./cmd/gitfive          # Build binary
go test -race ./...             # Run all tests
golangci-lint run               # Lint
goreleaser release --snapshot   # Test cross-compilation
```

## Code Conventions

- Go 1.22+, modules enabled
- `internal/` for all non-main packages (not importable externally)
- `context.Context` as first parameter on all functions that do I/O or concurrency
- Errors returned up the stack; only `commands/` prints and exits
- `errgroup` + `semaphore.Weighted` for bounded concurrency (replaces Python trio)
- `goquery` for HTML parsing (replaces BeautifulSoup)
- `CGO_ENABLED=0` for static binaries — all dependencies must be pure Go
- Table-driven tests, HTML fixtures in `testdata/` directories

## Key Files (Python → Go mapping)

| Python Source | Go Target |
|---|---|
| `gitfive/config.py` | `internal/config/config.go` |
| `gitfive/lib/objects.py` | `internal/auth/`, `internal/models/`, `internal/runner/` |
| `gitfive/lib/api.py` | `internal/api/api.go` |
| `gitfive/lib/cli.py` | `cmd/gitfive/main.go` + `internal/commands/` |
| `gitfive/lib/*.py` (scrapers) | `internal/scraper/` |
| `gitfive/lib/*.py` (analysis) | `internal/analysis/` |
| `gitfive/modules/*.py` | `internal/commands/` |

## Do NOT

- Use CGO (breaks static cross-compilation)
- Add Python code or Python dependencies
- Modify original Python files (kept as reference)
