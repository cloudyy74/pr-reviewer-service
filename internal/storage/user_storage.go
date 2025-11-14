package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cloudyy74/pr-reviewer-service/internal/models"
	"github.com/cloudyy74/pr-reviewer-service/pkg/postgres"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrNoCandidate  = errors.New("no active candidate")
)

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

func (s *UserStorage) GetUserWithTeam(ctx context.Context, userID string) (*models.UserWithTeam, error) {
	exec := getQueryExecer(ctx, s.db.DB)
	var u models.UserWithTeam
	err := exec.QueryRowContext(
		ctx,
		`select id, username, team_name, is_active from users where id = $1`,
		userID,
	).Scan(&u.ID, &u.Username, &u.TeamName, &u.IsActive)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get user with team: %w", ErrUserNotFound)
	}
	if err != nil {
		s.log.Error("failed to get user with team", slog.Any("error", err))
		return nil, fmt.Errorf("get user with team: %w", err)
	}
	return &u, nil
}

func (s *UserStorage) GetActiveTeammates(ctx context.Context, teamName, excludeUserID string, limit int) ([]*models.User, error) {
	if limit <= 0 {
		return []*models.User{}, nil
	}
	exec := getQueryExecer(ctx, s.db.DB)
	rows, err := exec.QueryContext(
		ctx,
		`
select id, username, is_active
from users
where team_name = $1
  and is_active
  and id <> $2
order by random()
limit $3
`,
		teamName,
		excludeUserID,
		limit,
	)
	if err != nil {
		s.log.Error("failed to get teammates", slog.Any("error", err))
		return nil, fmt.Errorf("get teammates: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.IsActive); err != nil {
			return nil, fmt.Errorf("scan teammate: %w", err)
		}
		users = append(users, &u)
	}

	return users, nil
}

func (s *UserStorage) GetRandomActiveTeammate(ctx context.Context, teamName string, excludeIDs []string) (*models.User, error) {
	exec := getQueryExecer(ctx, s.db.DB)
	args := []any{teamName}
	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(`
select id, username, is_active
from users
where team_name = $1
  and is_active`)

	unique := make([]string, 0, len(excludeIDs))
	seen := make(map[string]struct{}, len(excludeIDs))
	for _, id := range excludeIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	if len(unique) > 0 {
		placeholders := make([]string, len(unique))
		for i, id := range unique {
			placeholders[i] = fmt.Sprintf("$%d", i+2)
			args = append(args, id)
		}
		queryBuilder.WriteString("\n  and id not in (" + strings.Join(placeholders, ", ") + ")")
	}
	queryBuilder.WriteString("\norder by random()\nlimit 1")

	var u models.User
	if err := exec.QueryRowContext(ctx, queryBuilder.String(), args...).Scan(&u.ID, &u.Username, &u.IsActive); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoCandidate
		}
		return nil, fmt.Errorf("get random teammate: %w", err)
	}

	return &u, nil
}
