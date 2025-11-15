package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudyy74/pr-reviewer-service/internal/models"
	"github.com/cloudyy74/pr-reviewer-service/internal/service"
)

type fakeTeamService struct {
	createFn func(ctx context.Context, team *models.Team) (*models.Team, error)
	getFn    func(ctx context.Context, teamName string) ([]*models.User, error)
}

func (f *fakeTeamService) CreateTeam(ctx context.Context, team *models.Team) (*models.Team, error) {
	if f.createFn == nil {
		return nil, errors.New("not implemented")
	}
	return f.createFn(ctx, team)
}

func (f *fakeTeamService) GetTeamUsers(ctx context.Context, teamName string) ([]*models.User, error) {
	if f.getFn == nil {
		return nil, errors.New("not implemented")
	}
	return f.getFn(ctx, teamName)
}

func newTestRouterWithTeamService(svc TeamService) *router {
	return &router{
		teamService: svc,
		log:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestCreateTeam_Success(t *testing.T) {
	want := &models.Team{
		Name: "backend",
		Members: []*models.User{
			{ID: "u1", Username: "john"},
		},
	}
	svc := &fakeTeamService{
		createFn: func(ctx context.Context, team *models.Team) (*models.Team, error) {
			if team == nil {
				t.Fatalf("expected team payload")
			}
			if team.Name != want.Name {
				t.Fatalf("expected team name %s, got %s", want.Name, team.Name)
			}
			return want, nil
		},
	}
	rtr := newTestRouterWithTeamService(svc)

	body := `{"team_name":"backend","members":[{"user_id":"u1","username":"john"}]}`
	req := httptest.NewRequest(http.MethodPost, "/team/add", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	rtr.createTeam(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}
	var resp models.TeamResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Team.Name != want.Name {
		t.Fatalf("unexpected team name %s", resp.Team.Name)
	}
	if len(resp.Team.Members) != len(want.Members) || resp.Team.Members[0].ID != want.Members[0].ID {
		t.Fatalf("unexpected members: %+v", resp.Team.Members)
	}
}

func TestCreateTeam_BadJSON(t *testing.T) {
	svc := &fakeTeamService{
		createFn: func(context.Context, *models.Team) (*models.Team, error) {
			t.Fatalf("service should not be called")
			return nil, nil
		},
	}
	rtr := newTestRouterWithTeamService(svc)

	req := httptest.NewRequest(http.MethodPost, "/team/add", bytes.NewBufferString("{bad json"))
	rec := httptest.NewRecorder()

	rtr.createTeam(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
	var resp models.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Error.Code != ErrCodeBadRequest {
		t.Fatalf("expected code %s, got %s", ErrCodeBadRequest, resp.Error.Code)
	}
	if resp.Error.Message != "bad json request" {
		t.Fatalf("unexpected message: %s", resp.Error.Message)
	}
}

func TestCreateTeam_TeamExists(t *testing.T) {
	svc := &fakeTeamService{
		createFn: func(context.Context, *models.Team) (*models.Team, error) {
			return nil, service.ErrTeamExists
		},
	}
	rtr := newTestRouterWithTeamService(svc)

	req := httptest.NewRequest(http.MethodPost, "/team/add", bytes.NewBufferString(`{"team_name":"backend"}`))
	rec := httptest.NewRecorder()

	rtr.createTeam(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
	var resp models.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Error.Code != ErrCodeTeamExists {
		t.Fatalf("expected code %s, got %s", ErrCodeTeamExists, resp.Error.Code)
	}
	if resp.Error.Message != "team_name already exists" {
		t.Fatalf("unexpected message: %s", resp.Error.Message)
	}
}

func TestCreateTeam_ValidationError(t *testing.T) {
	validationErr := fmt.Errorf("%w: team_name is required", service.ErrTeamValidation)
	svc := &fakeTeamService{
		createFn: func(context.Context, *models.Team) (*models.Team, error) {
			return nil, validationErr
		},
	}
	rtr := newTestRouterWithTeamService(svc)

	req := httptest.NewRequest(http.MethodPost, "/team/add", bytes.NewBufferString(`{"team_name":""}`))
	rec := httptest.NewRecorder()

	rtr.createTeam(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
	var resp models.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Error.Code != ErrCodeValidation {
		t.Fatalf("expected code %s, got %s", ErrCodeValidation, resp.Error.Code)
	}
	if resp.Error.Message != validationErr.Error() {
		t.Fatalf("expected message %s, got %s", validationErr.Error(), resp.Error.Message)
	}
}

func TestCreateTeam_InternalError(t *testing.T) {
	internalErr := errors.New("db timeout")
	svc := &fakeTeamService{
		createFn: func(context.Context, *models.Team) (*models.Team, error) {
			return nil, internalErr
		},
	}
	rtr := newTestRouterWithTeamService(svc)

	req := httptest.NewRequest(http.MethodPost, "/team/add", bytes.NewBufferString(`{"team_name":"backend"}`))
	rec := httptest.NewRecorder()

	rtr.createTeam(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
	var resp models.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Error.Code != ErrCodeInternal {
		t.Fatalf("expected code %s, got %s", ErrCodeInternal, resp.Error.Code)
	}
	if resp.Error.Message != "internal error" {
		t.Fatalf("unexpected message: %s", resp.Error.Message)
	}
}

func TestGetTeam_Success(t *testing.T) {
	users := []*models.User{
		{ID: "u1", Username: "alice"},
		{ID: "u2", Username: "bob"},
	}
	svc := &fakeTeamService{
		getFn: func(ctx context.Context, teamName string) ([]*models.User, error) {
			if teamName != "backend" {
				t.Fatalf("expected team name backend, got %s", teamName)
			}
			return users, nil
		},
	}
	rtr := newTestRouterWithTeamService(svc)

	req := httptest.NewRequest(http.MethodGet, "/team/get?team_name=backend", nil)
	rec := httptest.NewRecorder()

	rtr.getTeam(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var resp models.Team
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Name != "backend" {
		t.Fatalf("unexpected team name %s", resp.Name)
	}
	if len(resp.Members) != len(users) {
		t.Fatalf("unexpected members count %d", len(resp.Members))
	}
}

func TestGetTeam_NotFound(t *testing.T) {
	svc := &fakeTeamService{
		getFn: func(context.Context, string) ([]*models.User, error) {
			return nil, service.ErrTeamNotFound
		},
	}
	rtr := newTestRouterWithTeamService(svc)

	req := httptest.NewRequest(http.MethodGet, "/team/get?team_name=unknown", nil)
	rec := httptest.NewRecorder()

	rtr.getTeam(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
	var resp models.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Error.Code != ErrCodeNotFound {
		t.Fatalf("expected code %s, got %s", ErrCodeNotFound, resp.Error.Code)
	}
	if resp.Error.Message != "resource not found" {
		t.Fatalf("unexpected message: %s", resp.Error.Message)
	}
}

func TestGetTeam_ValidationError(t *testing.T) {
	valErr := fmt.Errorf("%w: team_name is required", service.ErrTeamValidation)
	svc := &fakeTeamService{
		getFn: func(context.Context, string) ([]*models.User, error) {
			return nil, valErr
		},
	}
	rtr := newTestRouterWithTeamService(svc)

	req := httptest.NewRequest(http.MethodGet, "/team/get?team_name=", nil)
	rec := httptest.NewRecorder()

	rtr.getTeam(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
	var resp models.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Error.Code != ErrCodeValidation {
		t.Fatalf("expected code %s, got %s", ErrCodeValidation, resp.Error.Code)
	}
	if resp.Error.Message != service.ErrTeamValidation.Error() {
		t.Fatalf("expected message %s, got %s", service.ErrTeamValidation.Error(), resp.Error.Message)
	}
}

func TestGetTeam_InternalError(t *testing.T) {
	svc := &fakeTeamService{
		getFn: func(context.Context, string) ([]*models.User, error) {
			return nil, errors.New("db down")
		},
	}
	rtr := newTestRouterWithTeamService(svc)

	req := httptest.NewRequest(http.MethodGet, "/team/get?team_name=backend", nil)
	rec := httptest.NewRecorder()

	rtr.getTeam(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
	var resp models.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Error.Code != ErrCodeInternal {
		t.Fatalf("expected code %s, got %s", ErrCodeInternal, resp.Error.Code)
	}
	if resp.Error.Message != "internal error" {
		t.Fatalf("unexpected message: %s", resp.Error.Message)
	}
}
