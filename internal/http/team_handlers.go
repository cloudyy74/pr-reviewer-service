package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/cloudyy74/pr-reviewer-service/internal/models"
)

type TeamService interface {
	CreateTeam(context.Context, *models.Team) (*models.Team, error)
	GetTeamUsers(context.Context, string) ([]*models.User, error)
	DeactivateTeamUsers(context.Context, string) (*models.TeamDeactivateResponse, error)
}

func (rtr *router) createTeam(w http.ResponseWriter, r *http.Request) {
	var team models.Team
	if err := json.NewDecoder(r.Body).Decode(&team); err != nil {
		rtr.handleError(w, newResponseError(ErrCodeBadRequest, "bad json request"))
		return
	}

	createdTeam, err := rtr.teamService.CreateTeam(r.Context(), &team)
	if err != nil {
		rtr.handleError(w, err)
		return
	}

	response := &models.TeamResponse{
		Team: *createdTeam,
	}
	rtr.responseJSON(w, http.StatusCreated, response)
}

func (rtr *router) getTeam(w http.ResponseWriter, r *http.Request) {
	teamName := r.URL.Query().Get("team_name")
	users, err := rtr.teamService.GetTeamUsers(r.Context(), teamName)
	if err != nil {
		rtr.handleError(w, err)
		return
	}

	response := &models.Team{
		Name:    teamName,
		Members: users,
	}
	rtr.responseJSON(w, http.StatusOK, response)
}

func (rtr *router) deactivateTeamUsers(w http.ResponseWriter, r *http.Request) {
	var req models.TeamDeactivateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rtr.handleError(w, newResponseError(ErrCodeBadRequest, "bad json request"))
		return
	}
	resp, err := rtr.teamService.DeactivateTeamUsers(r.Context(), req.TeamName)
	if err != nil {
		rtr.handleError(w, err)
		return
	}
	rtr.responseJSON(w, http.StatusOK, resp)
}
