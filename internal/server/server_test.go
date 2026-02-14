package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tmdgusya/do-more/internal/config"
	"github.com/tmdgusya/do-more/internal/provider"
)

type mockTestProvider struct{ name string }

func (m *mockTestProvider) Name() string { return m.name }
func (m *mockTestProvider) Run(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}

func setupTestServer(t *testing.T) (*httptest.Server, *Server, string) {
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

	srv := NewServer(cfgPath, dir, registry)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, srv, cfgPath
}

func TestGetConfig(t *testing.T) {
	ts, _, _ := setupTestServer(t)

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
	ts, _, _ := setupTestServer(t)

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
	ts, _, cfgPath := setupTestServer(t)

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
	ts, _, _ := setupTestServer(t)

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
	ts, _, _ := setupTestServer(t)

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
	ts, _, cfgPath := setupTestServer(t)

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
	ts, _, _ := setupTestServer(t)

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
	ts, _, _ := setupTestServer(t)

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
	ts, _, cfgPath := setupTestServer(t)

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
	ts, _, _ := setupTestServer(t)

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
	ts, _, _ := setupTestServer(t)

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
	ts, _, cfgPath := setupTestServer(t)

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
	ts, _, cfgPath := setupTestServer(t)

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
	ts, _, _ := setupTestServer(t)

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
	ts, _, _ := setupTestServer(t)

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

func TestSSEHeaders(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", got)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}
	if got := resp.Header.Get("Connection"); got != "keep-alive" {
		t.Errorf("Connection = %q, want keep-alive", got)
	}
}

func TestSSEDeliverEvents(t *testing.T) {
	ts, srv, _ := setupTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var received []Event
	var mu sync.Mutex
	done := make(chan struct{})

	go func() {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			var ev Event
			if err := json.Unmarshal([]byte(data), &ev); err != nil {
				continue
			}
			mu.Lock()
			received = append(received, ev)
			if len(received) >= 2 {
				mu.Unlock()
				close(done)
				return
			}
			mu.Unlock()
		}
	}()

	time.Sleep(50 * time.Millisecond)

	srv.Hub().Broadcast(Event{
		Type:      "test_event",
		TaskID:    "42",
		Data:      map[string]any{"key": "value1"},
		Timestamp: time.Now(),
	})
	srv.Hub().Broadcast(Event{
		Type:      "test_event",
		TaskID:    "43",
		Data:      map[string]any{"key": "value2"},
		Timestamp: time.Now(),
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SSE events")
	}

	mu.Lock()
	defer mu.Unlock()

	if len(received) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(received))
	}
	if received[0].Type != "test_event" || received[0].TaskID != "42" {
		t.Errorf("event[0] = %+v, want type=test_event taskId=42", received[0])
	}
	if received[1].Type != "test_event" || received[1].TaskID != "43" {
		t.Errorf("event[1] = %+v, want type=test_event taskId=43", received[1])
	}
}

func TestSSEMultipleClients(t *testing.T) {
	ts, srv, _ := setupTestServer(t)

	connectSSE := func() (*http.Response, context.CancelFunc) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp, cancel
	}

	resp1, cancel1 := connectSSE()
	defer cancel1()
	defer resp1.Body.Close()

	resp2, cancel2 := connectSSE()
	defer cancel2()
	defer resp2.Body.Close()

	time.Sleep(50 * time.Millisecond)

	srv.Hub().Broadcast(Event{
		Type:      "multi_test",
		Timestamp: time.Now(),
	})

	readOne := func(resp *http.Response) (Event, error) {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				var ev Event
				json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev)
				return ev, nil
			}
		}
		return Event{}, scanner.Err()
	}

	ch1 := make(chan Event, 1)
	ch2 := make(chan Event, 1)
	go func() { ev, _ := readOne(resp1); ch1 <- ev }()
	go func() { ev, _ := readOne(resp2); ch2 <- ev }()

	select {
	case ev := <-ch1:
		if ev.Type != "multi_test" {
			t.Errorf("client1: got type %q, want multi_test", ev.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("client1: timed out")
	}

	select {
	case ev := <-ch2:
		if ev.Type != "multi_test" {
			t.Errorf("client2: got type %q, want multi_test", ev.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("client2: timed out")
	}
}

func TestSSEClientDisconnectCleansUp(t *testing.T) {
	_, srv, _ := setupTestServer(t)

	before := subscriberCount(srv.Hub())

	ch := srv.Hub().Subscribe()
	during := subscriberCount(srv.Hub())
	if during != before+1 {
		t.Errorf("expected %d subscribers during, got %d", before+1, during)
	}

	srv.Hub().Unsubscribe(ch)
	after := subscriberCount(srv.Hub())
	if after != before {
		t.Errorf("expected %d subscribers after cleanup, got %d", before, after)
	}
}

func subscriberCount(h *EventHub) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscribers)
}

type slowMockProvider struct {
	name string
}

func (m *slowMockProvider) Name() string { return m.name }
func (m *slowMockProvider) Run(ctx context.Context, _ string, _ string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(5 * time.Second):
		return "done", nil
	}
}

func setupLoopTestServer(t *testing.T, tasks []config.Task) (*httptest.Server, *Server, string) {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "do-more.json")

	cfg := &config.Config{
		Name:          "loop-test",
		Provider:      "slow",
		Branch:        "main",
		Gates:         []string{},
		MaxIterations: 3,
		Tasks:         tasks,
	}
	if err := config.SaveConfig(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}

	registry := provider.NewProviderRegistry()
	registry.Register(&slowMockProvider{name: "slow"})

	srv := NewServer(cfgPath, dir, registry)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		srv.mu.Lock()
		if srv.loopCancel != nil {
			srv.loopCancel()
		}
		srv.mu.Unlock()
		srv.loopWg.Wait()
		ts.Close()
	})
	return ts, srv, cfgPath
}

func TestLoopStart(t *testing.T) {
	tasks := []config.Task{
		{ID: "1", Title: "Task one", Description: "Do it", Status: config.StatusPending},
	}
	ts, _, _ := setupLoopTestServer(t, tasks)

	resp, err := http.Post(ts.URL+"/api/loop/start", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != "started" {
		t.Errorf("expected status started, got %s", result["status"])
	}
}

func TestLoopStartNoPendingTasks(t *testing.T) {
	tasks := []config.Task{
		{ID: "1", Title: "Done task", Description: "Already done", Status: config.StatusDone},
	}
	ts, _, _ := setupLoopTestServer(t, tasks)

	resp, err := http.Post(ts.URL+"/api/loop/start", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != "completed" {
		t.Errorf("expected status completed, got %s", result["status"])
	}
	if result["message"] != "no pending tasks" {
		t.Errorf("expected 'no pending tasks', got %s", result["message"])
	}
}

func TestLoopDoubleStart(t *testing.T) {
	tasks := []config.Task{
		{ID: "1", Title: "Task one", Description: "Do it", Status: config.StatusPending},
	}
	ts, _, _ := setupLoopTestServer(t, tasks)

	resp1, err := http.Post(ts.URL+"/api/loop/start", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first start: expected 200, got %d", resp1.StatusCode)
	}

	time.Sleep(50 * time.Millisecond)

	resp2, err := http.Post(ts.URL+"/api/loop/start", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusConflict {
		t.Fatalf("second start: expected 409, got %d", resp2.StatusCode)
	}

	var errResp map[string]string
	json.NewDecoder(resp2.Body).Decode(&errResp)
	if errResp["error"] != "loop already running" {
		t.Errorf("expected 'loop already running', got %s", errResp["error"])
	}
}

func TestLoopStop(t *testing.T) {
	tasks := []config.Task{
		{ID: "1", Title: "Task one", Description: "Do it", Status: config.StatusPending},
	}
	ts, srv, _ := setupLoopTestServer(t, tasks)

	resp, err := http.Post(ts.URL+"/api/loop/start", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	time.Sleep(50 * time.Millisecond)

	srv.mu.Lock()
	running := srv.loopRunning
	srv.mu.Unlock()
	if !running {
		t.Fatal("loop should be running before stop")
	}

	resp, err = http.Post(ts.URL+"/api/loop/stop", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != "stopped" {
		t.Errorf("expected status stopped, got %s", result["status"])
	}
}

func TestLoopStopNotRunning(t *testing.T) {
	tasks := []config.Task{
		{ID: "1", Title: "Task one", Description: "Do it", Status: config.StatusPending},
	}
	ts, _, _ := setupLoopTestServer(t, tasks)

	resp, err := http.Post(ts.URL+"/api/loop/stop", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 (idempotent), got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != "stopped" {
		t.Errorf("expected status stopped, got %s", result["status"])
	}
}

func TestLoopStatus(t *testing.T) {
	tasks := []config.Task{
		{ID: "1", Title: "Task one", Description: "Do it", Status: config.StatusPending},
	}
	ts, _, _ := setupLoopTestServer(t, tasks)

	resp, err := http.Get(ts.URL + "/api/loop/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	if result["running"] != false {
		t.Errorf("expected running=false, got %v", result["running"])
	}

	resp2, err := http.Post(ts.URL+"/api/loop/start", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()

	time.Sleep(50 * time.Millisecond)

	resp3, err := http.Get(ts.URL + "/api/loop/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()

	var result2 map[string]any
	json.NewDecoder(resp3.Body).Decode(&result2)
	if result2["running"] != true {
		t.Errorf("expected running=true after start, got %v", result2["running"])
	}
}
