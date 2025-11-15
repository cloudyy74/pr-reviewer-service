package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/cloudyy74/pr-reviewer-service/internal/models"
	"github.com/cloudyy74/pr-reviewer-service/internal/storage"
)

type fakeTxManager struct{}

func (fakeTxManager) Run(_ context.Context, fn func(ctx context.Context) error) error {
	return fn(context.Background())
}

type fakePRRepo struct {
	createPRFn        func(context.Context, models.PullRequest) (*models.PullRequest, error)
	addReviewersFn    func(context.Context, string, []string) error
	getReviewerPRsFn  func(context.Context, string) ([]*models.PullRequestShort, error)
	getPRFn           func(context.Context, string) (*models.PullRequest, error)
	updateStatusFn    func(context.Context, string, string) error
	replaceReviewerFn func(context.Context, string, string, string) error
}

func (f *fakePRRepo) CreatePR(ctx context.Context, pr models.PullRequest) (*models.PullRequest, error) {
	return f.createPRFn(ctx, pr)
}

func (f *fakePRRepo) AddReviewers(ctx context.Context, prID string, reviewerIDs []string) error {
	return f.addReviewersFn(ctx, prID, reviewerIDs)
}

func (f *fakePRRepo) GetReviewerPRs(ctx context.Context, userID string) ([]*models.PullRequestShort, error) {
	return f.getReviewerPRsFn(ctx, userID)
}

func (f *fakePRRepo) GetPR(ctx context.Context, prID string) (*models.PullRequest, error) {
	return f.getPRFn(ctx, prID)
}

func (f *fakePRRepo) UpdatePRStatus(ctx context.Context, prID, status string) error {
	return f.updateStatusFn(ctx, prID, status)
}

func (f *fakePRRepo) ReplaceReviewer(ctx context.Context, prID, oldReviewerID, newReviewerID string) error {
	return f.replaceReviewerFn(ctx, prID, oldReviewerID, newReviewerID)
}

type fakePRUserRepo struct {
	getUserFn       func(context.Context, string) (*models.UserWithTeam, error)
	getTeammatesFn  func(context.Context, string, string, int) ([]*models.User, error)
	getRandomMateFn func(context.Context, string, []string) (*models.User, error)
}

func (f *fakePRUserRepo) GetUserWithTeam(ctx context.Context, userID string) (*models.UserWithTeam, error) {
	return f.getUserFn(ctx, userID)
}

func (f *fakePRUserRepo) GetActiveTeammates(ctx context.Context, teamName, excludeUserID string, limit int) ([]*models.User, error) {
	return f.getTeammatesFn(ctx, teamName, excludeUserID, limit)
}

func (f *fakePRUserRepo) GetRandomActiveTeammate(ctx context.Context, teamName string, excludeIDs []string) (*models.User, error) {
	return f.getRandomMateFn(ctx, teamName, excludeIDs)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewPRService_ValidatesDependencies(t *testing.T) {
	_, err := NewPRService(nil, nil, nil, nil)
	if err == nil {
		t.Fatalf("expected error when dependencies are nil")
	}
}

func TestPRService_CreatePR_Success(t *testing.T) {
	created := models.PullRequest{ID: "pr-1"}
	receivedReviewers := []string{}
	repo := &fakePRRepo{
		createPRFn: func(_ context.Context, pr models.PullRequest) (*models.PullRequest, error) {
			return &created, nil
		},
		addReviewersFn: func(_ context.Context, prID string, reviewerIDs []string) error {
			receivedReviewers = append([]string{}, reviewerIDs...)
			return nil
		},
		getReviewerPRsFn:  nil,
		getPRFn:           nil,
		updateStatusFn:    nil,
		replaceReviewerFn: nil,
	}
	userRepo := &fakePRUserRepo{
		getUserFn: func(_ context.Context, userID string) (*models.UserWithTeam, error) {
			return &models.UserWithTeam{User: models.User{ID: userID}, TeamName: "backend"}, nil
		},
		getTeammatesFn: func(_ context.Context, teamName, exclude string, limit int) ([]*models.User, error) {
			return []*models.User{{ID: "u2"}, {ID: "u3"}}, nil
		},
		getRandomMateFn: nil,
	}
	service, err := NewPRService(fakeTxManager{}, repo, userRepo, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pr, err := service.CreatePR(context.Background(), &models.PRCreateRequest{
		ID:       " pr-1 ",
		Title:    " Add feature ",
		AuthorID: " u1 ",
	})
	if err != nil {
		t.Fatalf("CreatePR returned error: %v", err)
	}
	if pr == nil || pr.ID != "pr-1" {
		t.Fatalf("expected created PR, got %#v", pr)
	}
	if len(receivedReviewers) != 2 {
		t.Fatalf("expected 2 reviewers, got %v", receivedReviewers)
	}
	if pr.NeedMoreReviewers {
		t.Fatalf("did not expect NeedMoreReviewers to be true")
	}
}

func TestPRService_CreatePR_AuthorNotFound(t *testing.T) {
	repo := &fakePRRepo{}
	userRepo := &fakePRUserRepo{
		getUserFn: func(_ context.Context, _ string) (*models.UserWithTeam, error) {
			return nil, storage.ErrUserNotFound
		},
	}
	service, err := NewPRService(fakeTxManager{}, repo, userRepo, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = service.CreatePR(context.Background(), &models.PRCreateRequest{ID: "p", Title: "t", AuthorID: "a"})
	if !errors.Is(err, ErrPRAuthorNotFound) {
		t.Fatalf("expected ErrPRAuthorNotFound, got %v", err)
	}
}

func TestPRService_GetUserReviews_EmptyList(t *testing.T) {
	repo := &fakePRRepo{
		getReviewerPRsFn: func(_ context.Context, _ string) ([]*models.PullRequestShort, error) {
			return nil, nil
		},
	}
	userRepo := &fakePRUserRepo{
		getUserFn: func(_ context.Context, _ string) (*models.UserWithTeam, error) {
			return &models.UserWithTeam{TeamName: "backend"}, nil
		},
	}
	service, err := NewPRService(fakeTxManager{}, repo, userRepo, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, err := service.GetUserReviews(context.Background(), " u1 ")
	if err != nil {
		t.Fatalf("GetUserReviews returned error: %v", err)
	}
	if len(resp.PullRequests) != 0 {
		t.Fatalf("expected empty slice, got %d", len(resp.PullRequests))
	}
}

func TestPRService_MergePR_Idempotent(t *testing.T) {
	updateCalled := false
	repo := &fakePRRepo{
		getPRFn: func(_ context.Context, _ string) (*models.PullRequest, error) {
			return &models.PullRequest{ID: "pr", Status: models.StatusMerged}, nil
		},
		updateStatusFn: func(_ context.Context, _ string, _ string) error {
			updateCalled = true
			return nil
		},
	}
	userRepo := &fakePRUserRepo{}
	service, err := NewPRService(fakeTxManager{}, repo, userRepo, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pr, err := service.MergePR(context.Background(), &models.PRMergeRequest{ID: "pr"})
	if err != nil {
		t.Fatalf("MergePR returned error: %v", err)
	}
	if pr.Status != models.StatusMerged {
		t.Fatalf("expected status MERGED, got %s", pr.Status)
	}
	if updateCalled {
		t.Fatalf("did not expect UpdatePRStatus to be called for already merged PR")
	}
}

func TestPRService_ReassignReviewer_Success(t *testing.T) {
	repo := &fakePRRepo{
		getPRFn: func(_ context.Context, _ string) (*models.PullRequest, error) {
			return &models.PullRequest{ID: "pr", Status: models.StatusOpen, Reviewers: []string{"u2", "u3"}}, nil
		},
		replaceReviewerFn: func(_ context.Context, _, oldID, newID string) error {
			if oldID != "u2" || newID != "u4" {
				return fmt.Errorf("unexpected replacement %s -> %s", oldID, newID)
			}
			return nil
		},
	}
	userRepo := &fakePRUserRepo{
		getUserFn: func(_ context.Context, userID string) (*models.UserWithTeam, error) {
			return &models.UserWithTeam{User: models.User{ID: userID}, TeamName: "backend"}, nil
		},
		getRandomMateFn: func(_ context.Context, _ string, _ []string) (*models.User, error) {
			return &models.User{ID: "u4"}, nil
		},
	}
	service, err := NewPRService(fakeTxManager{}, repo, userRepo, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, err := service.ReassignReviewer(context.Background(), &models.PRReassignRequest{ID: "pr", OldReviewerID: "u2"})
	if err != nil {
		t.Fatalf("ReassignReviewer returned error: %v", err)
	}
	if resp.ReplacedBy != "u4" {
		t.Fatalf("expected replacement u4, got %s", resp.ReplacedBy)
	}
	if resp.PR.Reviewers[0] != "u4" {
		t.Fatalf("expected reviewers to be updated, got %v", resp.PR.Reviewers)
	}
}

func TestPRService_ReassignReviewer_NoCandidate(t *testing.T) {
	repo := &fakePRRepo{
		getPRFn: func(_ context.Context, _ string) (*models.PullRequest, error) {
			return &models.PullRequest{ID: "pr", Status: models.StatusOpen, Reviewers: []string{"u2"}}, nil
		},
		replaceReviewerFn: func(context.Context, string, string, string) error {
			return nil
		},
	}
	userRepo := &fakePRUserRepo{
		getUserFn: func(_ context.Context, _ string) (*models.UserWithTeam, error) {
			return &models.UserWithTeam{TeamName: "backend"}, nil
		},
		getRandomMateFn: func(context.Context, string, []string) (*models.User, error) {
			return nil, storage.ErrNoCandidate
		},
	}
	service, err := NewPRService(fakeTxManager{}, repo, userRepo, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = service.ReassignReviewer(context.Background(), &models.PRReassignRequest{ID: "pr", OldReviewerID: "u2"})
	if !errors.Is(err, ErrNoReplacement) {
		t.Fatalf("expected ErrNoReplacement, got %v", err)
	}
}
