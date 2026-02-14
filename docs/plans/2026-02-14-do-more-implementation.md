# do-more Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Go CLI that orchestrates AI coding providers (Claude Code, OpenCode, Kimi CLI) in an autonomous while-loop until all PRD tasks pass configurable quality gates.

**Architecture:** Flat package Go CLI using cobra for commands. Provider interface enables extensibility. JSON-based task file tracks state. Sequential execution with configurable gates.

**Tech Stack:** Go, cobra (CLI), text/template (prompts), os/exec (providers + gates), encoding/json (config)

---

### Task 1: Initialize Go Module

**Files:**
- Create: `go.mod`

**Step 1: Initialize the Go module**

Run: `go mod init github.com/tmdgusya/do-more`
Expected: `go.mod` created

**Step 2: Commit**

```bash
git add go.mod
git commit -m "chore: initialize go module"
```

---

### Task 2: Config Types and Loading

**Files:**
- Create: `config.go`
- Create: `config_test.go`

**Step 1: Write the failing test**

```go
// config_test.go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "do-more.json")
	data := []byte(`{
		"name": "test-project",
		"provider": "claude",
		"branch": "feat/test",
		"gates": ["go test ./..."],
		"maxIterations": 10,
		"tasks": [
			{
				"id": "1",
				"title": "Test task",
				"description": "A test task",
				"status": "pending",
				"learnings": ""
			}
		]
	}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Name != "test-project" {
		t.Errorf("Name = %q, want %q", cfg.Name, "test-project")
	}
	if cfg.Provider != "claude" {
		t.Errorf("Provider = %q, want %q", cfg.Provider, "claude")
	}
	if cfg.Branch != "feat/test" {
		t.Errorf("Branch = %q, want %q", cfg.Branch, "feat/test")
	}
	if len(cfg.Gates) != 1 || cfg.Gates[0] != "go test ./..." {
		t.Errorf("Gates = %v, want [go test ./...]", cfg.Gates)
	}
	if cfg.MaxIterations != 10 {
		t.Errorf("MaxIterations = %d, want 10", cfg.MaxIterations)
	}
	if len(cfg.Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d, want 1", len(cfg.Tasks))
	}
	if cfg.Tasks[0].Status != "pending" {
		t.Errorf("Tasks[0].Status = %q, want %q", cfg.Tasks[0].Status, "pending")
	}
}

func TestSaveConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "do-more.json")

	cfg := &Config{
		Name:          "test-project",
		Provider:      "claude",
		Branch:        "feat/test",
		Gates:         []string{"go test ./..."},
		MaxIterations: 10,
		Tasks: []Task{
			{ID: "1", Title: "Test task", Description: "desc", Status: StatusPending},
		},
	}

	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig after save failed: %v", err)
	}
	if loaded.Name != cfg.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, cfg.Name)
	}
	if len(loaded.Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d, want 1", len(loaded.Tasks))
	}
	if loaded.Tasks[0].ID != "1" {
		t.Errorf("Tasks[0].ID = %q, want %q", loaded.Tasks[0].ID, "1")
	}
}

func TestLoadConfigFileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/do-more.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestNextPendingTask(t *testing.T) {
	cfg := &Config{
		Tasks: []Task{
			{ID: "1", Title: "Done task", Status: StatusDone},
			{ID: "2", Title: "Pending task", Status: StatusPending},
			{ID: "3", Title: "Another pending", Status: StatusPending},
		},
	}

	task := cfg.NextPendingTask()
	if task == nil {
		t.Fatal("expected a pending task")
	}
	if task.ID != "2" {
		t.Errorf("NextPendingTask ID = %q, want %q", task.ID, "2")
	}
}

func TestNextPendingTaskNoneLeft(t *testing.T) {
	cfg := &Config{
		Tasks: []Task{
			{ID: "1", Title: "Done task", Status: StatusDone},
		},
	}

	task := cfg.NextPendingTask()
	if task != nil {
		t.Errorf("expected nil, got task %q", task.ID)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run TestLoadConfig -v`
Expected: FAIL — `LoadConfig` not defined

**Step 3: Write minimal implementation**

```go
// config.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusDone       = "done"
	StatusFailed     = "failed"
)

type Task struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Learnings   string `json:"learnings"`
}

type Config struct {
	Name          string   `json:"name"`
	Provider      string   `json:"provider"`
	Branch        string   `json:"branch"`
	Gates         []string `json:"gates"`
	MaxIterations int      `json:"maxIterations"`
	Tasks         []Task   `json:"tasks"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

func SaveConfig(path string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

func (c *Config) NextPendingTask() *Task {
	for i := range c.Tasks {
		if c.Tasks[i].Status == StatusPending {
			return &c.Tasks[i]
		}
	}
	return nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test -run "TestLoadConfig|TestSaveConfig|TestNextPendingTask" -v`
Expected: PASS (all 5 tests)

**Step 5: Commit**

```bash
git add config.go config_test.go
git commit -m "feat: add config types and JSON loading/saving"
```

---

### Task 3: Provider Interface and Registry

**Files:**
- Create: `provider.go`
- Create: `provider_test.go`

**Step 1: Write the failing test**

```go
// provider_test.go
package main

import (
	"context"
	"testing"
)

type mockProvider struct {
	name   string
	output string
	err    error
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Run(ctx context.Context, prompt string, workDir string) (string, error) {
	return m.output, m.err
}

func TestRegisterAndGetProvider(t *testing.T) {
	registry := NewProviderRegistry()
	mock := &mockProvider{name: "mock", output: "done"}

	registry.Register(mock)

	p, ok := registry.Get("mock")
	if !ok {
		t.Fatal("expected provider to be registered")
	}
	if p.Name() != "mock" {
		t.Errorf("Name() = %q, want %q", p.Name(), "mock")
	}
}

func TestGetUnregisteredProvider(t *testing.T) {
	registry := NewProviderRegistry()

	_, ok := registry.Get("nonexistent")
	if ok {
		t.Fatal("expected provider to not be found")
	}
}

func TestListProviders(t *testing.T) {
	registry := NewProviderRegistry()
	registry.Register(&mockProvider{name: "alpha"})
	registry.Register(&mockProvider{name: "beta"})

	names := registry.List()
	if len(names) != 2 {
		t.Fatalf("len(List()) = %d, want 2", len(names))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run "TestRegisterAndGetProvider|TestGetUnregisteredProvider|TestListProviders" -v`
Expected: FAIL — `NewProviderRegistry` not defined

**Step 3: Write minimal implementation**

```go
// provider.go
package main

import (
	"context"
	"sort"
)

type Provider interface {
	Name() string
	Run(ctx context.Context, prompt string, workDir string) (string, error)
}

type ProviderRegistry struct {
	providers map[string]Provider
}

func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]Provider),
	}
}

func (r *ProviderRegistry) Register(p Provider) {
	r.providers[p.Name()] = p
}

func (r *ProviderRegistry) Get(name string) (Provider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

func (r *ProviderRegistry) List() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
```

**Step 4: Run tests to verify they pass**

Run: `go test -run "TestRegisterAndGetProvider|TestGetUnregisteredProvider|TestListProviders" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add provider.go provider_test.go
git commit -m "feat: add provider interface and registry"
```

---

### Task 4: Claude Code Provider

**Files:**
- Create: `claude.go`

**Step 1: Write the implementation**

Note: No unit test for this provider — it shells out to `claude` which is an external binary. Testing it would require integration tests against the real CLI.

```go
// claude.go
package main

import (
	"context"
	"fmt"
	"os/exec"
)

type ClaudeProvider struct{}

func (p *ClaudeProvider) Name() string {
	return "claude"
}

func (p *ClaudeProvider) Run(ctx context.Context, prompt string, workDir string) (string, error) {
	cmd := exec.CommandContext(ctx, "claude", "-p", prompt, "--output-format", "text")
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("claude provider: %w\noutput: %s", err, output)
	}
	return string(output), nil
}
```

**Step 2: Commit**

```bash
git add claude.go
git commit -m "feat: add claude code provider"
```

---

### Task 5: OpenCode Provider

**Files:**
- Create: `opencode.go`

**Step 1: Write the implementation**

```go
// opencode.go
package main

import (
	"context"
	"fmt"
	"os/exec"
)

type OpenCodeProvider struct{}

func (p *OpenCodeProvider) Name() string {
	return "opencode"
}

func (p *OpenCodeProvider) Run(ctx context.Context, prompt string, workDir string) (string, error) {
	cmd := exec.CommandContext(ctx, "opencode", "-p", prompt, "-q", "-f", "text")
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("opencode provider: %w\noutput: %s", err, output)
	}
	return string(output), nil
}
```

**Step 2: Commit**

```bash
git add opencode.go
git commit -m "feat: add opencode provider"
```

---

### Task 6: Kimi CLI Provider

**Files:**
- Create: `kimi.go`

**Step 1: Write the implementation**

```go
// kimi.go
package main

import (
	"context"
	"fmt"
	"os/exec"
)

type KimiProvider struct{}

func (p *KimiProvider) Name() string {
	return "kimi"
}

func (p *KimiProvider) Run(ctx context.Context, prompt string, workDir string) (string, error) {
	cmd := exec.CommandContext(ctx, "kimi", "--print", "-p", prompt, "--final-message-only")
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("kimi provider: %w\noutput: %s", err, output)
	}
	return string(output), nil
}
```

**Step 2: Commit**

```bash
git add kimi.go
git commit -m "feat: add kimi cli provider"
```

---

### Task 7: Gate Runner

**Files:**
- Create: `gate.go`
- Create: `gate_test.go`

**Step 1: Write the failing test**

```go
// gate_test.go
package main

import (
	"context"
	"testing"
)

func TestRunGatesAllPass(t *testing.T) {
	gates := []string{"true", "echo hello"}
	results, err := RunGates(context.Background(), gates, t.TempDir())
	if err != nil {
		t.Fatalf("RunGates failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	for i, r := range results {
		if !r.Passed {
			t.Errorf("gate %d (%q) failed: %s", i, r.Command, r.Output)
		}
	}
}

func TestRunGatesOneFails(t *testing.T) {
	gates := []string{"true", "false", "true"}
	results, err := RunGates(context.Background(), gates, t.TempDir())
	if err != nil {
		t.Fatalf("RunGates failed: %v", err)
	}
	if results[0].Passed != true {
		t.Error("gate 0 should pass")
	}
	if results[1].Passed != false {
		t.Error("gate 1 should fail")
	}
	if results[2].Passed != true {
		t.Error("gate 2 should pass")
	}
}

func TestRunGatesEmpty(t *testing.T) {
	results, err := RunGates(context.Background(), []string{}, t.TempDir())
	if err != nil {
		t.Fatalf("RunGates failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("len(results) = %d, want 0", len(results))
	}
}

func TestAllGatesPassed(t *testing.T) {
	passing := []GateResult{
		{Command: "true", Passed: true},
		{Command: "echo ok", Passed: true},
	}
	if !AllGatesPassed(passing) {
		t.Error("expected AllGatesPassed to return true")
	}

	failing := []GateResult{
		{Command: "true", Passed: true},
		{Command: "false", Passed: false, Output: "failed"},
	}
	if AllGatesPassed(failing) {
		t.Error("expected AllGatesPassed to return false")
	}
}

func TestGateFailureSummary(t *testing.T) {
	results := []GateResult{
		{Command: "true", Passed: true},
		{Command: "false", Passed: false, Output: "exit status 1"},
	}
	summary := GateFailureSummary(results)
	if summary == "" {
		t.Error("expected non-empty summary")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run "TestRunGates|TestAllGatesPassed|TestGateFailureSummary" -v`
Expected: FAIL — `RunGates` not defined

**Step 3: Write minimal implementation**

```go
// gate.go
package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type GateResult struct {
	Command string
	Passed  bool
	Output  string
}

func RunGates(ctx context.Context, gates []string, workDir string) ([]GateResult, error) {
	results := make([]GateResult, 0, len(gates))
	for _, gate := range gates {
		cmd := exec.CommandContext(ctx, "sh", "-c", gate)
		cmd.Dir = workDir
		output, err := cmd.CombinedOutput()
		result := GateResult{
			Command: gate,
			Passed:  err == nil,
			Output:  string(output),
		}
		results = append(results, result)
	}
	return results, nil
}

func AllGatesPassed(results []GateResult) bool {
	for _, r := range results {
		if !r.Passed {
			return false
		}
	}
	return true
}

func GateFailureSummary(results []GateResult) string {
	var sb strings.Builder
	for _, r := range results {
		if !r.Passed {
			fmt.Fprintf(&sb, "FAIL: %s\n%s\n", r.Command, r.Output)
		}
	}
	return sb.String()
}
```

**Step 4: Run tests to verify they pass**

Run: `go test -run "TestRunGates|TestAllGatesPassed|TestGateFailureSummary" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add gate.go gate_test.go
git commit -m "feat: add gate runner for quality checks"
```

---

### Task 8: Prompt Builder

**Files:**
- Create: `prompt.go`
- Create: `prompt_test.go`

**Step 1: Write the failing test**

```go
// prompt_test.go
package main

import (
	"strings"
	"testing"
)

func TestBuildPrompt(t *testing.T) {
	task := &Task{
		Title:       "Add login endpoint",
		Description: "Create POST /api/login",
		Learnings:   "Use bcrypt for passwords",
	}
	gates := []string{"go test ./...", "golangci-lint run"}

	prompt := BuildPrompt(task, gates, "")

	if !strings.Contains(prompt, "Add login endpoint") {
		t.Error("prompt should contain task title")
	}
	if !strings.Contains(prompt, "Create POST /api/login") {
		t.Error("prompt should contain task description")
	}
	if !strings.Contains(prompt, "Use bcrypt for passwords") {
		t.Error("prompt should contain learnings")
	}
	if !strings.Contains(prompt, "go test ./...") {
		t.Error("prompt should contain gates")
	}
}

func TestBuildPromptWithGateFailures(t *testing.T) {
	task := &Task{
		Title:       "Fix tests",
		Description: "Make tests pass",
	}
	gates := []string{"go test ./..."}
	gateOutput := "FAIL: TestFoo expected 1 got 2"

	prompt := BuildPrompt(task, gates, gateOutput)

	if !strings.Contains(prompt, gateOutput) {
		t.Error("prompt should contain gate failure output")
	}
}

func TestBuildPromptNoLearnings(t *testing.T) {
	task := &Task{
		Title:       "New task",
		Description: "Do something",
		Learnings:   "",
	}

	prompt := BuildPrompt(task, []string{}, "")

	if strings.Contains(prompt, "Previous Learnings") {
		t.Error("prompt should not contain learnings section when empty")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run "TestBuildPrompt" -v`
Expected: FAIL — `BuildPrompt` not defined

**Step 3: Write minimal implementation**

```go
// prompt.go
package main

import (
	"fmt"
	"strings"
)

func BuildPrompt(task *Task, gates []string, gateOutput string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "You are working on the following task:\n\n")
	fmt.Fprintf(&sb, "## Task: %s\n%s\n", task.Title, task.Description)

	if task.Learnings != "" {
		fmt.Fprintf(&sb, "\n## Previous Learnings\n%s\n", task.Learnings)
	}

	if gateOutput != "" {
		fmt.Fprintf(&sb, "\n## Gate Failures (previous attempt)\n%s\n", gateOutput)
	}

	if len(gates) > 0 {
		fmt.Fprintf(&sb, "\n## Instructions\n")
		fmt.Fprintf(&sb, "- Work in the current directory\n")
		fmt.Fprintf(&sb, "- Make the minimal changes needed\n")
		fmt.Fprintf(&sb, "- When done, the following gates will be checked:\n")
		for _, g := range gates {
			fmt.Fprintf(&sb, "  - %s\n", g)
		}
	}

	return sb.String()
}
```

**Step 4: Run tests to verify they pass**

Run: `go test -run "TestBuildPrompt" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add prompt.go prompt_test.go
git commit -m "feat: add prompt builder"
```

---

### Task 9: Core Loop

**Files:**
- Create: `loop.go`
- Create: `loop_test.go`

**Step 1: Write the failing test**

```go
// loop_test.go
package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLoopAllTasksComplete(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "do-more.json")

	cfg := &Config{
		Name:          "test",
		Provider:      "mock",
		Gates:         []string{"true"},
		MaxIterations: 3,
		Tasks: []Task{
			{ID: "1", Title: "Task one", Description: "Do thing one", Status: StatusPending},
			{ID: "2", Title: "Task two", Description: "Do thing two", Status: StatusPending},
		},
	}
	if err := SaveConfig(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}

	registry := NewProviderRegistry()
	registry.Register(&mockProvider{name: "mock", output: "done"})

	logger := &LogRecorder{}
	err := RunLoop(context.Background(), cfgPath, "mock", registry, dir, logger)
	if err != nil {
		t.Fatalf("RunLoop failed: %v", err)
	}

	reloaded, _ := LoadConfig(cfgPath)
	for _, task := range reloaded.Tasks {
		if task.Status != StatusDone {
			t.Errorf("task %q status = %q, want %q", task.ID, task.Status, StatusDone)
		}
	}
}

func TestLoopProviderFails(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "do-more.json")

	cfg := &Config{
		Name:          "test",
		Provider:      "failing",
		Gates:         []string{"true"},
		MaxIterations: 2,
		Tasks: []Task{
			{ID: "1", Title: "Task one", Description: "Do thing", Status: StatusPending},
		},
	}
	if err := SaveConfig(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}

	registry := NewProviderRegistry()
	registry.Register(&mockProvider{
		name:   "failing",
		output: "",
		err:    os.ErrNotExist,
	})

	logger := &LogRecorder{}
	err := RunLoop(context.Background(), cfgPath, "failing", registry, dir, logger)
	if err != nil {
		t.Fatalf("RunLoop failed: %v", err)
	}

	reloaded, _ := LoadConfig(cfgPath)
	if reloaded.Tasks[0].Status != StatusFailed {
		t.Errorf("task status = %q, want %q", reloaded.Tasks[0].Status, StatusFailed)
	}
}

func TestLoopGateFails(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "do-more.json")

	cfg := &Config{
		Name:          "test",
		Provider:      "mock",
		Gates:         []string{"false"},
		MaxIterations: 2,
		Tasks: []Task{
			{ID: "1", Title: "Task one", Description: "Do thing", Status: StatusPending},
		},
	}
	if err := SaveConfig(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}

	registry := NewProviderRegistry()
	registry.Register(&mockProvider{name: "mock", output: "done"})

	logger := &LogRecorder{}
	err := RunLoop(context.Background(), cfgPath, "mock", registry, dir, logger)
	if err != nil {
		t.Fatalf("RunLoop failed: %v", err)
	}

	reloaded, _ := LoadConfig(cfgPath)
	if reloaded.Tasks[0].Status != StatusFailed {
		t.Errorf("task status = %q, want %q", reloaded.Tasks[0].Status, StatusFailed)
	}
}

// LogRecorder captures log output for testing
type LogRecorder struct {
	Messages []string
}

func (l *LogRecorder) Log(format string, args ...any) {
	l.Messages = append(l.Messages, format)
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run "TestLoop" -v`
Expected: FAIL — `RunLoop` not defined

**Step 3: Write minimal implementation**

```go
// loop.go
package main

import (
	"context"
	"fmt"
)

type Logger interface {
	Log(format string, args ...any)
}

type StdoutLogger struct{}

func (l *StdoutLogger) Log(format string, args ...any) {
	fmt.Printf("[do-more] "+format+"\n", args...)
}

func RunLoop(ctx context.Context, cfgPath string, providerName string, registry *ProviderRegistry, workDir string, logger Logger) error {
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	provider, ok := registry.Get(providerName)
	if !ok {
		return fmt.Errorf("unknown provider: %q", providerName)
	}

	logger.Log("Starting with provider: %s", provider.Name())

	for {
		task := cfg.NextPendingTask()
		if task == nil {
			break
		}

		task.Status = StatusInProgress
		if err := SaveConfig(cfgPath, cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		var gateOutput string
		completed := false

		for iteration := 1; iteration <= cfg.MaxIterations; iteration++ {
			logger.Log("── Iteration %d/%d ── Task #%s: %s", iteration, cfg.MaxIterations, task.ID, task.Title)

			prompt := BuildPrompt(task, cfg.Gates, gateOutput)

			logger.Log("Invoking %s...", provider.Name())
			output, err := provider.Run(ctx, prompt, workDir)
			if err != nil {
				logger.Log("Provider error: %v", err)
				if iteration >= cfg.MaxIterations {
					task.Status = StatusFailed
					task.Learnings += fmt.Sprintf("\nFailed after %d iterations. Last error: %v", iteration, err)
					break
				}
				gateOutput = fmt.Sprintf("Provider error: %v\nOutput: %s", err, output)
				continue
			}

			logger.Log("Provider finished")

			results, err := RunGates(ctx, cfg.Gates, workDir)
			if err != nil {
				return fmt.Errorf("running gates: %w", err)
			}

			allPassed := true
			for _, r := range results {
				if r.Passed {
					logger.Log("Running gate: %s  ✓", r.Command)
				} else {
					logger.Log("Running gate: %s  ✗", r.Command)
					allPassed = false
				}
			}

			if allPassed {
				task.Status = StatusDone
				completed = true
				logger.Log("Task #%s: done", task.ID)
				break
			}

			if iteration >= cfg.MaxIterations {
				task.Status = StatusFailed
				task.Learnings += fmt.Sprintf("\nFailed after %d iterations. Gates did not pass.", iteration)
				logger.Log("Task #%s: failed (max iterations reached)", task.ID)
				break
			}

			gateOutput = GateFailureSummary(results)
		}

		if err := SaveConfig(cfgPath, cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		_ = completed
	}

	// Print summary
	done := 0
	failed := 0
	for _, t := range cfg.Tasks {
		switch t.Status {
		case StatusDone:
			done++
		case StatusFailed:
			failed++
		}
	}
	total := len(cfg.Tasks)
	logger.Log("── Summary ──")
	logger.Log("%d/%d tasks done, %d failed", done, total, failed)

	return nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test -run "TestLoop" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add loop.go loop_test.go
git commit -m "feat: add core loop logic"
```

---

### Task 10: CLI with Cobra (init, run, status, providers)

**Files:**
- Create: `main.go`

**Step 1: Install cobra dependency**

Run: `go get github.com/spf13/cobra`

**Step 2: Write the CLI**

```go
// main.go
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func defaultRegistry() *ProviderRegistry {
	registry := NewProviderRegistry()
	registry.Register(&ClaudeProvider{})
	registry.Register(&OpenCodeProvider{})
	registry.Register(&KimiProvider{})
	return registry
}

func main() {
	registry := defaultRegistry()

	rootCmd := &cobra.Command{
		Use:   "do-more",
		Short: "Autonomous AI coding loop orchestrator",
	}

	// --- init ---
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Create a do-more.json template in the current directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := filepath.Join(".", "do-more.json")
			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("do-more.json already exists")
			}
			cfg := &Config{
				Name:          filepath.Base(mustGetwd()),
				Provider:      "claude",
				Branch:        "feat/do-more",
				Gates:         []string{},
				MaxIterations: 10,
				Tasks: []Task{
					{
						ID:          "1",
						Title:       "Example task",
						Description: "Describe what needs to be done",
						Status:      StatusPending,
						Learnings:   "",
					},
				},
			}
			if err := SaveConfig(path, cfg); err != nil {
				return err
			}
			fmt.Println("[do-more] Created do-more.json")
			return nil
		},
	}

	// --- run ---
	var providerFlag string
	var maxIterationsFlag int
	var configFlag string

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Start the autonomous loop",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := configFlag
			cfg, err := LoadConfig(cfgPath)
			if err != nil {
				return fmt.Errorf("loading %s: %w", cfgPath, err)
			}

			providerName := cfg.Provider
			if providerFlag != "" {
				providerName = providerFlag
			}

			if maxIterationsFlag > 0 {
				cfg.MaxIterations = maxIterationsFlag
				if err := SaveConfig(cfgPath, cfg); err != nil {
					return err
				}
			}

			workDir := filepath.Dir(cfgPath)
			if !filepath.IsAbs(workDir) {
				workDir = mustGetwd()
			}

			logger := &StdoutLogger{}
			return RunLoop(context.Background(), cfgPath, providerName, registry, workDir, logger)
		},
	}
	runCmd.Flags().StringVar(&providerFlag, "provider", "", "Override provider from config")
	runCmd.Flags().IntVar(&maxIterationsFlag, "max-iterations", 0, "Override max iterations per task")
	runCmd.Flags().StringVar(&configFlag, "config", "do-more.json", "Path to config file")

	// --- status ---
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show task status summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := LoadConfig("do-more.json")
			if err != nil {
				return err
			}

			fmt.Printf("Project: %s\n", cfg.Name)
			fmt.Printf("Provider: %s\n", cfg.Provider)
			fmt.Printf("Branch: %s\n", cfg.Branch)
			fmt.Printf("Gates: %s\n", strings.Join(cfg.Gates, ", "))
			fmt.Println()

			for _, t := range cfg.Tasks {
				marker := " "
				switch t.Status {
				case StatusDone:
					marker = "✓"
				case StatusFailed:
					marker = "✗"
				case StatusInProgress:
					marker = "→"
				}
				fmt.Printf("  [%s] #%s %s (%s)\n", marker, t.ID, t.Title, t.Status)
			}
			return nil
		},
	}

	// --- providers ---
	providersCmd := &cobra.Command{
		Use:   "providers",
		Short: "List available providers",
		Run: func(cmd *cobra.Command, args []string) {
			for _, name := range registry.List() {
				fmt.Printf("  - %s\n", name)
			}
		},
	}

	rootCmd.AddCommand(initCmd, runCmd, statusCmd, providersCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return wd
}
```

**Step 3: Tidy dependencies**

Run: `go mod tidy`

**Step 4: Build and verify**

Run: `go build -o do-more .`
Run: `./do-more --help`
Expected: Shows help with init, run, status, providers subcommands

Run: `./do-more providers`
Expected: Lists claude, kimi, opencode

**Step 5: Commit**

```bash
git add main.go go.mod go.sum
git commit -m "feat: add CLI with init, run, status, providers commands"
```

---

### Task 11: End-to-End Smoke Test

**Files:**
- Create: `e2e_test.go`

**Step 1: Write the test**

```go
// e2e_test.go
package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestE2EFullLoop(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "do-more.json")

	// Create a file for the "gate" to check
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		Name:          "e2e-test",
		Provider:      "mock",
		Gates:         []string{"test -f hello.txt"},
		MaxIterations: 3,
		Tasks: []Task{
			{ID: "1", Title: "First task", Description: "Do first thing", Status: StatusPending},
			{ID: "2", Title: "Second task", Description: "Do second thing", Status: StatusPending},
		},
	}
	if err := SaveConfig(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}

	registry := NewProviderRegistry()
	registry.Register(&mockProvider{name: "mock", output: "done"})

	logger := &LogRecorder{}
	err := RunLoop(context.Background(), cfgPath, "mock", registry, dir, logger)
	if err != nil {
		t.Fatalf("RunLoop failed: %v", err)
	}

	reloaded, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	for _, task := range reloaded.Tasks {
		if task.Status != StatusDone {
			t.Errorf("task %q status = %q, want %q", task.ID, task.Status, StatusDone)
		}
	}
}

func TestE2EInitCreatesConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "do-more.json")

	cfg := &Config{
		Name:          "new-project",
		Provider:      "claude",
		Branch:        "feat/do-more",
		Gates:         []string{},
		MaxIterations: 10,
		Tasks: []Task{
			{ID: "1", Title: "Example task", Description: "Describe what needs to be done", Status: StatusPending},
		},
	}
	if err := SaveConfig(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != "new-project" {
		t.Errorf("Name = %q, want %q", loaded.Name, "new-project")
	}
	if loaded.Provider != "claude" {
		t.Errorf("Provider = %q, want %q", loaded.Provider, "claude")
	}
}
```

**Step 2: Run all tests**

Run: `go test ./... -v`
Expected: ALL PASS

**Step 3: Commit**

```bash
git add e2e_test.go
git commit -m "test: add end-to-end smoke tests"
```

---

### Task 12: Build, Clean Up, Final Verification

**Step 1: Run full test suite**

Run: `go test ./... -v -count=1`
Expected: ALL PASS

**Step 2: Build the binary**

Run: `go build -o do-more .`
Expected: Binary created successfully

**Step 3: Test the binary manually**

Run: `./do-more init` (in a temp directory)
Run: `./do-more status`
Run: `./do-more providers`
Expected: All commands work correctly

**Step 4: Add .gitignore**

```
# .gitignore
do-more
```

**Step 5: Final commit**

```bash
git add .gitignore
git commit -m "chore: add gitignore and finalize v0.1"
```
