package postgres

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	defaultMaxPoolSize     = 10
	defaultConnAttempts    = 10
	defaultConnTimeout     = time.Second
	defaultConnMaxLifetime = time.Hour
)

type Postgres struct {
	maxPoolSize     int
	connAttempts    int
	connTimeout     time.Duration
	connMaxLifetime time.Duration

	DB  *sql.DB
	log *slog.Logger
}

func New(ctx context.Context, dbURL string, log *slog.Logger, opts ...Option) (*Postgres, error) {
	pg := &Postgres{
		maxPoolSize:     defaultMaxPoolSize,
		connAttempts:    defaultConnAttempts,
		connTimeout:     defaultConnTimeout,
		connMaxLifetime: defaultConnMaxLifetime,
		log:             log,
	}

	for _, opt := range opts {
		opt(pg)
	}

	var err error
	for pg.connAttempts > 0 {
		pg.DB, err = sql.Open("pgx", dbURL)
		if err == nil {
			err = pg.DB.Ping()
			if err != nil {
				pg.DB.Close()
				log.Error("failed to ping database", slog.Any("error", err))
				return nil, err
			}

			pg.DB.SetConnMaxLifetime(pg.connMaxLifetime)
			pg.DB.SetMaxOpenConns(pg.maxPoolSize)
			break
		}

		pg.connAttempts--
		log.Info("postgres is trying to connect", slog.Any("attempts left", pg.connAttempts))
		time.Sleep(pg.connTimeout)
	}
	if err != nil {
		log.Error("failed to connect to database", slog.Any("error", err))
		return nil, err
	}

	return pg, nil
}

func (p *Postgres) Close() {
	if err := p.DB.Close(); err != nil {
		p.log.Error("failed to close database", slog.Any("error", err))
	}
}
