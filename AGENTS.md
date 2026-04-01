# AGENTS.md — Agent Collaboration Protocol

## Roles

### Claude Code (Planner / Orchestrator)
- Owns architecture decisions and implementation planning
- Reviews Codex output and decides whether to iterate or ship
- Creates GitHub Issues and PRs
- Handles code review triage (Copilot review analysis, adversarial review)

### Codex (Implementer / Reviewer)
- Executes implementation tasks with full context
- Performs adversarial review on generated code
- Each task receives: goal, target file paths, existing code context, test requirements

## Workflow

```
1. Claude Code plans the task and prepares context
2. Claude Code or Codex implements the code + tests
3. Run tests: go test -race ./internal/... ./test/...
4. Codex adversarial review (/codex:adversarial-review)
5. If issues found → fix → back to step 3
6. If clean → commit, push, PR
```

## Review Criteria

### Standard Review
- Correctness: Does the code work as intended?
- Idiomatic Go: error handling, context usage, naming conventions
- Concurrency safety: mutex usage, race conditions
- No CGO dependencies
- Tests pass with -race

### Adversarial Review
- Race conditions under concurrent access
- Resource leaks (HTTP clients, file handles, goroutines)
- Error paths that silently swallow failures
- Missing context cancellation checks
- Security: credential handling, token exposure, input sanitization

## Release Process

1. All tests pass (CI runs automatically on push)
2. Tag the release: `git tag v<version> && git push origin v<version>`
3. GitHub Actions triggers GoReleaser — builds binaries and creates GitHub Release
4. Target platforms: linux/amd64, linux/arm64, darwin/arm64, windows/amd64, windows/arm64
