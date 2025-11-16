package storage

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/cloudyy74/pr-reviewer-service/internal/models"
	"github.com/cloudyy74/pr-reviewer-service/pkg/postgres"
)

func newUserStorage(t *testing.T) (*UserStorage, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	pg := &postgres.Postgres{
		DB: db,
	}

	st, err := NewUserStorage(pg, log)
	if err != nil {
		t.Fatalf("NewUserStorage: %v", err)
	}
	return st, mock
}

func TestUserStorage_UpsertUser(t *testing.T) {
	st, mock := newUserStorage(t)
	mock.ExpectExec(regexp.QuoteMeta("insert into users")).
		WithArgs("u1", "user", "team", true).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := st.UpsertUser(context.Background(), models.User{
		ID:       "u1",
		Username: "user",
		IsActive: true,
	}, "team")
	if err != nil {
		t.Fatalf("UpsertUser returned err: %v", err)
	}
	verifyExpectations(t, mock)
}

func TestUserStorage_GetUsersByTeam(t *testing.T) {
	st, mock := newUserStorage(t)
	rows := sqlmock.NewRows([]string{"id", "username", "is_active"}).
		AddRow("u1", "user1", true).
		AddRow("u2", "user2", false)
	mock.ExpectQuery(regexp.QuoteMeta("select id, username, is_active from users")).
		WithArgs("team").
		WillReturnRows(rows)

	users, err := st.GetUsersByTeam(context.Background(), "team")
	if err != nil {
		t.Fatalf("GetUsersByTeam returned err: %v", err)
	}
	if len(users) != 2 || users[0].ID != "u1" || users[1].ID != "u2" {
		t.Fatalf("unexpected users result: %#v", users)
	}
	verifyExpectations(t, mock)
}

func TestUserStorage_GetUsersByTeam_Empty(t *testing.T) {
	st, mock := newUserStorage(t)
	rows := sqlmock.NewRows([]string{"id", "username", "is_active"})
	mock.ExpectQuery(regexp.QuoteMeta("select id, username, is_active from users")).
		WithArgs("team").
		WillReturnRows(rows)

	users, err := st.GetUsersByTeam(context.Background(), "team")
	if err != nil {
		t.Fatalf("GetUsersByTeam returned err: %v", err)
	}
	if users == nil || len(users) != 0 {
		t.Fatalf("expected empty slice, got %#v", users)
	}
	verifyExpectations(t, mock)
}

func TestUserStorage_DeactivateTeamUsers(t *testing.T) {
	st, mock := newUserStorage(t)
	mock.ExpectExec(regexp.QuoteMeta(`update users set is_active = false where team_name = $1 and is_active`)).
		WithArgs("team").
		WillReturnResult(sqlmock.NewResult(0, 3))

	count, err := st.DeactivateTeamUsers(context.Background(), "team")
	if err != nil {
		t.Fatalf("DeactivateTeamUsers returned err: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 affected rows, got %d", count)
	}
	verifyExpectations(t, mock)
}

func TestUserStorage_SetUserActive(t *testing.T) {
	st, mock := newUserStorage(t)
	query := regexp.QuoteMeta(`
update users set is_active = $1 where id = $2
 returning id, username, team_name, is_active`)
	mock.ExpectQuery(query).
		WithArgs(true, "u1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "team_name", "is_active"}).
			AddRow("u1", "user", "team", true))

	user, err := st.SetUserActive(context.Background(), "u1", true)
	if err != nil {
		t.Fatalf("SetUserActive returned err: %v", err)
	}
	if user.ID != "u1" || !user.IsActive {
		t.Fatalf("unexpected user returned: %#v", user)
	}
	verifyExpectations(t, mock)
}

func TestUserStorage_SetUserActive_NotFound(t *testing.T) {
	st, mock := newUserStorage(t)
	query := regexp.QuoteMeta(`
update users set is_active = $1 where id = $2
 returning id, username, team_name, is_active`)
	mock.ExpectQuery(query).
		WithArgs(true, "u1").
		WillReturnError(sql.ErrNoRows)

	_, err := st.SetUserActive(context.Background(), "u1", true)
	if err == nil || !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
	verifyExpectations(t, mock)
}

func TestUserStorage_GetUserWithTeam(t *testing.T) {
	st, mock := newUserStorage(t)
	mock.ExpectQuery(regexp.QuoteMeta("select id, username, team_name, is_active from users where id = $1")).
		WithArgs("u1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "team_name", "is_active"}).
			AddRow("u1", "user", "team", true))

	user, err := st.GetUserWithTeam(context.Background(), "u1")
	if err != nil {
		t.Fatalf("GetUserWithTeam returned err: %v", err)
	}
	if user.TeamName != "team" {
		t.Fatalf("unexpected user: %#v", user)
	}
	verifyExpectations(t, mock)
}

func TestUserStorage_GetActiveTeammates(t *testing.T) {
	st, mock := newUserStorage(t)
	rows := sqlmock.NewRows([]string{"id", "username", "is_active"}).
		AddRow("u2", "user2", true)
	mock.ExpectQuery(regexp.QuoteMeta(`
select id, username, is_active
from users
where team_name = $1
  and is_active
  and id <> $2
order by random()
limit $3
`)).
		WithArgs("team", "u1", 1).
		WillReturnRows(rows)

	users, err := st.GetActiveTeammates(context.Background(), "team", "u1", 1)
	if err != nil {
		t.Fatalf("GetActiveTeammates returned err: %v", err)
	}
	if len(users) != 1 || users[0].ID != "u2" {
		t.Fatalf("unexpected users: %#v", users)
	}
	verifyExpectations(t, mock)
}

func TestUserStorage_GetRandomActiveTeammate_NoCandidate(t *testing.T) {
	st, mock := newUserStorage(t)
	mock.ExpectQuery(regexp.QuoteMeta(`
select id, username, is_active
from users
where team_name = $1
  and is_active
  and id not in ($2)
order by random()
limit 1`)).
		WithArgs("team", "u1").
		WillReturnError(sql.ErrNoRows)

	_, err := st.GetRandomActiveTeammate(context.Background(), "team", []string{"u1"})
	if err == nil || !errors.Is(err, ErrNoCandidate) {
		t.Fatalf("expected ErrNoCandidate, got %v", err)
	}
	verifyExpectations(t, mock)
}

func verifyExpectations(t *testing.T, mock sqlmock.Sqlmock) {
	t.Helper()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
