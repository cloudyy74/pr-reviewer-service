package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/cloudyy74/pr-reviewer-service/internal/models"
	"github.com/cloudyy74/pr-reviewer-service/pkg/postgres"
)

var (
	ErrPRExists            = errors.New("pr already exists")
	ErrPRNotFound          = errors.New("pr not found")
	ErrReviewerNotAssigned = errors.New("reviewer not assigned")
)

type PRStorage struct {
	db  *postgres.Postgres
	log *slog.Logger
}

func NewPRStorage(db *postgres.Postgres, log *slog.Logger) (*PRStorage, error) {
	if db == nil {
		return nil, errors.New("database cannot be nil")
	}
	if log == nil {
		return nil, errors.New("logger cannot be nil")
	}
	return &PRStorage{
		db:  db,
		log: log,
	}, nil
}

func (s *PRStorage) CreatePR(ctx context.Context, pr models.PullRequest) (*models.PullRequest, error) {
	exec := getQueryExecer(ctx, s.db.DB)
	var created models.PullRequest
	err := exec.QueryRowContext(ctx, `
        insert into pull_requests (id, title, author_id, status_id, need_more_reviewers)
        values ($1, $2, $3, (select id from statuses where name = $4), $5)
        returning id, title, author_id, $4 as status, need_more_reviewers`,
		pr.ID, pr.Title, pr.AuthorID, pr.Status, pr.NeedMoreReviewers,
	).Scan(&created.ID, &created.Title, &created.AuthorID, &created.Status, &created.NeedMoreReviewers)
	if err != nil {
		if postgres.IsUniqueViolation(err) {
			return nil, ErrPRExists
		}
		return nil, fmt.Errorf("insert pr: %w", err)
	}
	return &created, nil
}

func (s *PRStorage) AddReviewers(ctx context.Context, prID string, reviewerIDs []string) error {
	if len(reviewerIDs) == 0 {
		return nil
	}
	exec := getExecer(ctx, s.db.DB)
	for _, reviewerID := range reviewerIDs {
		if _, err := exec.ExecContext(
			ctx,
			"insert into pull_requests_reviewers (pull_request_id, user_id) values ($1, $2)",
			prID,
			reviewerID,
		); err != nil {
			s.log.Error("failed to add reviewer", slog.Any("error", err), slog.String("pr_id", prID), slog.String("user_id", reviewerID))
			return fmt.Errorf("add reviewer %s: %w", reviewerID, err)
		}
	}
	return nil
}

func (s *PRStorage) GetReviewerPRs(ctx context.Context, userID string) ([]*models.PullRequestShort, error) {
	exec := getQueryExecer(ctx, s.db.DB)
	rows, err := exec.QueryContext(
		ctx,
		`
select pr.id, pr.title, pr.author_id, s.name
from pull_requests pr
    join pull_requests_reviewers r on r.pull_request_id = pr.id
    join statuses s on s.id = pr.status_id
where r.user_id = $1
order by pr.id
`,
		userID,
	)
	if err != nil {
		s.log.Error("failed to get reviewer prs", slog.Any("error", err), slog.String("user_id", userID))
		return nil, fmt.Errorf("get reviewer prs: %w", err)
	}
	defer rows.Close()

	prs := make([]*models.PullRequestShort, 0)
	for rows.Next() {
		var pr models.PullRequestShort
		if err := rows.Scan(&pr.ID, &pr.Title, &pr.AuthorID, &pr.Status); err != nil {
			return nil, fmt.Errorf("scan reviewer pr: %w", err)
		}
		prs = append(prs, &pr)
	}

	return prs, nil
}

func (s *PRStorage) GetPR(ctx context.Context, prID string) (*models.PullRequest, error) {
	exec := getQueryExecer(ctx, s.db.DB)
	var pr models.PullRequest
	err := exec.QueryRowContext(
		ctx,
		`
select pr.id, pr.title, pr.author_id, s.name, pr.need_more_reviewers
from pull_requests pr
    join statuses s on s.id = pr.status_id
where pr.id = $1
`,
		prID,
	).Scan(&pr.ID, &pr.Title, &pr.AuthorID, &pr.Status, &pr.NeedMoreReviewers)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get pr: %w", ErrPRNotFound)
	}
	if err != nil {
		s.log.Error("failed to get pr", slog.Any("error", err), slog.String("pr_id", prID))
		return nil, fmt.Errorf("get pr: %w", err)
	}

	rows, err := exec.QueryContext(
		ctx,
		`select user_id from pull_requests_reviewers where pull_request_id = $1 order by user_id`,
		prID,
	)
	if err != nil {
		return nil, fmt.Errorf("get pr reviewers: %w", err)
	}
	defer rows.Close()
	reviewers := make([]string, 0)
	for rows.Next() {
		var reviewer string
		if err := rows.Scan(&reviewer); err != nil {
			return nil, fmt.Errorf("scan reviewer: %w", err)
		}
		reviewers = append(reviewers, reviewer)
	}
	pr.Reviewers = reviewers
	return &pr, nil
}

func (s *PRStorage) UpdatePRStatus(ctx context.Context, prID, status string) error {
	exec := getExecer(ctx, s.db.DB)
	res, err := exec.ExecContext(
		ctx,
		`
update pull_requests
set status_id = (select id from statuses where name = $2)
where id = $1
`,
		prID,
		status,
	)
	if err != nil {
		return fmt.Errorf("update pr status: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrPRNotFound
	}
	return nil
}

func (s *PRStorage) ReplaceReviewer(ctx context.Context, prID, oldReviewerID, newReviewerID string) error {
	exec := getExecer(ctx, s.db.DB)
	res, err := exec.ExecContext(
		ctx,
		`delete from pull_requests_reviewers where pull_request_id = $1 and user_id = $2`,
		prID,
		oldReviewerID,
	)
	if err != nil {
		return fmt.Errorf("delete reviewer: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete reviewer rows: %w", err)
	}
	if rows == 0 {
		return ErrReviewerNotAssigned
	}
	if _, err := exec.ExecContext(
		ctx,
		`insert into pull_requests_reviewers (pull_request_id, user_id) values ($1, $2)`,
		prID,
		newReviewerID,
	); err != nil {
		return fmt.Errorf("insert reviewer: %w", err)
	}
	return nil
}
