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
