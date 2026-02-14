package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/tmdgusya/do-more/internal/config"
	"github.com/tmdgusya/do-more/internal/provider"
)

type mockTestProvider struct{ name string }

func (m *mockTestProvider) Name() string { return m.name }
func (m *mockTestProvider) Run(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}

func setupTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "do-more.json")

	cfg := &config.Config{
		Name:          "test-project",
		Provider:      "claude",
		Branch:        "main",
		Gates:         []string{"go test ./..."},
		MaxIterations: 5,
		Tasks: []config.Task{
			{ID: "1", Title: "First task", Description: "Do first thing", Status: config.StatusPending},
			{ID: "2", Title: "Second task", Description: "Do second thing", Status: config.StatusDone},
			{ID: "3", Title: "Running task", Description: "Currently running", Status: config.StatusInProgress},
		},
	}
	if err := config.SaveConfig(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}

	registry := provider.NewProviderRegistry()
	registry.Register(&mockTestProvider{name: "claude"})
	registry.Register(&mockTestProvider{name: "kimi"})
	registry.Register(&mockTestProvider{name: "opencode"})

	srv := NewServer(cfgPath, registry)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, cfgPath
}

func TestGetConfig(t *testing.T) {
	ts, _ := setupTestServer(t)

	resp, err := http.Get(ts.URL + "/api/config")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}

	var cfg config.Config
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "test-project" {
		t.Errorf("expected name test-project, got %s", cfg.Name)
	}
	if len(cfg.Tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(cfg.Tasks))
	}
}

func TestGetProviders(t *testing.T) {
	ts, _ := setupTestServer(t)

	resp, err := http.Get(ts.URL + "/api/providers")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var providers []string
	if err := json.NewDecoder(resp.Body).Decode(&providers); err != nil {
		t.Fatal(err)
	}
	if len(providers) != 3 {
		t.Fatalf("expected 3 providers, got %d", len(providers))
	}
	expected := []string{"claude", "kimi", "opencode"}
	for i, p := range providers {
		if p != expected[i] {
			t.Errorf("expected provider[%d]=%s, got %s", i, expected[i], p)
		}
	}
}

func TestCreateTask(t *testing.T) {
	ts, cfgPath := setupTestServer(t)

	body := `{"title":"New task","description":"A new task","provider":"kimi"}`
	resp, err := http.Post(ts.URL+"/api/tasks", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var task config.Task
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		t.Fatal(err)
	}
	if task.ID != "4" {
		t.Errorf("expected ID 4, got %s", task.ID)
	}
	if task.Title != "New task" {
		t.Errorf("expected title 'New task', got %s", task.Title)
	}
	if task.Status != config.StatusPending {
		t.Errorf("expected status pending, got %s", task.Status)
	}
	if task.Provider != "kimi" {
		t.Errorf("expected provider kimi, got %s", task.Provider)
	}

	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Tasks) != 4 {
		t.Errorf("expected 4 tasks on disk, got %d", len(cfg.Tasks))
	}
}

func TestCreateTaskEmptyTitle(t *testing.T) {
	ts, _ := setupTestServer(t)

	body := `{"title":"","description":"no title"}`
	resp, err := http.Post(ts.URL+"/api/tasks", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	var errResp map[string]string
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp["error"] != "title is required" {
		t.Errorf("expected 'title is required', got %s", errResp["error"])
	}
}

func TestCreateTaskInvalidJSON(t *testing.T) {
	ts, _ := setupTestServer(t)

	resp, err := http.Post(ts.URL+"/api/tasks", "application/json", bytes.NewBufferString("{bad"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestUpdateTask(t *testing.T) {
	ts, cfgPath := setupTestServer(t)

	body := `{"title":"Updated title","description":"Updated desc","provider":"opencode"}`
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/tasks/1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var task config.Task
	json.NewDecoder(resp.Body).Decode(&task)
	if task.Title != "Updated title" {
		t.Errorf("expected 'Updated title', got %s", task.Title)
	}
	if task.Provider != "opencode" {
		t.Errorf("expected provider opencode, got %s", task.Provider)
	}

	cfg, _ := config.LoadConfig(cfgPath)
	if cfg.Tasks[0].Title != "Updated title" {
		t.Errorf("disk not updated: got %s", cfg.Tasks[0].Title)
	}
}

func TestUpdateTaskInProgress(t *testing.T) {
	ts, _ := setupTestServer(t)

	body := `{"title":"Try update"}`
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/tasks/3", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}

	var errResp map[string]string
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp["error"] != "cannot modify in_progress task" {
		t.Errorf("unexpected error: %s", errResp["error"])
	}
}

func TestUpdateTaskNotFound(t *testing.T) {
	ts, _ := setupTestServer(t)

	body := `{"title":"Ghost"}`
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/tasks/999", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDeleteTask(t *testing.T) {
	ts, cfgPath := setupTestServer(t)

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/tasks/1", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	cfg, _ := config.LoadConfig(cfgPath)
	if len(cfg.Tasks) != 2 {
		t.Errorf("expected 2 tasks after delete, got %d", len(cfg.Tasks))
	}
	for _, task := range cfg.Tasks {
		if task.ID == "1" {
			t.Error("task 1 should have been deleted")
		}
	}
}

func TestDeleteTaskInProgress(t *testing.T) {
	ts, _ := setupTestServer(t)

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/tasks/3", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}

func TestDeleteTaskNotFound(t *testing.T) {
	ts, _ := setupTestServer(t)

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/tasks/999", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUpdateConfig(t *testing.T) {
	ts, cfgPath := setupTestServer(t)

	maxIter := 20
	body, _ := json.Marshal(map[string]any{
		"provider":      "opencode",
		"branch":        "feat/new",
		"gates":         []string{"make test", "make lint"},
		"maxIterations": maxIter,
	})

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var cfg config.Config
	json.NewDecoder(resp.Body).Decode(&cfg)
	if cfg.Provider != "opencode" {
		t.Errorf("expected provider opencode, got %s", cfg.Provider)
	}
	if cfg.Branch != "feat/new" {
		t.Errorf("expected branch feat/new, got %s", cfg.Branch)
	}
	if cfg.MaxIterations != 20 {
		t.Errorf("expected maxIterations 20, got %d", cfg.MaxIterations)
	}
	if len(cfg.Gates) != 2 {
		t.Errorf("expected 2 gates, got %d", len(cfg.Gates))
	}

	diskCfg, _ := config.LoadConfig(cfgPath)
	if diskCfg.Provider != "opencode" {
		t.Errorf("disk not updated: provider=%s", diskCfg.Provider)
	}
	if len(diskCfg.Tasks) != 3 {
		t.Errorf("tasks should be unchanged, got %d", len(diskCfg.Tasks))
	}
}

func TestUpdateConfigPartial(t *testing.T) {
	ts, cfgPath := setupTestServer(t)

	body := `{"branch":"feat/partial"}`
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/config", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	cfg, _ := config.LoadConfig(cfgPath)
	if cfg.Branch != "feat/partial" {
		t.Errorf("expected branch feat/partial, got %s", cfg.Branch)
	}
	if cfg.Provider != "claude" {
		t.Errorf("provider should be unchanged, got %s", cfg.Provider)
	}
}

func TestUpdateConfigInvalidJSON(t *testing.T) {
	ts, _ := setupTestServer(t)

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/config", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestMutationPersists(t *testing.T) {
	ts, _ := setupTestServer(t)

	body := `{"title":"Persisted task","description":"Check persistence"}`
	resp, err := http.Post(ts.URL+"/api/tasks", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/api/config")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var cfg config.Config
	json.NewDecoder(resp.Body).Decode(&cfg)

	found := false
	for _, task := range cfg.Tasks {
		if task.Title == "Persisted task" {
			found = true
			if task.ID != "4" {
				t.Errorf("expected ID 4, got %s", task.ID)
			}
			break
		}
	}
	if !found {
		t.Error("created task not found in GET /api/config response")
	}
}

func TestNextTaskID(t *testing.T) {
	tests := []struct {
		name  string
		tasks []config.Task
		want  string
	}{
		{"empty", nil, "1"},
		{"sequential", []config.Task{{ID: "1"}, {ID: "2"}, {ID: "3"}}, "4"},
		{"gap", []config.Task{{ID: "1"}, {ID: "5"}}, "6"},
		{"non-numeric", []config.Task{{ID: "abc"}, {ID: "2"}}, "3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextTaskID(tt.tasks)
			if got != tt.want {
				t.Errorf("nextTaskID() = %s, want %s", got, tt.want)
			}
		})
	}
}
