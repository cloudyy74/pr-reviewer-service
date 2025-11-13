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

var ErrUserNotFound = errors.New("user not found")

type UserStorage struct {
	db  *postgres.Postgres
	log *slog.Logger
}

func NewUserStorage(db *postgres.Postgres, log *slog.Logger) (*UserStorage, error) {
	if db == nil {
		return nil, errors.New("database cannot be nil")
	}
	if log == nil {
		return nil, errors.New("logger cannot be nil")
	}
	return &UserStorage{
		db:  db,
		log: log,
	}, nil
}

func (s *UserStorage) UpsertUser(ctx context.Context, u models.User, teamName string) error {
	exec := getExecer(ctx, s.db.DB)
	_, err := exec.ExecContext(
		ctx,
		`
insert into users (id, username, team_name, is_active) values ($1, $2, $3, $4) on conflict (id) do update set
username = excluded.username,
team_name = excluded.team_name,
is_active = excluded.is_active`,
		u.ID,
		u.Username,
		teamName,
		u.IsActive,
	)
	if err != nil {
		s.log.Error("failed to upsert user", slog.Any("error", err))
		return fmt.Errorf("upsert user: %w", err)
	}
	return nil
}

func (s *UserStorage) GetUsersByTeam(ctx context.Context, teamName string) ([]*models.User, error) {
	exec := getQueryExecer(ctx, s.db.DB)
	rows, err := exec.QueryContext(
		ctx,
		`
select id, username, is_active from users
where team_name = $1
`,
		teamName,
	)
	if err != nil {
		s.log.Error("failed to get users by team", slog.Any("error", err))
		return nil, fmt.Errorf("get users by team: %w", err)
	}
	defer rows.Close()

	var users []*models.User

	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.IsActive); err != nil {
			return nil, fmt.Errorf("get users by team: %w", err)
		}
		users = append(users, &u)
	}

	return users, nil
}

func (s *UserStorage) SetUserActive(ctx context.Context, userID string, isActive bool) (*models.UserWithTeam, error) {
    exec := getQueryExecer(ctx, s.db.DB)
	var u models.UserWithTeam
    err := exec.QueryRowContext(ctx, 
		`update users set is_active = $1 where id = $2
		 returning id, username, team_name, is_active`, 
		isActive, 
		userID,
	).Scan(&u.ID, &u.Username, &u.TeamName, &u.IsActive)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("set user active: %w", ErrUserNotFound)
	}
    if err != nil {
        return nil, fmt.Errorf("set user active: %w", err)
    }

    return &u, nil
}
