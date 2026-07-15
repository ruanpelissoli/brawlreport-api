// Package api contains the HTTP server and route handlers for the BrawlReport
// backend API. It uses only stdlib net/http — no router framework dependency.
package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

const (
	readTimeout  = 5 * time.Second
	writeTimeout = 10 * time.Second
	idleTimeout  = 60 * time.Second
)

// Server wraps an http.Server with a configured ServeMux.
type Server struct {
	srv    *http.Server
	logger *slog.Logger
}

// NewServer constructs a Server with all routes registered.
// Pass additional route setup functions via opts to extend the mux.
func NewServer(logger *slog.Logger) *Server {
	mux := http.NewServeMux()

	s := &Server{
		srv: &http.Server{
			Handler:      mux,
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
			IdleTimeout:  idleTimeout,
		},
		logger: logger,
	}

	// Register built-in routes.
	mux.HandleFunc("GET /health", healthHandler)

	return s
}

// Start begins listening on addr (e.g. ":8080") and blocks until the server
// returns. It logs the listen address before blocking.
func (s *Server) Start(addr string) error {
	s.srv.Addr = addr
	s.logger.Info("HTTP server listening", "addr", addr)
	return s.srv.ListenAndServe()
}

// Shutdown gracefully stops the server within the given timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

// Addr returns the configured listen address string.
func (s *Server) Addr() string {
	return fmt.Sprintf(":%s", s.srv.Addr)
}
