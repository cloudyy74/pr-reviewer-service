package storage

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/cloudyy74/pr-reviewer-service/pkg/postgres"
)

func newTeamStorage(t *testing.T) (*TeamStorage, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	pg := &postgres.Postgres{DB: db}
	storage, err := NewTeamStorage(pg, log)
	if err != nil {
		t.Fatalf("NewTeamStorage: %v", err)
	}
	return storage, mock
}

func TestTeamStorage_CreateTeam_Success(t *testing.T) {
	st, mock := newTeamStorage(t)
	mock.ExpectExec(regexp.QuoteMeta("insert into teams (name) values ($1) on conflict (name) do nothing")).
		WithArgs("backend").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := st.CreateTeam(context.Background(), "backend"); err != nil {
		t.Fatalf("CreateTeam returned err: %v", err)
	}
	verifyExpectations(t, mock)
}

func TestTeamStorage_CreateTeam_AlreadyExists(t *testing.T) {
	st, mock := newTeamStorage(t)
	mock.ExpectExec(regexp.QuoteMeta("insert into teams (name) values ($1) on conflict (name) do nothing")).
		WithArgs("backend").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := st.CreateTeam(context.Background(), "backend")
	if err == nil || !errors.Is(err, ErrTeamExists) {
		t.Fatalf("expected ErrTeamExists, got %v", err)
	}
	verifyExpectations(t, mock)
}

func TestTeamStorage_CreateTeam_DBError(t *testing.T) {
	st, mock := newTeamStorage(t)
	mock.ExpectExec(regexp.QuoteMeta("insert into teams (name) values ($1) on conflict (name) do nothing")).
		WithArgs("backend").
		WillReturnError(errors.New("db error"))

	err := st.CreateTeam(context.Background(), "backend")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	verifyExpectations(t, mock)
}

func TestTeamStorage_ExistsTeam(t *testing.T) {
	st, mock := newTeamStorage(t)
	mock.ExpectQuery(regexp.QuoteMeta(`select exists(
            select 1 from teams where name = $1
        )`)).
		WithArgs("backend").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	exists, err := st.ExistsTeam(context.Background(), "backend")
	if err != nil {
		t.Fatalf("ExistsTeam returned err: %v", err)
	}
	if !exists {
		t.Fatalf("expected team to exist")
	}
	verifyExpectations(t, mock)
}
