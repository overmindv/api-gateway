package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/overmindv/laserbeak/internal/client/arcee"
	"github.com/overmindv/laserbeak/internal/config"
	"github.com/overmindv/laserbeak/internal/middleware"
	"github.com/overmindv/laserbeak/internal/server"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		log.Error("load configuration", "error", err)
		os.Exit(1)
	}

	users := arcee.New(cfg.Arcee, log)
	authenticator := middleware.NewJWTAuthenticator(cfg.JWT.Secret, cfg.JWT.Issuer)
	httpServer := server.New(cfg.HTTP, users, users, authenticator, log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := httpServer.Run(ctx); err != nil {
		log.Error("run laserbeak", "error", err)
		os.Exit(1)
	}
}
