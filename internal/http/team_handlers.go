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
)

type TeamService interface {
	CreateTeam(context.Context, *models.Team) (*models.Team, error)
	GetTeamUsers(context.Context, string) ([]*models.User, error)
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
		case errors.Is(err, service.ErrTeamValidation):
			rtr.responseError(w, http.StatusBadRequest, ErrCodeValidation, errors.Unwrap(err).Error())
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

func (rtr *router) getTeam(w http.ResponseWriter, r *http.Request) {
	teamName := r.URL.Query().Get("team_name")
	users, err := rtr.teamService.GetTeamUsers(r.Context(), teamName)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTeamNotFound):
			rtr.responseError(w, http.StatusBadRequest, ErrCodeNotFound, "resource not found")
		case errors.Is(err, service.ErrTeamValidation):
			rtr.responseError(w, http.StatusBadRequest, ErrCodeValidation, errors.Unwrap(err).Error())
		default:
			rtr.responseError(w, http.StatusInternalServerError, ErrCodeInternal, "internal error")
		}
		return
	}

	response := &models.Team{
		Name:    teamName,
		Members: users,
	}
	rtr.responseJSON(w, http.StatusOK, response)
}
