package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cloudyy74/pr-reviewer-service/internal/models"
	"github.com/cloudyy74/pr-reviewer-service/internal/storage"
)

const reviewersPerPR = 2

var (
	ErrPRValidation     = errors.New("validation error")
	ErrPRAuthorNotFound = errors.New("author not found")
	ErrPRTeamNotFound   = errors.New("team not found")
	ErrPRAlreadyExists  = errors.New("pull request already exists")
)

type PRRepository interface {
	CreatePR(ctx context.Context, pr models.PullRequest) (*models.PullRequest, error)
	AddReviewers(ctx context.Context, prID string, reviewerIDs []string) error
	GetReviewerPRs(ctx context.Context, userID string) ([]*models.PullRequestShort, error)
}

type PRUserRepository interface {
	GetUserWithTeam(ctx context.Context, userID string) (*models.UserWithTeam, error)
	GetActiveTeammates(ctx context.Context, teamName, excludeUserID string, limit int) ([]*models.User, error)
}

type PRService struct {
	tx    txManager
	prs   PRRepository
	users PRUserRepository
	log   *slog.Logger
}

func NewPRService(tx txManager, prs PRRepository, users PRUserRepository, log *slog.Logger) (*PRService, error) {
	if tx == nil {
		return nil, errors.New("tx manager cannot be nil")
	}
	if prs == nil {
		return nil, errors.New("pr repository cannot be nil")
	}
	if users == nil {
		return nil, errors.New("user repository cannot be nil")
	}
	if log == nil {
		return nil, errors.New("logger cannot be nil")
	}
	return &PRService{tx: tx, prs: prs, users: users, log: log}, nil
}

func (s *PRService) CreatePR(ctx context.Context, req *models.PRCreateRequest) (*models.PullRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: empty body", ErrPRValidation)
	}
	prID := strings.TrimSpace(req.ID)
	title := strings.TrimSpace(req.Title)
	authorID := strings.TrimSpace(req.AuthorID)
	if prID == "" {
		return nil, fmt.Errorf("%w: pull_request_id is required", ErrPRValidation)
	}
	if title == "" {
		return nil, fmt.Errorf("%w: pull_request_name is required", ErrPRValidation)
	}
	if authorID == "" {
		return nil, fmt.Errorf("%w: author_id is required", ErrPRValidation)
	}

	var createdPR *models.PullRequest
	err := s.tx.Run(ctx, func(ctx context.Context) error {
		author, err := s.users.GetUserWithTeam(ctx, authorID)
		if err != nil {
			switch {
			case errors.Is(err, storage.ErrUserNotFound):
				return ErrPRAuthorNotFound
			default:
				return fmt.Errorf("get author: %w", err)
			}
		}
		teamName := strings.TrimSpace(author.TeamName)
		if teamName == "" {
			return ErrPRTeamNotFound
		}

		teammates, err := s.users.GetActiveTeammates(ctx, teamName, author.ID, reviewersPerPR)
		if err != nil {
			return fmt.Errorf("get teammates: %w", err)
		}
		reviewers := make([]string, 0, len(teammates))
		for _, tm := range teammates {
			reviewers = append(reviewers, tm.ID)
		}
		needMore := len(reviewers) < reviewersPerPR

		pr := models.PullRequest{
			ID:                prID,
			Title:             title,
			AuthorID:          author.ID,
			Status:            models.StatusOpen,
			NeedMoreReviewers: needMore,
		}
		created, err := s.prs.CreatePR(ctx, pr)
		if err != nil {
			switch {
			case errors.Is(err, storage.ErrPRExists):
				return ErrPRAlreadyExists
			default:
				return fmt.Errorf("create pr: %w", err)
			}
		}
		if err := s.prs.AddReviewers(ctx, created.ID, reviewers); err != nil {
			return fmt.Errorf("add reviewers: %w", err)
		}
		created.Reviewers = reviewers
		created.NeedMoreReviewers = needMore
		createdPR = created
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrPRValidation),
			errors.Is(err, ErrPRAuthorNotFound),
			errors.Is(err, ErrPRTeamNotFound),
			errors.Is(err, ErrPRAlreadyExists):
			return nil, err
		default:
			return nil, fmt.Errorf("create pr transaction: %w", err)
		}
	}
	return createdPR, nil
}

func (s *PRService) GetUserReviews(ctx context.Context, userID string) (*models.UserReviewsResponse, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("%w: user_id is required", ErrPRValidation)
	}

	if _, err := s.users.GetUserWithTeam(ctx, userID); err != nil {
		switch {
		case errors.Is(err, storage.ErrUserNotFound):
			return nil, ErrUserNotFound
		default:
			return nil, fmt.Errorf("get user: %w", err)
		}
	}

	prs, err := s.prs.GetReviewerPRs(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user reviews: %w", err)
	}
	if prs == nil {
		prs = make([]*models.PullRequestShort, 0)
	}

	return &models.UserReviewsResponse{
		UserID:       userID,
		PullRequests: prs,
	}, nil
}
