package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/overmindv/laserbeak/internal/client/arcee"
	"github.com/overmindv/laserbeak/internal/config"
	graphqldelivery "github.com/overmindv/laserbeak/internal/graphql"
	"github.com/overmindv/laserbeak/internal/middleware"
)

type Server struct {
	config config.HTTP
	log    *slog.Logger
	http   *http.Server
}

type healthChecker interface{ Health(context.Context) error }

func New(cfg config.HTTP, users arcee.UserService, health healthChecker, authenticator *middleware.JWTAuthenticator, log *slog.Logger) *Server {
	mux := http.NewServeMux()

	mux.Handle("POST /graphql", graphqldelivery.Handler(users, log))
	mux.Handle("GET /playground", graphqldelivery.Playground())

	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := health.Health(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"unhealthy","arcee":"unavailable"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","arcee":"ok"}`))
	}

	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /healthz", healthHandler)

	var handler http.Handler = mux
	handler = middleware.JWT(authenticator, handler)
	handler = middleware.CORS(cfg.CORSOrigins, handler)
	handler = middleware.Logging(log, handler)
	handler = middleware.RequestIDMiddleware(handler)

	return &Server{
		config: cfg,
		log:    log,
		http: &http.Server{
			Handler:      handler,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
	}
}

func (s *Server) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.config.Address)
	if err != nil {
		return fmt.Errorf("listen HTTP: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		s.log.Info("laserbeak HTTP server started", "address", s.config.Address)
		if err := s.http.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("serve HTTP: %w", err)
		}
	}()

	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-errCh:
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer cancel()
	if err := s.http.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP: %w", err)
	}

	return runErr
}
