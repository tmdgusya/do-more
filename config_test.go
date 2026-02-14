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
