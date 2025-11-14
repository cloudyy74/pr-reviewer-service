package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/cloudyy74/pr-reviewer-service/pkg/postgres"
)

type TxManagerSQL struct {
	db  *postgres.Postgres
	log *slog.Logger
}

func TxFromCtx(ctx context.Context) (*sql.Tx, bool) {
	tx, ok := ctx.Value(txCtxKey{}).(*sql.Tx)
	return tx, ok
}

func NewTxManager(db *postgres.Postgres, log *slog.Logger) (*TxManagerSQL, error) {
	if db == nil {
		return nil, errors.New("database cannot be nil")
	}
	if log == nil {
		return nil, errors.New("logger cannot be nil")
	}
	return &TxManagerSQL{
		db:  db,
		log: log,
	}, nil
}

type txCtxKey struct{}

func (m *TxManagerSQL) Run(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := m.db.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	ctx = context.WithValue(ctx, txCtxKey{}, tx)

	defer func() {
		if p := recover(); p != nil {
			m.rollback(tx)
			panic(p)
		}
	}()

	if err := fn(ctx); err != nil {
		m.rollback(tx)
		return fmt.Errorf("run in transaction: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

func (m *TxManagerSQL) rollback(tx *sql.Tx) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		m.log.Error("failed to rollback transaction", slog.Any("error", err))
	}
}
