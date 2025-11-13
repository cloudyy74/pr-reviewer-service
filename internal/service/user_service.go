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
	ErrUserValidation = errors.New("validation error")
	ErrUserNotFound   = errors.New("user not found")
)

type UserRepository interface {
	SetUserActive(context.Context, string, bool) (*models.UserWithTeam, error)
}

type UserService struct {
	tx    txManager
	users UserRepository
	log   *slog.Logger
}

func NewUserService(tx txManager, users UserRepository, log *slog.Logger) (*UserService, error) {
	if tx == nil {
		return nil, errors.New("tx manager cannot be nil")
	}
	if users == nil {
		return nil, errors.New("users repository cannot be nil")
	}
	if log == nil {
		return nil, errors.New("logger cannot be nil")
	}
	return &UserService{
		tx:    tx,
		users: users,
		log:   log,
	}, nil
}

func (s *UserService) SetUserActive(ctx context.Context, userID string, isActive bool) (*models.UserWithTeam, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("%w: user_id is required", ErrUserValidation)
	}

	u, err := s.users.SetUserActive(ctx, userID, isActive)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrUserNotFound):
			return nil, fmt.Errorf("set user active: %w", ErrUserNotFound)
		default:
			return nil, fmt.Errorf("set user active: %w", err)
		}
	}

	return u, nil
}
