package httpinfra

import (
	"context"
	"log"
	"net/http"
	"time"
)

type Config struct {
	Address         string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

type Server struct {
	httpServer      *http.Server
	shutdownTimeout time.Duration
}

// NewServer creates HTTP server infra (pure transport layer)
func NewServer(cfg Config, handler http.Handler) *Server {

	// default safe values
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 10 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 10 * time.Second
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = 60 * time.Second
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = 5 * time.Second
	}

	server := &http.Server{
		Addr:         cfg.Address,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	return &Server{
		httpServer:      server,
		shutdownTimeout: cfg.ShutdownTimeout,
	}
}

// Start runs HTTP server
func (s *Server) Start() error {
	log.Printf("[HTTP] starting server on %s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down server
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()

	log.Println("[HTTP] shutting down server...")
	return s.httpServer.Shutdown(ctx)
}
