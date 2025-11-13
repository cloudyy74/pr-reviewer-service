package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/cloudyy74/pr-reviewer-service/internal/models"
	"github.com/cloudyy74/pr-reviewer-service/pkg/postgres"
)

type UserStorage struct {
	db  *postgres.Postgres
	log *slog.Logger
}

func NewUserStorage(db *postgres.Postgres, log *slog.Logger) (*UserStorage, error) {
	if db == nil {
		return nil, errors.New("database cannot be nil")
	}
	if log == nil {
		return nil, errors.New("logger cannot be nil")
	}
	return &UserStorage{
		db:  db,
		log: log,
	}, nil
}

func (s *UserStorage) UpsertUser(ctx context.Context, u models.User, teamName string) error {
	_, err := s.db.DB.ExecContext(
		ctx,
		`
insert into users (id, username, team_name, is_active) values ($1, $2, $3, $4) on conflict (id) do update set
username = excluded.username,
team_name = excluded.team_name,
is_active = excluded.is_active`,
		u.ID,
		u.Username,
		teamName,
		u.IsActive,
	)
	if err != nil {
		s.log.Error("failed to upsert user", slog.Any("error", err))
		return fmt.Errorf("upsert user: %w", err)
	}
	return nil
}
