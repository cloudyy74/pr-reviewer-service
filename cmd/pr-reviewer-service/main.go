package main

import (
	"log/slog"
	"os"

	"github.com/cloudyy74/pr-reviewer-service/internal/config"
)

func main() {
	config := config.MustLoadConfig()
	log := newLogger(config.Env)
	log.Info("starting pr-reviewer service", slog.String("env", config.Env))
	log.Debug("debug messages are enabled")
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
