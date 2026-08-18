package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/overmindv/api-gateway/internal/client/entities"
	mediaclient "github.com/overmindv/api-gateway/internal/client/media"
	"github.com/overmindv/api-gateway/internal/client/taskhunter"
	"github.com/overmindv/api-gateway/internal/client/tasks"
	"github.com/overmindv/api-gateway/internal/client/users"
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

	usersService := users.New(cfg.Users, requestLog)
	entitiesService := entities.New(cfg.Entities.URL, cfg.Entities.Timeout, requestLog)
	tasksService := tasks.New(cfg.Tasks.URL, cfg.Tasks.Timeout, requestLog)
	taskHunterService := taskhunter.New(cfg.TaskHunter.URL, cfg.TaskHunter.Token, cfg.TaskHunter.Timeout, requestLog)
	mediaService := mediaclient.New(cfg.Media.URL, cfg.Media.Token, cfg.Media.Timeout, requestLog)
	authenticator := middleware.NewJWTAuthenticator(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.AdminUserIDs)
	httpServer := server.NewWithMedia(cfg.HTTP, usersService, entitiesService, tasksService, taskHunterService, mediaService, usersService, authenticator, log, requestLog)

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
