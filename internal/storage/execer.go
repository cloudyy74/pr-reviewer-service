package storage

import (
	"context"
	"database/sql"
)

type execer interface {
    ExecContext(context.Context, string, ...any) (sql.Result, error)
}
type queryExecer interface {
    execer
    QueryContext(context.Context, string, ...any) (*sql.Rows, error)
    QueryRowContext(context.Context, string, ...any) *sql.Row
}

func getExecer(ctx context.Context, db *sql.DB) execer {
    if tx, ok := TxFromCtx(ctx); ok {
        return tx
    }
    return db
}

func getQueryExecer(ctx context.Context, db *sql.DB) queryExecer {
    if tx, ok := TxFromCtx(ctx); ok {
        return tx
    }
    return db
}