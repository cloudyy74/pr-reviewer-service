package storage

import (
	"fmt"
	"context"
	"errors"
	"log/slog"

	"github.com/cloudyy74/pr-reviewer-service/pkg/postgres"
)

var (
	ErrTeamExists = errors.New("team already exists")
)

type TeamStorage struct {
	db  *postgres.Postgres
	log *slog.Logger
}

func NewTeamStorage(db *postgres.Postgres, log *slog.Logger) (*TeamStorage, error) {
	if db == nil {
		return nil, errors.New("database cannot be nil")
	}
	if log == nil {
		return nil, errors.New("logger cannot be nil")
	}
	return &TeamStorage{
		db:  db,
		log: log,
	}, nil
}

func (s *TeamStorage) CreateTeam(ctx context.Context, teamName string) error {
	exec := getExecer(ctx, s.db.DB)
	res, err := exec.ExecContext(
		ctx,
		"insert into teams (name) values ($1) on conflict (name) do nothing",
		teamName,
	)
	if err != nil {
		s.log.Error("failed to create team", slog.Any("error", err))
		return fmt.Errorf("insert team %q: %w", teamName, err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		s.log.Error("failed check rows affected", slog.Any("error", err))
		return fmt.Errorf("rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("insert team: %w", ErrTeamExists)
	}

	return nil
}

func (s *TeamStorage) ExistsTeam(ctx context.Context, name string) (bool, error) {
	exec := getQueryExecer(ctx, s.db.DB)
    var exists bool

    err := exec.QueryRowContext(
        ctx,
        `SELECT EXISTS(
            SELECT 1 FROM teams WHERE name = $1
        )`,
        name,
    ).Scan(&exists)
    if err != nil {
        return false, fmt.Errorf("check team exists: %w", err)
    }

    return exists, nil
}
