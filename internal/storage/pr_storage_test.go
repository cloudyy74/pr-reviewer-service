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
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/cloudyy74/pr-reviewer-service/internal/models"
	"github.com/cloudyy74/pr-reviewer-service/pkg/postgres"
)

func newPRStorage(t *testing.T) (*PRStorage, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	pg := &postgres.Postgres{
		DB: db,
	}

	st, err := NewPRStorage(pg, log)
	if err != nil {
		t.Fatalf("NewPRStorage: %v", err)
	}
	return st, mock
}

func TestPRStorage_CreatePR_Success(t *testing.T) {
	st, mock := newPRStorage(t)
	query := regexp.QuoteMeta(`
        insert into pull_requests (id, title, author_id, status_id)
        values ($1, $2, $3, (select id from statuses where name = $4))
        returning id, title, author_id, $4 as status`)
	mock.ExpectQuery(query).
		WithArgs("pr1", "title", "author", models.StatusOpen).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "author_id", "status"}).
			AddRow("pr1", "title", "author", models.StatusOpen))

	pr, err := st.CreatePR(context.Background(), models.PullRequest{
		ID:       "pr1",
		Title:    "title",
		AuthorID: "author",
		Status:   models.StatusOpen,
	})
	if err != nil {
		t.Fatalf("CreatePR returned err: %v", err)
	}
	if pr == nil || pr.ID != "pr1" {
		t.Fatalf("unexpected PR: %#v", pr)
	}
	verifyExpectations(t, mock)
}

func TestPRStorage_CreatePR_UniqueViolation(t *testing.T) {
	st, mock := newPRStorage(t)
	query := regexp.QuoteMeta(`
        insert into pull_requests (id, title, author_id, status_id)
        values ($1, $2, $3, (select id from statuses where name = $4))
        returning id, title, author_id, $4 as status`)
	mock.ExpectQuery(query).
		WithArgs("pr1", "title", "author", models.StatusOpen).
		WillReturnError(&pgconn.PgError{Code: "23505"})

	_, err := st.CreatePR(context.Background(), models.PullRequest{
		ID:       "pr1",
		Title:    "title",
		AuthorID: "author",
		Status:   models.StatusOpen,
	})
	if err == nil || !errors.Is(err, ErrPRExists) {
		t.Fatalf("expected ErrPRExists, got %v", err)
	}
	verifyExpectations(t, mock)
}

func TestPRStorage_AddReviewers(t *testing.T) {
	st, mock := newPRStorage(t)
	mock.ExpectExec(regexp.QuoteMeta("insert into pull_requests_reviewers (pull_request_id, user_id) values ($1, $2)")).
		WithArgs("pr1", "u1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("insert into pull_requests_reviewers (pull_request_id, user_id) values ($1, $2)")).
		WithArgs("pr1", "u2").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := st.AddReviewers(context.Background(), "pr1", []string{"u1", "u2"})
	if err != nil {
		t.Fatalf("AddReviewers returned err: %v", err)
	}
	verifyExpectations(t, mock)
}

func TestPRStorage_GetReviewerPRs(t *testing.T) {
	st, mock := newPRStorage(t)
	query := regexp.QuoteMeta(`
select pr.id, pr.title, pr.author_id, s.name
from pull_requests pr
    join pull_requests_reviewers r on r.pull_request_id = pr.id
    join statuses s on s.id = pr.status_id
where r.user_id = $1
order by pr.id
`)
	rows := sqlmock.NewRows([]string{"id", "title", "author_id", "status"}).
		AddRow("pr1", "title1", "author1", models.StatusOpen)
	mock.ExpectQuery(query).
		WithArgs("u1").
		WillReturnRows(rows)

	prs, err := st.GetReviewerPRs(context.Background(), "u1")
	if err != nil {
		t.Fatalf("GetReviewerPRs returned err: %v", err)
	}
	if len(prs) != 1 || prs[0].ID != "pr1" {
		t.Fatalf("unexpected prs: %#v", prs)
	}
	verifyExpectations(t, mock)
}

func TestPRStorage_GetAssignmentsStats_Success(t *testing.T) {
	st, mock := newPRStorage(t)
	userQuery := regexp.QuoteMeta(`
select user_id, count(*) as assignments
from pull_requests_reviewers
group by user_id
order by assignments desc, user_id
`)
	userRows := sqlmock.NewRows([]string{"user_id", "assignments"}).
		AddRow("u1", 3).
		AddRow("u2", 1)
	mock.ExpectQuery(userQuery).WillReturnRows(userRows)

	prQuery := regexp.QuoteMeta(`
select pull_request_id, count(*) as reviewers
from pull_requests_reviewers
group by pull_request_id
order by reviewers desc, pull_request_id
`)
	prRows := sqlmock.NewRows([]string{"pull_request_id", "reviewers"}).
		AddRow("pr1", 2).
		AddRow("pr2", 1)
	mock.ExpectQuery(prQuery).WillReturnRows(prRows)

	stats, err := st.GetAssignmentsStats(context.Background())
	if err != nil {
		t.Fatalf("GetAssignmentsStats returned err: %v", err)
	}
	if len(stats.ByUser) != 2 || stats.ByUser[0].UserID != "u1" {
		t.Fatalf("unexpected user stats: %#v", stats.ByUser)
	}
	if len(stats.ByPR) != 2 || stats.ByPR[0].PullRequestID != "pr1" {
		t.Fatalf("unexpected pr stats: %#v", stats.ByPR)
	}
	verifyExpectations(t, mock)
}

func TestPRStorage_GetAssignmentsStats_UserQueryError(t *testing.T) {
	st, mock := newPRStorage(t)
	userQuery := regexp.QuoteMeta(`
select user_id, count(*) as assignments
from pull_requests_reviewers
group by user_id
order by assignments desc, user_id
`)
	mock.ExpectQuery(userQuery).WillReturnError(errors.New("db error"))

	_, err := st.GetAssignmentsStats(context.Background())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	verifyExpectations(t, mock)
}

func TestPRStorage_GetAssignmentsStats_PRQueryError(t *testing.T) {
	st, mock := newPRStorage(t)
	userQuery := regexp.QuoteMeta(`
select user_id, count(*) as assignments
from pull_requests_reviewers
group by user_id
order by assignments desc, user_id
`)
	mock.ExpectQuery(userQuery).WillReturnRows(sqlmock.NewRows([]string{"user_id", "assignments"}))

	prQuery := regexp.QuoteMeta(`
select pull_request_id, count(*) as reviewers
from pull_requests_reviewers
group by pull_request_id
order by reviewers desc, pull_request_id
`)
	mock.ExpectQuery(prQuery).WillReturnError(errors.New("db error"))

	_, err := st.GetAssignmentsStats(context.Background())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	verifyExpectations(t, mock)
}

func TestPRStorage_GetPR_Success(t *testing.T) {
	st, mock := newPRStorage(t)
	prQuery := regexp.QuoteMeta(`
select pr.id, pr.title, pr.author_id, s.name
from pull_requests pr
    join statuses s on s.id = pr.status_id
where pr.id = $1
`)
	mock.ExpectQuery(prQuery).
		WithArgs("pr1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "author_id", "status"}).
			AddRow("pr1", "title", "author", models.StatusOpen))

	reviewerRows := sqlmock.NewRows([]string{"user_id"}).AddRow("u1").AddRow("u2")
	mock.ExpectQuery(regexp.QuoteMeta(`select user_id from pull_requests_reviewers where pull_request_id = $1 order by user_id`)).
		WithArgs("pr1").
		WillReturnRows(reviewerRows)

	pr, err := st.GetPR(context.Background(), "pr1")
	if err != nil {
		t.Fatalf("GetPR returned err: %v", err)
	}
	if pr.Status != models.StatusOpen || len(pr.Reviewers) != 2 {
		t.Fatalf("unexpected pr: %#v", pr)
	}
	verifyExpectations(t, mock)
}

func TestPRStorage_GetPR_NotFound(t *testing.T) {
	st, mock := newPRStorage(t)
	query := regexp.QuoteMeta(`
select pr.id, pr.title, pr.author_id, s.name
from pull_requests pr
    join statuses s on s.id = pr.status_id
where pr.id = $1
`)
	mock.ExpectQuery(query).
		WithArgs("pr1").
		WillReturnError(sql.ErrNoRows)

	_, err := st.GetPR(context.Background(), "pr1")
	if err == nil || !errors.Is(err, ErrPRNotFound) {
		t.Fatalf("expected ErrPRNotFound, got %v", err)
	}
	verifyExpectations(t, mock)
}

func TestPRStorage_UpdatePRStatus(t *testing.T) {
	st, mock := newPRStorage(t)
	mock.ExpectExec(regexp.QuoteMeta(`
update pull_requests
set status_id = (select id from statuses where name = $2)
where id = $1
`)).
		WithArgs("pr1", models.StatusMerged).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := st.UpdatePRStatus(context.Background(), "pr1", models.StatusMerged)
	if err != nil {
		t.Fatalf("UpdatePRStatus returned err: %v", err)
	}
	verifyExpectations(t, mock)
}

func TestPRStorage_UpdatePRStatus_NotFound(t *testing.T) {
	st, mock := newPRStorage(t)
	mock.ExpectExec(regexp.QuoteMeta(`
update pull_requests
set status_id = (select id from statuses where name = $2)
where id = $1
`)).
		WithArgs("pr1", models.StatusMerged).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := st.UpdatePRStatus(context.Background(), "pr1", models.StatusMerged)
	if err == nil || !errors.Is(err, ErrPRNotFound) {
		t.Fatalf("expected ErrPRNotFound, got %v", err)
	}
	verifyExpectations(t, mock)
}

func TestPRStorage_ReplaceReviewer(t *testing.T) {
	st, mock := newPRStorage(t)
	mock.ExpectExec(regexp.QuoteMeta(`delete from pull_requests_reviewers where pull_request_id = $1 and user_id = $2`)).
		WithArgs("pr1", "u1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`insert into pull_requests_reviewers (pull_request_id, user_id) values ($1, $2)`)).
		WithArgs("pr1", "u2").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := st.ReplaceReviewer(context.Background(), "pr1", "u1", "u2")
	if err != nil {
		t.Fatalf("ReplaceReviewer returned err: %v", err)
	}
	verifyExpectations(t, mock)
}

func TestPRStorage_ReplaceReviewer_NotAssigned(t *testing.T) {
	st, mock := newPRStorage(t)
	mock.ExpectExec(regexp.QuoteMeta(`delete from pull_requests_reviewers where pull_request_id = $1 and user_id = $2`)).
		WithArgs("pr1", "u1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := st.ReplaceReviewer(context.Background(), "pr1", "u1", "u2")
	if err == nil || !errors.Is(err, ErrReviewerNotAssigned) {
		t.Fatalf("expected ErrReviewerNotAssigned, got %v", err)
	}
	verifyExpectations(t, mock)
}
