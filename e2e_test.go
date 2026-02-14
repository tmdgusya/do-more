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
