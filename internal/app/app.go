package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/cloudyy74/pr-reviewer-service/internal/config"
	router "github.com/cloudyy74/pr-reviewer-service/internal/http"
	"github.com/cloudyy74/pr-reviewer-service/internal/service"
	"github.com/cloudyy74/pr-reviewer-service/internal/storage"
	"github.com/cloudyy74/pr-reviewer-service/pkg/postgres"
)

const (
	defaultAddr = "localhost:8080"
)

type App struct {
	httpServer *http.Server
	addr       string
	database   *postgres.Postgres
	log        *slog.Logger
}

func NewApp(cfg *config.Config, log *slog.Logger) (*App, error) {
	if cfg.Addr == "" {
		cfg.Addr = defaultAddr
	}
	if cfg.DBURL == "" {
		return nil, errors.New("database url cannot be empty")
	}

	ctx := context.Background()
	database, err := postgres.New(ctx, cfg.DBURL, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	teamStorage, err := storage.NewTeamStorage(database, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create team storage: %w", err)
	}
	userStorage, err := storage.NewUserStorage(database, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create user storage: %w", err)
	}
	prStorage, err := storage.NewPRStorage(database, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create pr storage: %w", err)
	}
	txManager, err := storage.NewTxManager(database, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create tx manager: %w", err)
	}

	teamService, err := service.NewTeamService(txManager, teamStorage, userStorage, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create team service: %w", err)
	}
	userService, err := service.NewUserService(txManager, userStorage, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create user service: %w", err)
	}
	prService, err := service.NewPRService(txManager, prStorage, userStorage, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create pr service: %w", err)
	}

	_, port, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("invalid port in config: %w", err)
	}

	mux := http.NewServeMux()
	if err := router.SetupRouter(mux, port, teamService, userService, prService, log); err != nil {
		return nil, fmt.Errorf("failed to create router: %w", err)
	}
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: cfg.Timeout,
		ReadTimeout:       cfg.Timeout,
		WriteTimeout:      cfg.Timeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	return &App{
		httpServer: httpServer,
		addr:       cfg.Addr,
		database:   database,
		log:        log,
	}, nil
}

func (a *App) Run() error {
	a.log.Info("starting http server", slog.String("port", a.addr))
	return a.httpServer.ListenAndServe()
}

func (a *App) MustRun() {
	if err := a.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		a.log.Error("failed to run http server", slog.Any("error", err))
		panic(err)
	}
}

func (a *App) Close(ctx context.Context) {
	a.database.Close()
	a.log.Info("trying to shutdown server")
	if err := a.httpServer.Shutdown(ctx); err != nil {
		a.log.Warn("failed to close http server", slog.Any("error", err))
	}
}
