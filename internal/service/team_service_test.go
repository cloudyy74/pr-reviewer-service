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

type fakeTeamTx struct {
	runFn func(context.Context, func(context.Context) error) error
}

func (f fakeTeamTx) Run(ctx context.Context, fn func(context.Context) error) error {
	if f.runFn != nil {
		return f.runFn(ctx, fn)
	}
	return fn(ctx)
}

type fakeTeamsRepo struct {
	createFn func(context.Context, string) error
	existsFn func(context.Context, string) (bool, error)
}

func (f *fakeTeamsRepo) CreateTeam(ctx context.Context, name string) error {
	if f.createFn != nil {
		return f.createFn(ctx, name)
	}
	return nil
}

func (f *fakeTeamsRepo) ExistsTeam(ctx context.Context, name string) (bool, error) {
	if f.existsFn != nil {
		return f.existsFn(ctx, name)
	}
	return false, nil
}

type fakeTeamUsersRepo struct {
	upsertFn   func(context.Context, models.User, string) error
	getUsersFn func(context.Context, string) ([]*models.User, error)
}

func (f *fakeTeamUsersRepo) UpsertUser(ctx context.Context, u models.User, teamName string) error {
	if f.upsertFn != nil {
		return f.upsertFn(ctx, u, teamName)
	}
	return nil
}

func (f *fakeTeamUsersRepo) GetUsersByTeam(ctx context.Context, teamName string) ([]*models.User, error) {
	if f.getUsersFn != nil {
		return f.getUsersFn(ctx, teamName)
	}
	return nil, nil
}

func teamTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewTeamService_Validation(t *testing.T) {
	_, err := NewTeamService(nil, nil, nil, nil)
	if err == nil {
		t.Fatalf("expected error when dependencies are nil")
	}
}

func TestTeamService_CreateTeam_Success(t *testing.T) {
	var createdTeam string
	var upserted []models.User
	service, err := NewTeamService(
		fakeTeamTx{},
		&fakeTeamsRepo{
			createFn: func(_ context.Context, name string) error {
				createdTeam = name
				return nil
			},
		},
		&fakeTeamUsersRepo{
			upsertFn: func(_ context.Context, u models.User, team string) error {
				upserted = append(upserted, u)
				return nil
			},
			getUsersFn: nil,
		},
		teamTestLogger(),
	)
	if err != nil {
		t.Fatalf("NewTeamService returned err: %v", err)
	}

	input := &models.Team{
		Name: " backend ",
		Members: []*models.User{
			{ID: "u1", Username: "Alice"},
			{ID: "u1", Username: "Alice"}, // duplicate
			nil,
			{ID: "u2", Username: "Bob"},
		},
	}

	team, err := service.CreateTeam(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateTeam returned err: %v", err)
	}
	if team.Name != "backend" {
		t.Fatalf("team name not trimmed: %#v", team.Name)
	}
	if createdTeam != "backend" {
		t.Fatalf("CreateTeam did not call repository with trimmed name")
	}
	if len(upserted) != 2 {
		t.Fatalf("expected 2 unique members, got %d", len(upserted))
	}
}

func TestTeamService_CreateTeam_TeamExists(t *testing.T) {
	service, err := NewTeamService(
		fakeTeamTx{},
		&fakeTeamsRepo{
			createFn: func(context.Context, string) error {
				return storage.ErrTeamExists
			},
		},
		&fakeTeamUsersRepo{
			upsertFn:   func(context.Context, models.User, string) error { return nil },
			getUsersFn: nil,
		},
		teamTestLogger(),
	)
	if err != nil {
		t.Fatalf("NewTeamService returned err: %v", err)
	}

	_, err = service.CreateTeam(context.Background(), &models.Team{Name: "backend"})
	if !errors.Is(err, ErrTeamExists) {
		t.Fatalf("expected ErrTeamExists, got %v", err)
	}
}

func TestTeamService_CreateTeam_Validation(t *testing.T) {
	service, err := NewTeamService(
		fakeTeamTx{},
		&fakeTeamsRepo{
			createFn: func(context.Context, string) error { return nil },
			existsFn: nil,
		},
		&fakeTeamUsersRepo{
			upsertFn:   func(context.Context, models.User, string) error { return nil },
			getUsersFn: nil,
		},
		teamTestLogger(),
	)
	if err != nil {
		t.Fatalf("NewTeamService returned err: %v", err)
	}

	_, err = service.CreateTeam(context.Background(), &models.Team{Name: "   "})
	if !errors.Is(err, ErrTeamValidation) {
		t.Fatalf("expected ErrTeamValidation, got %v", err)
	}
}

func TestTeamService_GetTeamUsers_Success(t *testing.T) {
	service, err := NewTeamService(
		fakeTeamTx{},
		&fakeTeamsRepo{
			createFn: nil,
			existsFn: func(context.Context, string) (bool, error) { return true, nil },
		},
		&fakeTeamUsersRepo{
			upsertFn: nil,
			getUsersFn: func(context.Context, string) ([]*models.User, error) {
				return []*models.User{{ID: "u1"}}, nil
			},
		},
		teamTestLogger(),
	)
	if err != nil {
		t.Fatalf("NewTeamService returned err: %v", err)
	}

	users, err := service.GetTeamUsers(context.Background(), " backend ")
	if err != nil {
		t.Fatalf("GetTeamUsers returned err: %v", err)
	}
	if len(users) != 1 || users[0].ID != "u1" {
		t.Fatalf("unexpected users: %#v", users)
	}
}

func TestTeamService_GetTeamUsers_NotFound(t *testing.T) {
	service, err := NewTeamService(
		fakeTeamTx{},
		&fakeTeamsRepo{
			createFn: nil,
			existsFn: func(context.Context, string) (bool, error) { return false, nil },
		},
		&fakeTeamUsersRepo{
			upsertFn: nil,
			getUsersFn: func(context.Context, string) ([]*models.User, error) {
				return nil, nil
			},
		},
		teamTestLogger(),
	)
	if err != nil {
		t.Fatalf("NewTeamService returned err: %v", err)
	}

	_, err = service.GetTeamUsers(context.Background(), "backend")
	if !errors.Is(err, ErrTeamNotFound) {
		t.Fatalf("expected ErrTeamNotFound, got %v", err)
	}
}

func TestTeamService_GetTeamUsers_Validation(t *testing.T) {
	service, err := NewTeamService(
		fakeTeamTx{},
		&fakeTeamsRepo{
			createFn: nil,
			existsFn: func(context.Context, string) (bool, error) { return true, nil },
		},
		&fakeTeamUsersRepo{
			upsertFn:   nil,
			getUsersFn: func(context.Context, string) ([]*models.User, error) { return nil, nil },
		},
		teamTestLogger(),
	)
	if err != nil {
		t.Fatalf("NewTeamService returned err: %v", err)
	}

	_, err = service.GetTeamUsers(context.Background(), "  ")
	if !errors.Is(err, ErrTeamValidation) {
		t.Fatalf("expected ErrTeamValidation, got %v", err)
	}
}
