# do-more Design Document

**Date:** 2026-02-14
**Status:** Approved

## Overview

do-more is a Go CLI that runs an autonomous while-loop, feeding tasks from a PRD (JSON) to AI coding providers one at a time until all tasks pass configurable quality gates. Inspired by the [ralph pattern](https://github.com/snarktank/ralph).

## Core Concepts

- **Provider** — an AI coding tool (Claude Code, OpenCode, Kimi CLI). Implements a Go interface. Each gets invoked with a prompt and returns when done.
- **PRD** — a `do-more.json` file listing tasks with status, acceptance criteria, and learnings.
- **Gate** — a shell command that must exit 0 for a task to be marked complete (e.g., `go test ./...`).
- **Iteration** — one provider invocation on one task. Fresh context each time.
- **Loop** — the outer while loop: pick next incomplete task, invoke provider, run gates, mark done or retry.

## Architecture

**Approach:** Flat package. All Go code in a single `main` package. No `cmd/`, no `internal/`. Grow into packages only when complexity demands it.

**Concurrency:** Sequential only. One provider works on one task at a time.

**Extensibility:** Go interface + registry. New providers implement the `Provider` interface and register themselves.

## Provider Interface

```go
type Provider interface {
    Name() string
    Run(ctx context.Context, prompt string, workDir string) (string, error)
}
```

Built-in providers at launch:
- **Claude Code** — `claude -p "<prompt>" --output-format text`
- **OpenCode** — `opencode -p "<prompt>"`
- **Kimi CLI** — `kimi "<prompt>"`

Registration via map:

```go
var providers = map[string]Provider{
    "claude":   &ClaudeProvider{},
    "opencode": &OpenCodeProvider{},
    "kimi":     &KimiProvider{},
}
```

## Task File Format (do-more.json)

```json
{
  "name": "my-project",
  "provider": "claude",
  "branch": "feat/my-feature",
  "gates": [
    "go test ./...",
    "golangci-lint run"
  ],
  "maxIterations": 20,
  "tasks": [
    {
      "id": "1",
      "title": "Add user authentication endpoint",
      "description": "Create POST /api/auth/login that accepts email+password and returns a JWT.",
      "status": "done",
      "learnings": "Used golang-jwt/jwt/v5. Token expiry set to 24h."
    },
    {
      "id": "2",
      "title": "Add input validation middleware",
      "description": "Validate request bodies using go-playground/validator.",
      "status": "pending",
      "learnings": ""
    }
  ]
}
```

**Task statuses:** `pending` | `in_progress` | `done` | `failed`

## The Loop

```
do-more run [--provider claude] [--config do-more.json]

1. Load do-more.json
2. Create/checkout branch
3. while (has pending tasks):
   a. Pick next pending task -> in_progress
   b. Build prompt (task + learnings + gate failures if retry)
   c. Invoke provider.Run(prompt, workDir)
   d. Run all gates
   e. If gates pass:
      - Mark task "done"
      - Git commit
      - Save learnings to do-more.json
   f. If gates fail:
      - Increment retry counter
      - If retries >= maxIterations: mark "failed", move to next task
      - Otherwise: loop back to (c) with gate failure output in prompt
4. Print summary
```

## Prompt Template

```
You are working on the following task:

## Task: {{.Title}}
{{.Description}}

## Previous Learnings
{{.Learnings}}

## Gate Failures (if retry)
{{.GateOutput}}

## Instructions
- Work in the current directory
- Make the minimal changes needed
- When done, the following gates will be checked:
{{range .Gates}}- {{.}}
{{end}}
```

## CLI Commands

```
do-more init                    # Create a do-more.json template
do-more run                     # Start the loop
do-more run --provider opencode # Override provider
do-more run --max-iterations 10 # Override max iterations
do-more status                  # Show task status summary
do-more providers               # List available providers
```

Output format:

```
[do-more] Starting with provider: claude
[do-more] Branch: feat/my-feature
[do-more] -- Iteration 1/20 -- Task #2: Add input validation middleware
[do-more] Invoking claude...
[do-more] Provider finished (45s)
[do-more] Running gate: go test ./...  OK
[do-more] Running gate: golangci-lint run  OK
[do-more] Task #2: done
[do-more] -- Summary --
[do-more] 2/2 tasks done. All complete!
```

## File Structure

```
do-more/
  main.go           # CLI entry point (cobra commands)
  loop.go           # Core loop logic
  loop_test.go      # Loop tests
  provider.go       # Provider interface + registry
  provider_test.go  # Provider tests (with mocks)
  claude.go         # Claude Code provider
  opencode.go       # OpenCode provider
  kimi.go           # Kimi CLI provider
  config.go         # do-more.json loading/saving
  config_test.go    # Config tests
  gate.go           # Gate runner
  gate_test.go      # Gate tests
  prompt.go         # Prompt template builder
  prompt_test.go    # Prompt tests
  go.mod
  go.sum
  docs/
    plans/
```

## Testing Strategy

- Provider interface enables mock providers for loop tests
- Gate runner tested with real shell commands (`echo`, `true`, `false`)
- Config tested with JSON fixtures
- No integration tests against real AI providers (too slow, non-deterministic)
- TDD: write failing test first, then implement

## Key Design Decisions

1. **Flat package** — YAGNI. Restructure when complexity demands it.
2. **Sequential execution** — avoid git conflicts and complexity. Parallelism can be added later.
3. **Go interface for providers** — compile-time extensibility, no dynamic plugins.
4. **JSON for task state** — simple, git-friendly, machine-readable.
5. **Configurable gates** — user defines quality checks, not hardcoded.
6. **Fresh context per iteration** — like ralph, each provider invocation starts clean.
7. **Learnings persist** — gate failures and discoveries carry forward via do-more.json.
