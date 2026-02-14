# do-more

Autonomous AI coding loop orchestrator. Runs AI coding providers (Claude Code, OpenCode, Kimi CLI) in a while-loop until all tasks pass configurable quality gates.

## Build

```bash
go build -o do-more .
```

## Quick Start

### 1. Initialize a project

```bash
cd your-project
do-more init
```

This creates a `do-more.json` template:

```json
{
  "name": "your-project",
  "provider": "claude",
  "branch": "feat/do-more",
  "gates": [],
  "maxIterations": 10,
  "tasks": [
    {
      "id": "1",
      "title": "Example task",
      "description": "Describe what needs to be done",
      "status": "pending",
      "learnings": ""
    }
  ]
}
```

### 2. Configure your tasks and gates

Edit `do-more.json` to define your actual work:

```json
{
  "name": "my-api",
  "provider": "claude",
  "branch": "feat/user-auth",
  "gates": ["go test ./...", "golangci-lint run"],
  "maxIterations": 5,
  "tasks": [
    {
      "id": "1",
      "title": "Add login endpoint",
      "description": "Create POST /api/login that accepts email and password, validates credentials, and returns a JWT token.",
      "status": "pending",
      "learnings": ""
    },
    {
      "id": "2",
      "title": "Add signup endpoint",
      "description": "Create POST /api/signup that accepts email and password, hashes password with bcrypt, and stores user.",
      "status": "pending",
      "learnings": ""
    }
  ]
}
```

**Fields:**

| Field | Description |
|-------|-------------|
| `name` | Project name |
| `provider` | AI provider to use: `claude`, `opencode`, or `kimi` |
| `branch` | Git branch name (informational) |
| `gates` | Shell commands that must all pass for a task to be "done" |
| `maxIterations` | Max retry attempts per task before marking it failed |
| `tasks` | List of tasks to complete |

**Task statuses:** `pending` → `in_progress` → `done` or `failed`

### 3. Run the loop

```bash
do-more run
```

For each pending task, do-more will:
1. Build a prompt with the task description, gates, and any learnings from previous attempts
2. Send it to the configured AI provider
3. Run all gate commands to verify the work
4. If gates pass → mark task `done`, move to next task
5. If gates fail → feed failure output back to the provider and retry
6. If max iterations reached → mark task `failed`

### 4. Check status

```bash
do-more status
```

Output:

```
Project: my-api
Provider: claude
Branch: feat/user-auth
Gates: go test ./..., golangci-lint run

  [✓] #1 Add login endpoint (done)
  [→] #2 Add signup endpoint (in_progress)
```

## CLI Reference

```bash
do-more init                          # Create do-more.json template
do-more run                           # Start the autonomous loop
do-more run --provider opencode       # Override provider
do-more run --max-iterations 20       # Override max iterations
do-more run --config path/to/file.json  # Use custom config path
do-more status                        # Show task status
do-more providers                     # List available providers
```

## Available Providers

| Provider | CLI Required | Description |
|----------|-------------|-------------|
| `claude` | [Claude Code](https://github.com/anthropics/claude-code) | Anthropic's Claude Code CLI |
| `opencode` | [OpenCode](https://github.com/opencode-ai/opencode) | OpenCode CLI |
| `kimi` | [Kimi CLI](https://github.com/anthropics/kimi) | Kimi CLI |

The selected provider must be installed and available on your `PATH`.

## Running Tests

```bash
go test ./... -v
```
