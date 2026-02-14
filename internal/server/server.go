package server

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"
	"sync"

	"github.com/tmdgusya/do-more/internal/config"
	"github.com/tmdgusya/do-more/internal/provider"
)

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	mu          sync.Mutex
	cfgPath     string
	registry    *provider.ProviderRegistry
	loopRunning bool
	loopCancel  context.CancelFunc
	mux         *http.ServeMux
	httpServer  *http.Server
}

func NewServer(cfgPath string, registry *provider.ProviderRegistry) *Server {
	mux := http.NewServeMux()

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	s := &Server{
		cfgPath:  cfgPath,
		registry: registry,
		mux:      mux,
	}

	mux.HandleFunc("GET /api/config", s.handleGetConfig)
	mux.HandleFunc("PUT /api/config", s.handleUpdateConfig)
	mux.HandleFunc("GET /api/providers", s.handleGetProviders)
	mux.HandleFunc("POST /api/tasks", s.handleCreateTask)
	mux.HandleFunc("PUT /api/tasks/{id}", s.handleUpdateTask)
	mux.HandleFunc("DELETE /api/tasks/{id}", s.handleDeleteTask)

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
