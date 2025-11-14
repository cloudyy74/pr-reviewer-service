package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/cloudyy74/pr-reviewer-service/internal/models"
	"github.com/cloudyy74/pr-reviewer-service/internal/service"
)

type UserService interface {
	SetUserActive(context.Context, string, bool) (*models.UserWithTeam, error)
}

func (rtr *router) setUserActive(w http.ResponseWriter, r *http.Request) {
	var req models.SetActiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rtr.responseError(w, http.StatusBadRequest, ErrCodeBadRequest, "bad json request")
		return
	}

	resp, err := rtr.userService.SetUserActive(r.Context(), req.ID, req.IsActive)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			rtr.responseError(w, http.StatusNotFound, ErrCodeNotFound, "resource not found")
		case errors.Is(err, service.ErrUserValidation):
			rtr.responseError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		default:
			rtr.responseError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
		}
		return
	}
	rtr.responseJSON(w, http.StatusOK, resp)
}
