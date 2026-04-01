# AGENTS.md — Agent Collaboration Protocol

## Roles

### Claude Code (Planner / Orchestrator)
- Owns the implementation plan and phase sequencing
- Reviews Codex output and decides whether to iterate or advance
- Creates prompts for Codex with full context (Python source, Go target path, interfaces from prior phases)
- Handles architectural decisions and trade-off analysis

### Codex (Implementer / Reviewer)
- Executes implementation tasks per phase
- Performs code review and adversarial review on generated code
- Each task receives: target file path, Python source being ported, Go types/interfaces it depends on, test file to generate

## Iteration Loop

```
1. Claude Code prepares a phase prompt with full context
2. Codex implements the code + tests
3. Codex reviews its own output (self-review)
4. Claude Code runs adversarial review via Codex (separate prompt focusing on bugs, race conditions, missing edge cases)
5. If issues found → Codex fixes → back to step 3
6. If clean → Claude Code advances to next phase
```

## Phase Execution Protocol

For each phase, Claude Code provides Codex with:

1. **Goal**: What this phase accomplishes
2. **Python source**: The exact Python files being ported (read and included in prompt)
3. **Go target paths**: Where to write the Go files
4. **Dependencies**: Go types/interfaces from prior phases that this code uses
5. **Test requirements**: What tests to write and what fixtures to use
6. **Constraints**: CGO_ENABLED=0, error handling patterns, context threading

## Review Criteria

### Standard Review (after implementation)
- Correctness: Does the Go code match Python behavior?
- Idiomatic Go: proper error handling, context usage, naming conventions
- Concurrency safety: mutex usage, channel safety, race conditions
- No CGO dependencies

### Adversarial Review (separate pass)
- Race conditions under concurrent access
- Resource leaks (HTTP clients, file handles, goroutines)
- Error paths that silently swallow failures
- Missing context cancellation checks
- Hardcoded values that should be configurable
- Security: credential handling, input sanitization

## Phase Order (dependency chain)

```
Phase 0: Scaffolding (no deps)
Phase 1: Config + Utils (no deps)
Phase 2: HTTP Client + UI (no deps)
Phase 3: Auth (depends on: Phase 2)
Phase 4: API (depends on: Phase 2, 3)
Phase 5: Models + Runner (depends on: Phase 2, 3, 4)
Phase 6: Scrapers (depends on: Phase 2, 4, 5)
Phase 7: Analysis (depends on: Phase 5, 6)
Phase 8: Commands/CLI (depends on: all above)
Phase 9: Image + Version (depends on: Phase 2)
Phase 10: Polish + Release (depends on: all above)
```
