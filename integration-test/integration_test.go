package integration_test

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudyy74/pr-reviewer-service/internal/models"
	"github.com/cloudyy74/pr-reviewer-service/internal/service"
	"github.com/cloudyy74/pr-reviewer-service/internal/storage"
	"github.com/cloudyy74/pr-reviewer-service/pkg/postgres"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	upMigrations = []string{
		"../internal/data/000001_users_teams_tables.up.sql",
		"../internal/data/000002_pr_tables.up.sql",
		"../internal/data/000003_pr_statuses.up.sql",
	}
	downMigrations = []string{
		"../internal/data/000003_pr_statuses.down.sql",
		"../internal/data/000002_pr_tables.down.sql",
		"../internal/data/000001_users_teams_tables.down.sql",
	}
)

func setupIntegrationDB(t *testing.T) (*postgres.Postgres, *slog.Logger) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if strings.TrimSpace(dsn) == "" {
		t.Skip("TEST_DATABASE_URL is not set, skipping integration tests")
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	pg, err := postgres.New(context.Background(), dsn, log)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}
	t.Cleanup(func() {
		pg.Close()
	})

	resetDatabase(t, pg.DB)
	return pg, log
}

func resetDatabase(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, path := range downMigrations {
		execSQLFile(t, db, path)
	}
	for _, path := range upMigrations {
		execSQLFile(t, db, path)
	}
}

func execSQLFile(t *testing.T, db *sql.DB, path string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read sql file %s: %v", path, err)
	}
	query := strings.TrimSpace(string(data))
	if query == "" {
		return
	}
	if _, err := db.Exec(query); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
			return
		}
		t.Fatalf("exec sql %s: %v", path, err)
	}
}

func newServices(t *testing.T, pg *postgres.Postgres, log *slog.Logger) (*service.TeamService, *service.UserService, *service.PRService) {
	t.Helper()
	teamStorage, err := storage.NewTeamStorage(pg, log)
	if err != nil {
		t.Fatalf("team storage: %v", err)
	}
	userStorage, err := storage.NewUserStorage(pg, log)
	if err != nil {
		t.Fatalf("user storage: %v", err)
	}
	prStorage, err := storage.NewPRStorage(pg, log)
	if err != nil {
		t.Fatalf("pr storage: %v", err)
	}
	txManager, err := storage.NewTxManager(pg, log)
	if err != nil {
		t.Fatalf("tx manager: %v", err)
	}

	teamSvc, err := service.NewTeamService(txManager, teamStorage, userStorage, log)
	if err != nil {
		t.Fatalf("team service: %v", err)
	}
	userSvc, err := service.NewUserService(txManager, userStorage, log)
	if err != nil {
		t.Fatalf("user service: %v", err)
	}
	prSvc, err := service.NewPRService(txManager, prStorage, userStorage, log)
	if err != nil {
		t.Fatalf("pr service: %v", err)
	}
	return teamSvc, userSvc, prSvc
}

func TestIntegrationTeamLifecycle(t *testing.T) {
	pg, log := setupIntegrationDB(t)
	teamSvc, userSvc, _ := newServices(t, pg, log)

	ctx := context.Background()
	team := &models.Team{
		Name: "backend",
		Members: []*models.User{
			{ID: "u1", Username: "alice", IsActive: true},
			{ID: "u2", Username: "bob", IsActive: true},
		},
	}
	created, err := teamSvc.CreateTeam(ctx, team)
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}
	if created.Name != team.Name {
		t.Fatalf("expected team name %s, got %s", team.Name, created.Name)
	}

	users, err := teamSvc.GetTeamUsers(ctx, "backend")
	if err != nil {
		t.Fatalf("GetTeamUsers: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}

	updated, err := userSvc.SetUserActive(ctx, "u2", false)
	if err != nil {
		t.Fatalf("SetUserActive: %v", err)
	}
	if updated.User.IsActive {
		t.Fatalf("expected user u2 to be inactive")
	}

	resp, err := teamSvc.DeactivateTeamUsers(ctx, "backend")
	if err != nil {
		t.Fatalf("DeactivateTeamUsers: %v", err)
	}
	if resp.DeactivatedCount == 0 {
		t.Fatalf("expected at least one deactivated user, got %d", resp.DeactivatedCount)
	}
}

func TestIntegrationPRWorkflow(t *testing.T) {
	pg, log := setupIntegrationDB(t)
	teamSvc, _, prSvc := newServices(t, pg, log)

	ctx := context.Background()
	team := &models.Team{
		Name: "platform",
		Members: []*models.User{
			{ID: "author", Username: "author", IsActive: true},
			{ID: "reviewer1", Username: "reviewer1", IsActive: true},
			{ID: "reviewer2", Username: "reviewer2", IsActive: true},
			{ID: "reviewer3", Username: "reviewer3", IsActive: true},
		},
	}
	if _, err := teamSvc.CreateTeam(ctx, team); err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}

	pr, err := prSvc.CreatePR(ctx, &models.PRCreateRequest{
		ID:       "pr-1",
		Title:    "add feature",
		AuthorID: "author",
	})
	if err != nil {
		t.Fatalf("CreatePR: %v", err)
	}
	if len(pr.Reviewers) != 2 {
		t.Fatalf("expected 2 reviewers, got %d", len(pr.Reviewers))
	}

	reviews, err := prSvc.GetUserReviews(ctx, pr.Reviewers[0])
	if err != nil {
		t.Fatalf("GetUserReviews: %v", err)
	}
	if len(reviews.PullRequests) != 1 {
		t.Fatalf("expected reviewer to have 1 assigned PR, got %d", len(reviews.PullRequests))
	}

	oldReviewer := pr.Reviewers[0]
	reassignResp, err := prSvc.ReassignReviewer(ctx, &models.PRReassignRequest{
		ID:            pr.ID,
		OldReviewerID: oldReviewer,
	})
	if err != nil {
		t.Fatalf("ReassignReviewer: %v", err)
	}
	if reassignResp.ReplacedBy == oldReviewer {
		t.Fatalf("expected reviewer to be replaced with a new teammate")
	}

	merged, err := prSvc.MergePR(ctx, &models.PRMergeRequest{ID: pr.ID})
	if err != nil {
		t.Fatalf("MergePR: %v", err)
	}
	if merged.Status != models.StatusMerged {
		t.Fatalf("expected PR status MERGED, got %s", merged.Status)
	}
	if merged.MergedAt == nil {
		t.Fatalf("expected merged_at to be set")
	}

	stats, err := prSvc.GetAssignmentsStats(ctx)
	if err != nil {
		t.Fatalf("GetAssignmentsStats: %v", err)
	}
	if len(stats.ByPR) != 1 || stats.ByPR[0].PullRequestID != pr.ID || stats.ByPR[0].Reviewers != 2 {
		t.Fatalf("unexpected PR stats: %#v", stats.ByPR)
	}
	totalAssignments := 0
	for _, stat := range stats.ByUser {
		totalAssignments += stat.Assignments
	}
	if totalAssignments != 2 {
		t.Fatalf("expected total assignments 2, got %d", totalAssignments)
	}
}
