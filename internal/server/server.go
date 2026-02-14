package server

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"sync"

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

// NewServer creates a new Server instance with embedded static files
func NewServer(cfgPath string, registry *provider.ProviderRegistry) *Server {
	mux := http.NewServeMux()

	// Mount embedded static files at /
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	return &Server{
		cfgPath:  cfgPath,
		registry: registry,
		mux:      mux,
	}
}

// ListenAndServe starts the HTTP server on the given address
func (s *Server) ListenAndServe(addr string) error {
	s.mu.Lock()
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.mux,
	}
	s.mu.Unlock()

	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.httpServer == nil {
		return nil
	}

	return s.httpServer.Shutdown(ctx)
}
