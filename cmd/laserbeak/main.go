package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/overmindv/api-gateway/internal/client/arcee"
	"github.com/overmindv/api-gateway/internal/client/ironhide"
	"github.com/overmindv/api-gateway/internal/client/tasksit"
	"github.com/overmindv/api-gateway/internal/config"
	"github.com/overmindv/api-gateway/internal/middleware"
	"github.com/overmindv/api-gateway/internal/server"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		log.Error("load configuration", "error", err)
		os.Exit(1)
	}

	requestLog, closeRequestLog, err := requestLogger(cfg.HTTP.RequestLogPath, log)
	if err != nil {
		log.Error("initialize request logger", "error", err)
		os.Exit(1)
	}
	defer closeRequestLog()

	users := arcee.New(cfg.Arcee, requestLog)
	catalog := ironhide.New(cfg.Ironhide.URL, cfg.Ironhide.Timeout, requestLog)
	tasks := tasksit.New(cfg.TasksIT.URL, cfg.TasksIT.Timeout, requestLog)
	authenticator := middleware.NewJWTAuthenticator(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.AdminUserIDs)
	httpServer := server.New(cfg.HTTP, users, catalog, tasks, users, authenticator, log, requestLog)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := httpServer.Run(ctx); err != nil {
		log.Error("run api-gateway", "error", err)
		os.Exit(1)
	}
}

// requestLogger создаёт отдельный JSON-логгер для пользовательских HTTP-запросов
// и upstream-вызовов. Если путь не задан, используется основной stdout logger.
func requestLogger(path string, fallback *slog.Logger) (*slog.Logger, func(), error) {
	if path == "" {
		return fallback, func() {}, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, nil, fmt.Errorf("create request log directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open request log file: %w", err)
	}

	return slog.New(slog.NewJSONHandler(file, nil)), func() { _ = file.Close() }, nil
}
