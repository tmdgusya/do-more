package config

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

func TestBackwardCompat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "do-more.json")
	// Old-format JSON without provider field
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

	if len(cfg.Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d, want 1", len(cfg.Tasks))
	}
	if cfg.Tasks[0].Provider != "" {
		t.Errorf("Tasks[0].Provider = %q, want empty string", cfg.Tasks[0].Provider)
	}
}

func TestProviderRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "do-more.json")

	cfg := &Config{
		Name:          "test-project",
		Provider:      "claude",
		Branch:        "feat/test",
		Gates:         []string{"go test ./..."},
		MaxIterations: 10,
		Tasks: []Task{
			{ID: "1", Title: "Test task", Description: "desc", Status: StatusPending, Provider: "opencode"},
		},
	}

	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig after save failed: %v", err)
	}

	if len(loaded.Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d, want 1", len(loaded.Tasks))
	}
	if loaded.Tasks[0].Provider != "opencode" {
		t.Errorf("Tasks[0].Provider = %q, want %q", loaded.Tasks[0].Provider, "opencode")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	jsonStr := string(data)
	if !contains(jsonStr, `"provider": "opencode"`) {
		t.Errorf("JSON should contain task provider field")
	}

	cfg2 := &Config{
		Name:     "test-project",
		Provider: "claude",
		Tasks: []Task{
			{ID: "2", Title: "Test task 2", Description: "desc", Status: StatusPending, Provider: ""},
		},
	}
	path2 := filepath.Join(dir, "do-more2.json")
	if err := SaveConfig(path2, cfg2); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}
	data2, err := os.ReadFile(path2)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	jsonStr2 := string(data2)
	if contains(jsonStr2, `"provider":`) && contains(jsonStr2, `"id": "2"`) {
		idIdx := findIndex(jsonStr2, `"id": "2"`)
		providerIdx := findIndex(jsonStr2, `"provider":`)
		if idIdx < providerIdx {
			t.Errorf("JSON should omit empty provider field in task")
		}
	}
}

func TestEffectiveProvider(t *testing.T) {
	tests := []struct {
		name     string
		task     *Task
		fallback string
		want     string
	}{
		{
			name:     "task provider set",
			task:     &Task{ID: "1", Provider: "kimi"},
			fallback: "claude",
			want:     "kimi",
		},
		{
			name:     "task provider empty, use fallback",
			task:     &Task{ID: "2", Provider: ""},
			fallback: "opencode",
			want:     "opencode",
		},
		{
			name:     "task provider empty, fallback empty",
			task:     &Task{ID: "3", Provider: ""},
			fallback: "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.task.EffectiveProvider(tt.fallback)
			if got != tt.want {
				t.Errorf("EffectiveProvider(%q) = %q, want %q", tt.fallback, got, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func findIndex(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
