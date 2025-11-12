package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudyy74/pr-reviewer-service/internal/app"
	"github.com/cloudyy74/pr-reviewer-service/internal/config"
)

func main() {
	cfg := config.MustLoadConfig()
	log := newLogger(cfg.Env)
	log.Debug("debug messages are enabled")

	app, err := app.NewApp(cfg, log)
	if err != nil {
		panic(err)
	}

	go app.MustRun()

	notifyCh := make(chan os.Signal, 1)
	signal.Notify(notifyCh, syscall.SIGINT, os.Interrupt)

	<-notifyCh
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	app.Close(ctx)
}

func newLogger(env string) *slog.Logger {
	var log *slog.Logger

	opts := &slog.HandlerOptions{AddSource: true}

	switch env {
	case "local":
		opts.Level = slog.LevelDebug
		log = slog.New(slog.NewTextHandler(os.Stdout, opts))
	case "dev":
		opts.Level = slog.LevelDebug
		log = slog.New(slog.NewJSONHandler(os.Stdout, opts))
	case "prod":
		opts.Level = slog.LevelInfo
		log = slog.New(slog.NewJSONHandler(os.Stdout, opts))
	default:
		panic("unknown env")
	}

	return log
}
