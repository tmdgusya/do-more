package loop

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/tmdgusya/do-more/internal/config"
	"github.com/tmdgusya/do-more/internal/provider"
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

// LogRecorder captures log output for testing.
type LogRecorder struct {
	Messages []string
}

func (l *LogRecorder) Log(format string, args ...any) {
	l.Messages = append(l.Messages, format)
}

func TestLoopAllTasksComplete(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "do-more.json")

	cfg := &config.Config{
		Name:          "test",
		Provider:      "mock",
		Gates:         []string{"true"},
		MaxIterations: 3,
		Tasks: []config.Task{
			{ID: "1", Title: "Task one", Description: "Do thing one", Status: config.StatusPending},
			{ID: "2", Title: "Task two", Description: "Do thing two", Status: config.StatusPending},
		},
	}
	if err := config.SaveConfig(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}

	registry := provider.NewProviderRegistry()
	registry.Register(&mockProvider{name: "mock", output: "done"})

	logger := &LogRecorder{}
	err := RunLoop(context.Background(), cfgPath, "mock", registry, dir, logger)
	if err != nil {
		t.Fatalf("RunLoop failed: %v", err)
	}

	reloaded, _ := config.LoadConfig(cfgPath)
	for _, task := range reloaded.Tasks {
		if task.Status != config.StatusDone {
			t.Errorf("task %q status = %q, want %q", task.ID, task.Status, config.StatusDone)
		}
	}
}

func TestLoopProviderFails(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "do-more.json")

	cfg := &config.Config{
		Name:          "test",
		Provider:      "failing",
		Gates:         []string{"true"},
		MaxIterations: 2,
		Tasks: []config.Task{
			{ID: "1", Title: "Task one", Description: "Do thing", Status: config.StatusPending},
		},
	}
	if err := config.SaveConfig(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}

	registry := provider.NewProviderRegistry()
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

	reloaded, _ := config.LoadConfig(cfgPath)
	if reloaded.Tasks[0].Status != config.StatusFailed {
		t.Errorf("task status = %q, want %q", reloaded.Tasks[0].Status, config.StatusFailed)
	}
}

func TestLoopGateFails(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "do-more.json")

	cfg := &config.Config{
		Name:          "test",
		Provider:      "mock",
		Gates:         []string{"false"},
		MaxIterations: 2,
		Tasks: []config.Task{
			{ID: "1", Title: "Task one", Description: "Do thing", Status: config.StatusPending},
		},
	}
	if err := config.SaveConfig(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}

	registry := provider.NewProviderRegistry()
	registry.Register(&mockProvider{name: "mock", output: "done"})

	logger := &LogRecorder{}
	err := RunLoop(context.Background(), cfgPath, "mock", registry, dir, logger)
	if err != nil {
		t.Fatalf("RunLoop failed: %v", err)
	}

	reloaded, _ := config.LoadConfig(cfgPath)
	if reloaded.Tasks[0].Status != config.StatusFailed {
		t.Errorf("task status = %q, want %q", reloaded.Tasks[0].Status, config.StatusFailed)
	}
}

func TestPerTaskProvider(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "do-more.json")

	cfg := &config.Config{
		Name:          "test",
		Provider:      "mock-a",
		Gates:         []string{"true"},
		MaxIterations: 3,
		Tasks: []config.Task{
			{ID: "1", Title: "Task one", Description: "Do thing", Status: config.StatusPending, Provider: "mock-b"},
		},
	}
	if err := config.SaveConfig(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}

	registry := provider.NewProviderRegistry()
	registry.Register(&mockProvider{name: "mock-a", output: "from-a"})
	registry.Register(&mockProvider{name: "mock-b", output: "from-b"})

	logger := &LogRecorder{}
	err := RunLoop(context.Background(), cfgPath, "mock-a", registry, dir, logger)
	if err != nil {
		t.Fatalf("RunLoop failed: %v", err)
	}

	reloaded, _ := config.LoadConfig(cfgPath)
	if reloaded.Tasks[0].Status != config.StatusDone {
		t.Errorf("task status = %q, want %q", reloaded.Tasks[0].Status, config.StatusDone)
	}
}

func TestDefaultProviderFallback(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "do-more.json")

	cfg := &config.Config{
		Name:          "test",
		Provider:      "mock",
		Gates:         []string{"true"},
		MaxIterations: 3,
		Tasks: []config.Task{
			{ID: "1", Title: "Task one", Description: "Do thing", Status: config.StatusPending},
		},
	}
	if err := config.SaveConfig(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}

	registry := provider.NewProviderRegistry()
	registry.Register(&mockProvider{name: "mock", output: "done"})

	logger := &LogRecorder{}
	err := RunLoop(context.Background(), cfgPath, "mock", registry, dir, logger)
	if err != nil {
		t.Fatalf("RunLoop failed: %v", err)
	}

	reloaded, _ := config.LoadConfig(cfgPath)
	if reloaded.Tasks[0].Status != config.StatusDone {
		t.Errorf("task status = %q, want %q", reloaded.Tasks[0].Status, config.StatusDone)
	}
}

func TestInvalidProviderFails(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "do-more.json")

	cfg := &config.Config{
		Name:          "test",
		Provider:      "mock",
		Gates:         []string{"true"},
		MaxIterations: 3,
		Tasks: []config.Task{
			{ID: "1", Title: "Task one", Description: "Do thing", Status: config.StatusPending, Provider: "nonexistent"},
		},
	}
	if err := config.SaveConfig(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}

	registry := provider.NewProviderRegistry()
	registry.Register(&mockProvider{name: "mock", output: "done"})

	logger := &LogRecorder{}
	err := RunLoop(context.Background(), cfgPath, "mock", registry, dir, logger)
	if err != nil {
		t.Fatalf("RunLoop failed: %v", err)
	}

	reloaded, _ := config.LoadConfig(cfgPath)
	if reloaded.Tasks[0].Status != config.StatusFailed {
		t.Errorf("task status = %q, want %q", reloaded.Tasks[0].Status, config.StatusFailed)
	}
	if !contains(reloaded.Tasks[0].Learnings, "Unknown provider") {
		t.Errorf("task learnings should contain 'Unknown provider', got: %q", reloaded.Tasks[0].Learnings)
	}
}

func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
