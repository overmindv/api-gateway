package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/overmindv/api-gateway/internal/client/entities"
	"github.com/overmindv/api-gateway/internal/client/media"
	"github.com/overmindv/api-gateway/internal/client/taskhunter"
	"github.com/overmindv/api-gateway/internal/client/tasks"
	"github.com/overmindv/api-gateway/internal/client/users"
	"github.com/overmindv/api-gateway/internal/config"
	graphqldelivery "github.com/overmindv/api-gateway/internal/graphql"
	"github.com/overmindv/api-gateway/internal/middleware"
)

type Server struct {
	config config.HTTP
	log    *slog.Logger
	http   *http.Server
}

type healthChecker interface{ Health(context.Context) error }

// New собирает HTTP server api-gateway со всеми middleware и routes.
// На вход получает конфигурацию, clients, health checker, authenticator и loggers, на выход возвращает готовый Server.
func New(cfg config.HTTP, users users.UserService, catalog entities.CatalogService, tasks tasks.Service, taskHunter taskhunter.Service, usersHealth healthChecker, authenticator *middleware.JWTAuthenticator, log *slog.Logger, requestLog *slog.Logger) *Server {
	return NewWithMedia(cfg, users, catalog, tasks, taskHunter, nil, usersHealth, authenticator, log, requestLog)
}

// NewWithMedia собирает gateway server с Media client и сохраняет совместимость старого New для тестов.
func NewWithMedia(cfg config.HTTP, users users.UserService, catalog entities.CatalogService, tasks tasks.Service, taskHunter taskhunter.Service, mediaSvc media.Service, usersHealth healthChecker, authenticator *middleware.JWTAuthenticator, log *slog.Logger, requestLog *slog.Logger) *Server {
	mux := http.NewServeMux()
	metrics := &graphqldelivery.Metrics{}

	mux.Handle("POST /graphql", graphqldelivery.HandlerWithMediaAndMetrics(users, catalog, tasks, taskHunter, mediaSvc, requestLog, metrics))
	mux.Handle("GET /playground", graphqldelivery.Playground())
	mux.HandleFunc("GET /metrics", metrics.Handler)

	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := usersHealth.Health(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"unhealthy","users":"unavailable"}`))
			return
		}
		if err := tasks.Health(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"unhealthy","tasks":"unavailable"}`))
			return
		}
		if err := taskHunter.Health(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"unhealthy","task_hunter":"unavailable"}`))
			return
		}
		if mediaSvc != nil {
			if err := mediaSvc.Health(r.Context()); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"status":"unhealthy","media":"unavailable"}`))
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","users":"ok","tasks":"ok","task_hunter":"ok"}`))
	}

	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /healthz", healthHandler)

	var handler http.Handler = mux
	handler = middleware.JWT(authenticator, handler)
	handler = middleware.CORS(cfg.CORSOrigins, handler)
	handler = middleware.Logging(requestLog, handler, "/health", "/healthz", "/metrics")
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

// Run запускает HTTP listener и корректно завершает server по context cancellation.
// На вход получает context жизненного цикла, на выход возвращает ошибку запуска или shutdown.
func (s *Server) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.config.Address)
	if err != nil {
		return fmt.Errorf("listen HTTP: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		s.log.Info("api-gateway HTTP server started", "address", s.config.Address)
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
