package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/cloudyy74/pr-reviewer-service/internal/models"
)

type UserService interface {
	SetUserActive(context.Context, string, bool) (*models.UserResponse, error)
}

func (rtr *router) setUserActive(w http.ResponseWriter, r *http.Request) {
	var req models.SetActiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rtr.handleError(w, newResponseError(ErrCodeBadRequest, "bad json request"))
		return
	}

	resp, err := rtr.userService.SetUserActive(r.Context(), req.ID, req.IsActive)
	if err != nil {
		rtr.handleError(w, err)
		return
	}
	rtr.responseJSON(w, http.StatusOK, resp)
}
