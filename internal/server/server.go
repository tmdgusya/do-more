package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/tmdgusya/do-more/internal/config"
	"github.com/tmdgusya/do-more/internal/loop"
	"github.com/tmdgusya/do-more/internal/provider"
)

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	mu          sync.Mutex
	cfgPath     string
	workDir     string
	registry    *provider.ProviderRegistry
	loopRunning bool
	loopCancel  context.CancelFunc
	loopWg      sync.WaitGroup
	hub         *EventHub
	mux         *http.ServeMux
	httpServer  *http.Server
}

func NewServer(cfgPath string, workDir string, registry *provider.ProviderRegistry) *Server {
	mux := http.NewServeMux()

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	s := &Server{
		cfgPath:  cfgPath,
		workDir:  workDir,
		registry: registry,
		hub:      NewEventHub(),
		mux:      mux,
	}

	mux.HandleFunc("GET /api/config", s.handleGetConfig)
	mux.HandleFunc("PUT /api/config", s.handleUpdateConfig)
	mux.HandleFunc("GET /api/providers", s.handleGetProviders)
	mux.HandleFunc("POST /api/tasks", s.handleCreateTask)
	mux.HandleFunc("PUT /api/tasks/{id}", s.handleUpdateTask)
	mux.HandleFunc("DELETE /api/tasks/{id}", s.handleDeleteTask)
	mux.HandleFunc("GET /api/events", s.handleSSE)
	mux.HandleFunc("POST /api/loop/start", s.handleLoopStart)
	mux.HandleFunc("POST /api/loop/stop", s.handleLoopStop)
	mux.HandleFunc("POST /api/loop/skip", s.handleLoopSkip)
	mux.HandleFunc("GET /api/loop/status", s.handleLoopStatus)

	return s
}

func (s *Server) ListenAndServe(addr string) error {
	s.mu.Lock()
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.mux,
	}
	s.mu.Unlock()

	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.httpServer == nil {
		return nil
	}

	return s.httpServer.Shutdown(ctx)
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

// Hub returns the server's EventHub for broadcasting events.
func (s *Server) Hub() *EventHub {
	return s.hub
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	ch := s.hub.Subscribe()
	defer s.hub.Unsubscribe(ch)

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", event.JSON())
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	cfg, err := config.LoadConfig(s.cfgPath)
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load config")
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) handleGetProviders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.registry.List())
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Provider    string `json:"provider"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if input.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cfg, err := config.LoadConfig(s.cfgPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load config")
		return
	}

	task := config.Task{
		ID:          nextTaskID(cfg.Tasks),
		Title:       input.Title,
		Description: input.Description,
		Status:      config.StatusPending,
		Provider:    input.Provider,
	}
	cfg.Tasks = append(cfg.Tasks, task)

	if err := config.SaveConfig(s.cfgPath, cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}

	writeJSON(w, http.StatusCreated, task)
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var input struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Provider    string `json:"provider"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cfg, err := config.LoadConfig(s.cfgPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load config")
		return
	}

	for i := range cfg.Tasks {
		if cfg.Tasks[i].ID == id {
			if cfg.Tasks[i].Status == config.StatusInProgress {
				writeError(w, http.StatusConflict, "cannot modify in_progress task")
				return
			}
			if input.Title != "" {
				cfg.Tasks[i].Title = input.Title
			}
			if input.Description != "" {
				cfg.Tasks[i].Description = input.Description
			}
			if input.Provider != "" {
				cfg.Tasks[i].Provider = input.Provider
			}
			if err := config.SaveConfig(s.cfgPath, cfg); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to save config")
				return
			}
			writeJSON(w, http.StatusOK, cfg.Tasks[i])
			return
		}
	}

	writeError(w, http.StatusNotFound, "task not found")
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	s.mu.Lock()
	defer s.mu.Unlock()

	cfg, err := config.LoadConfig(s.cfgPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load config")
		return
	}

	for i := range cfg.Tasks {
		if cfg.Tasks[i].ID == id {
			if cfg.Tasks[i].Status == config.StatusInProgress {
				writeError(w, http.StatusConflict, "cannot delete in_progress task")
				return
			}
			cfg.Tasks = append(cfg.Tasks[:i], cfg.Tasks[i+1:]...)
			if err := config.SaveConfig(s.cfgPath, cfg); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to save config")
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	writeError(w, http.StatusNotFound, "task not found")
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Provider      string   `json:"provider"`
		Branch        string   `json:"branch"`
		Gates         []string `json:"gates"`
		MaxIterations *int     `json:"maxIterations"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cfg, err := config.LoadConfig(s.cfgPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load config")
		return
	}

	if input.Provider != "" {
		cfg.Provider = input.Provider
	}
	if input.Branch != "" {
		cfg.Branch = input.Branch
	}
	if input.Gates != nil {
		cfg.Gates = input.Gates
	}
	if input.MaxIterations != nil {
		cfg.MaxIterations = *input.MaxIterations
	}

	if err := config.SaveConfig(s.cfgPath, cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}

	writeJSON(w, http.StatusOK, cfg)
}

func nextTaskID(tasks []config.Task) string {
	maxID := 0
	for _, t := range tasks {
		if n, err := strconv.Atoi(t.ID); err == nil && n > maxID {
			maxID = n
		}
	}
	return strconv.Itoa(maxID + 1)
}

func (s *Server) handleLoopStart(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if s.loopRunning {
		s.mu.Unlock()
		writeError(w, http.StatusConflict, "loop already running")
		return
	}

	cfg, err := config.LoadConfig(s.cfgPath)
	if err != nil {
		s.mu.Unlock()
		writeError(w, http.StatusInternalServerError, "failed to load config")
		return
	}
	if cfg.NextPendingTask() == nil {
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]string{"status": "completed", "message": "no pending tasks"})
		return
	}
	if cfg.MaxIterations < 1 {
		s.mu.Unlock()
		writeError(w, http.StatusBadRequest, "maxIterations must be >= 1")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.loopCancel = cancel
	s.loopRunning = true
	s.mu.Unlock()

	s.hub.Broadcast(Event{
		Type:      EventLoopStarted,
		Data:      map[string]any{"provider": cfg.Provider},
		Timestamp: time.Now(),
	})

	s.loopWg.Add(1)
	go func() {
		defer s.loopWg.Done()
		logger := NewEventLogger(s.hub)
		err := loop.RunLoop(ctx, s.cfgPath, cfg.Provider, s.registry, s.workDir, logger)

		s.mu.Lock()
		s.loopRunning = false
		s.loopCancel = nil
		s.mu.Unlock()

		if err != nil {
			s.hub.Broadcast(Event{
				Type:      EventLoopError,
				Data:      map[string]any{"error": err.Error()},
				Timestamp: time.Now(),
			})
		} else {
			s.hub.Broadcast(Event{
				Type:      EventLoopCompleted,
				Timestamp: time.Now(),
			})
		}
	}()

	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func (s *Server) handleLoopStop(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if s.loopCancel != nil {
		s.loopCancel()
		s.loopRunning = false
		s.loopCancel = nil
	}
	s.mu.Unlock()

	s.hub.Broadcast(Event{
		Type:      EventLoopStopped,
		Timestamp: time.Now(),
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (s *Server) handleLoopSkip(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if !s.loopRunning {
		s.mu.Unlock()
		writeError(w, http.StatusConflict, "no loop running")
		return
	}

	cfg, err := config.LoadConfig(s.cfgPath)
	if err != nil {
		s.mu.Unlock()
		writeError(w, http.StatusInternalServerError, "failed to load config")
		return
	}
	for i := range cfg.Tasks {
		if cfg.Tasks[i].Status == config.StatusInProgress {
			cfg.Tasks[i].Status = config.StatusFailed
			cfg.Tasks[i].Learnings += "\nSkipped by user via dashboard"

			s.hub.Broadcast(Event{
				Type:      EventTaskFailed,
				TaskID:    cfg.Tasks[i].ID,
				Data:      map[string]any{"reason": "skipped"},
				Timestamp: time.Now(),
			})
			break
		}
	}
	config.SaveConfig(s.cfgPath, cfg)

	if s.loopCancel != nil {
		s.loopCancel()
	}
	s.loopRunning = false
	s.loopCancel = nil
	s.mu.Unlock()

	cfg2, _ := config.LoadConfig(s.cfgPath)
	if cfg2 != nil && cfg2.NextPendingTask() != nil {
		s.mu.Lock()
		ctx, cancel := context.WithCancel(context.Background())
		s.loopCancel = cancel
		s.loopRunning = true
		s.mu.Unlock()

		s.loopWg.Add(1)
		go func() {
			defer s.loopWg.Done()
			logger := NewEventLogger(s.hub)
			err := loop.RunLoop(ctx, s.cfgPath, cfg2.Provider, s.registry, s.workDir, logger)
			s.mu.Lock()
			s.loopRunning = false
			s.loopCancel = nil
			s.mu.Unlock()
			if err != nil {
				s.hub.Broadcast(Event{Type: EventLoopError, Data: map[string]any{"error": err.Error()}, Timestamp: time.Now()})
			} else {
				s.hub.Broadcast(Event{Type: EventLoopCompleted, Timestamp: time.Now()})
			}
		}()
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "skipped"})
}

func (s *Server) handleLoopStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	running := s.loopRunning
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{"running": running})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
