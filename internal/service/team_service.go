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

var (
	ErrValidation = errors.New("validation error")
	ErrTeamExists = errors.New("team already exists")
)

type TeamRepository interface {
	CreateTeam(ctx context.Context, teamName string) error
}

type UserRepository interface {
	UpsertUser(ctx context.Context, u models.User, teamName string) error
}

type txManager interface {
	Run(ctx context.Context, fn func(ctx context.Context) error) error
}

type TeamService struct {
	tx    txManager
	teams TeamRepository
	users UserRepository
	log   *slog.Logger
}

func NewTeamService(tx txManager, teams TeamRepository, users UserRepository, log *slog.Logger) (*TeamService, error) {
	if tx == nil {
		return nil, errors.New("tx manager cannot be nil")
	}
	if users == nil {
		return nil, errors.New("users repository cannot be nil")
	}
	if teams == nil {
		return nil, errors.New("teams repository cannot be nil")
	}
	if log == nil {
		return nil, errors.New("logger cannot be nil")
	}
	return &TeamService{
		tx:    tx,
		users: users,
		teams: teams,
		log:   log,
	}, nil
}

func (s *TeamService) CreateTeam(ctx context.Context, team *models.Team) (*models.Team, error) {
	if team == nil {
		return nil, fmt.Errorf("%w: empty body", ErrValidation)
	}
	team.Name = strings.TrimSpace(team.Name)
	if team.Name == "" {
		return nil, fmt.Errorf("%w: team_name is required", ErrValidation)
	}

	if team.Members == nil {
		team.Members = []*models.User{}
	}
	seen := make(map[string]struct{}, len(team.Members))
	uniq := make([]*models.User, 0, len(team.Members))
	for _, m := range team.Members {
		if m == nil {
			continue
		}
		m.ID = strings.TrimSpace(m.ID)
		m.Username = strings.TrimSpace(m.Username)
		if m.ID == "" || m.Username == "" {
			return nil, fmt.Errorf("%w: member requires user_id and username", ErrValidation)
		}
		if _, ok := seen[m.ID]; ok {
			continue
		}
		seen[m.ID] = struct{}{}
		uniq = append(uniq, m)
	}
	team.Members = uniq

	err := s.tx.Run(ctx, func(ctx context.Context) error {
		if err := s.teams.CreateTeam(ctx, team.Name); err != nil {
			if errors.Is(err, storage.ErrTeamExists) {
				return ErrTeamExists
			}
			return fmt.Errorf("service create team: %w", err)
		}

		for _, m := range team.Members {
			if err := s.users.UpsertUser(ctx, *m, team.Name); err != nil {
				return fmt.Errorf("service upsert user: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error in transcation: %w", err)
	}

	return team, nil
}
