package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/cloudyy74/pr-reviewer-service/internal/models"
	"github.com/cloudyy74/pr-reviewer-service/pkg/postgres"
)

var (
	ErrPRExists = errors.New("pr already exists")
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
