package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/cloudyy74/pr-reviewer-service/internal/models"
	"github.com/cloudyy74/pr-reviewer-service/internal/service"
)

const (
	ErrCodeTeamExists = "TEAM_EXISTS"
	ErrCodeBadRequest = "BAD_REQUEST"
	ErrCodeInternal   = "INTERNAL"
)

type TeamService interface {
	CreateTeam(ctx context.Context, team *models.Team) (*models.Team, error)
}

func (rtr *router) createTeam(w http.ResponseWriter, r *http.Request) {
	var team models.Team
	if err := json.NewDecoder(r.Body).Decode(&team); err != nil {
		rtr.responseError(w, http.StatusBadRequest, ErrCodeBadRequest, "bad json request")
		return
	}

	createdTeam, err := rtr.teamService.CreateTeam(r.Context(), &team)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTeamExists):
			rtr.responseError(w, http.StatusBadRequest, ErrCodeTeamExists, "team_name already exists")
		default:
			rtr.responseError(w, http.StatusInternalServerError, ErrCodeInternal, "internal error")
		}
		return
	}
	response := &models.TeamResponse{
		Team: *createdTeam,
	}
	rtr.responseJSON(w, http.StatusCreated, response)
}
