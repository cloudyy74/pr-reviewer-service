package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/cloudyy74/pr-reviewer-service/internal/models"
	"github.com/cloudyy74/pr-reviewer-service/internal/storage"
)

type fakeUserSetRepo struct {
	setUserActiveFn func(context.Context, string, bool) (*models.UserWithTeam, error)
}

func (f *fakeUserSetRepo) SetUserActive(ctx context.Context, userID string, isActive bool) (*models.UserWithTeam, error) {
	return f.setUserActiveFn(ctx, userID, isActive)
}

type fakeTx struct{}

func (fakeTx) Run(_ context.Context, fn func(ctx context.Context) error) error {
	return fn(context.Background())
}

func userTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewUserService_Validation(t *testing.T) {
	_, err := NewUserService(nil, nil, nil)
	if err == nil {
		t.Fatalf("expected error when dependencies are nil")
	}
}

func TestUserService_SetUserActive_Success(t *testing.T) {
	repo := &fakeUserSetRepo{
		setUserActiveFn: func(_ context.Context, userID string, isActive bool) (*models.UserWithTeam, error) {
			return &models.UserWithTeam{User: models.User{ID: userID, IsActive: isActive}, TeamName: "backend"}, nil
		},
	}
	service, err := NewUserService(fakeTx{}, repo, userTestLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	user, err := service.SetUserActive(context.Background(), " user-1 ", true)
	if err != nil {
		t.Fatalf("SetUserActive returned error: %v", err)
	}
	if user.ID != "user-1" || !user.IsActive {
		t.Fatalf("unexpected user returned: %#v", user)
	}
}

func TestUserService_SetUserActive_UserNotFound(t *testing.T) {
	repo := &fakeUserSetRepo{
		setUserActiveFn: func(context.Context, string, bool) (*models.UserWithTeam, error) {
			return nil, storage.ErrUserNotFound
		},
	}
	service, err := NewUserService(fakeTx{}, repo, userTestLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = service.SetUserActive(context.Background(), "user-1", true)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserService_SetUserActive_Validation(t *testing.T) {
	repo := &fakeUserSetRepo{}
	service, err := NewUserService(fakeTx{}, repo, userTestLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = service.SetUserActive(context.Background(), " \t \n ", true)
	if !errors.Is(err, ErrUserValidation) {
		t.Fatalf("expected ErrUserValidation, got %v", err)
	}
}
