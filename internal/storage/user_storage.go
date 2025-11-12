package storage

import (
	"context"
	"errors"
	"log/slog"

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

func (s *UserStorage) CreateUser(ctx context.Context, teamName string) error {
	// TODO
	return nil
}
