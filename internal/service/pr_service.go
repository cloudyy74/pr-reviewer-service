package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/cloudyy74/pr-reviewer-service/internal/models"
	"github.com/cloudyy74/pr-reviewer-service/internal/storage"
)

const reviewersPerPR = 2

var (
	ErrPRValidation        = errors.New("validation error")
	ErrPRAuthorNotFound    = errors.New("author not found")
	ErrPRTeamNotFound      = errors.New("team not found")
	ErrPRAlreadyExists     = errors.New("pull request already exists")
	ErrPRNotFound          = errors.New("pull request not found")
	ErrPRMerged            = errors.New("pull request already merged")
	ErrReviewerNotAssigned = errors.New("reviewer not assigned")
	ErrNoReplacement       = errors.New("no replacement candidate")
)

type PRRepository interface {
	CreatePR(ctx context.Context, pr models.PullRequest) (*models.PullRequest, error)
	AddReviewers(ctx context.Context, prID string, reviewerIDs []string) error
	GetReviewerPRs(ctx context.Context, userID string) ([]*models.PullRequestShort, error)
	GetPR(ctx context.Context, prID string) (*models.PullRequest, error)
	UpdatePRStatus(ctx context.Context, prID, status string) error
	ReplaceReviewer(ctx context.Context, prID, oldReviewerID, newReviewerID string) error
}

type PRUserRepository interface {
	GetUserWithTeam(ctx context.Context, userID string) (*models.UserWithTeam, error)
	GetActiveTeammates(ctx context.Context, teamName, excludeUserID string, limit int) ([]*models.User, error)
	GetRandomActiveTeammate(ctx context.Context, teamName string, excludeIDs []string) (*models.User, error)
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

func (s *PRService) MergePR(ctx context.Context, req *models.PRMergeRequest) (*models.PullRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: empty body", ErrPRValidation)
	}
	prID := strings.TrimSpace(req.ID)
	if prID == "" {
		return nil, fmt.Errorf("%w: pull_request_id is required", ErrPRValidation)
	}

	var mergedPR *models.PullRequest
	err := s.tx.Run(ctx, func(ctx context.Context) error {
		pr, err := s.prs.GetPR(ctx, prID)
		if err != nil {
			switch {
			case errors.Is(err, storage.ErrPRNotFound):
				return ErrPRNotFound
			default:
				return fmt.Errorf("get pr: %w", err)
			}
		}
		if pr.Status == models.StatusMerged {
			mergedPR = pr
			return nil
		}
		if err := s.prs.UpdatePRStatus(ctx, prID, models.StatusMerged); err != nil {
			return fmt.Errorf("update pr status: %w", err)
		}
		pr.Status = models.StatusMerged
		mergedPR = pr
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrPRValidation), errors.Is(err, ErrPRNotFound):
			return nil, err
		default:
			return nil, fmt.Errorf("merge pr transaction: %w", err)
		}
	}
	return mergedPR, nil
}

func (s *PRService) ReassignReviewer(ctx context.Context, req *models.PRReassignRequest) (*models.PRReassignResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: empty body", ErrPRValidation)
	}
	prID := strings.TrimSpace(req.ID)
	oldReviewerID := strings.TrimSpace(req.OldReviewerID)
	if prID == "" {
		return nil, fmt.Errorf("%w: pull_request_id is required", ErrPRValidation)
	}
	if oldReviewerID == "" {
		return nil, fmt.Errorf("%w: old_user_id is required", ErrPRValidation)
	}

	var reassignResp *models.PRReassignResponse
	err := s.tx.Run(ctx, func(ctx context.Context) error {
		pr, err := s.prs.GetPR(ctx, prID)
		if err != nil {
			switch {
			case errors.Is(err, storage.ErrPRNotFound):
				return ErrPRNotFound
			default:
				return fmt.Errorf("get pr: %w", err)
			}
		}
		if pr.Status == models.StatusMerged {
			return ErrPRMerged
		}

		assigned := slices.Contains(pr.Reviewers, oldReviewerID)
		if !assigned {
			return ErrReviewerNotAssigned
		}

		reviewerUser, err := s.users.GetUserWithTeam(ctx, oldReviewerID)
		if err != nil {
			switch {
			case errors.Is(err, storage.ErrUserNotFound):
				return ErrUserNotFound
			default:
				return fmt.Errorf("get reviewer: %w", err)
			}
		}
		teamName := strings.TrimSpace(reviewerUser.TeamName)
		if teamName == "" {
			return ErrPRTeamNotFound
		}

		excludeIDs := make(map[string]struct{}, len(pr.Reviewers)+1)
		excludeIDs[oldReviewerID] = struct{}{}
		for _, reviewer := range pr.Reviewers {
			excludeIDs[reviewer] = struct{}{}
		}
		excludeList := make([]string, 0, len(excludeIDs))
		for id := range excludeIDs {
			excludeList = append(excludeList, id)
		}

		replacement, err := s.users.GetRandomActiveTeammate(ctx, teamName, excludeList)
		if err != nil {
			switch {
			case errors.Is(err, storage.ErrNoCandidate):
				return ErrNoReplacement
			default:
				return fmt.Errorf("get replacement: %w", err)
			}
		}

		if err := s.prs.ReplaceReviewer(ctx, prID, oldReviewerID, replacement.ID); err != nil {
			switch {
			case errors.Is(err, storage.ErrReviewerNotAssigned):
				return ErrReviewerNotAssigned
			default:
				return fmt.Errorf("replace reviewer: %w", err)
			}
		}

		for i, reviewer := range pr.Reviewers {
			if reviewer == oldReviewerID {
				pr.Reviewers[i] = replacement.ID
				break
			}
		}

		reassignResp = &models.PRReassignResponse{
			PR:         *pr,
			ReplacedBy: replacement.ID,
		}
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrPRValidation),
			errors.Is(err, ErrPRNotFound),
			errors.Is(err, ErrUserNotFound),
			errors.Is(err, ErrReviewerNotAssigned),
			errors.Is(err, ErrNoReplacement),
			errors.Is(err, ErrPRMerged),
			errors.Is(err, ErrPRTeamNotFound):
			return nil, err
		default:
			return nil, fmt.Errorf("reassign reviewer transaction: %w", err)
		}
	}

	return reassignResp, nil
}
