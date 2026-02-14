package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/tmdgusya/do-more/internal/config"
	"github.com/tmdgusya/do-more/internal/loop"
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

type logRecorder struct {
	Messages []string
}

func (l *logRecorder) Log(format string, args ...any) {
	l.Messages = append(l.Messages, format)
}

func TestE2EFullLoop(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "do-more.json")

	// Create a file for the "gate" to check
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Name:          "e2e-test",
		Provider:      "mock",
		Gates:         []string{"test -f hello.txt"},
		MaxIterations: 3,
		Tasks: []config.Task{
			{ID: "1", Title: "First task", Description: "Do first thing", Status: config.StatusPending},
			{ID: "2", Title: "Second task", Description: "Do second thing", Status: config.StatusPending},
		},
	}
	if err := config.SaveConfig(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}

	registry := provider.NewProviderRegistry()
	registry.Register(&mockProvider{name: "mock", output: "done"})

	logger := &logRecorder{}
	err := loop.RunLoop(context.Background(), cfgPath, "mock", registry, dir, logger)
	if err != nil {
		t.Fatalf("RunLoop failed: %v", err)
	}

	reloaded, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	for _, task := range reloaded.Tasks {
		if task.Status != config.StatusDone {
			t.Errorf("task %q status = %q, want %q", task.ID, task.Status, config.StatusDone)
		}
	}
}

func TestE2EInitCreatesConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "do-more.json")

	cfg := &config.Config{
		Name:          "new-project",
		Provider:      "claude",
		Branch:        "feat/do-more",
		Gates:         []string{},
		MaxIterations: 10,
		Tasks: []config.Task{
			{ID: "1", Title: "Example task", Description: "Describe what needs to be done", Status: config.StatusPending},
		},
	}
	if err := config.SaveConfig(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := config.LoadConfig(cfgPath)
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
