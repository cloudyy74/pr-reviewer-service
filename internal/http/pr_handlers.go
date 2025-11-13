package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/cloudyy74/pr-reviewer-service/internal/models"
	"github.com/cloudyy74/pr-reviewer-service/internal/service"
)

type PRService interface {
	CreatePR(context.Context, *models.PRCreateRequest) (*models.PullRequest, error)
	GetUserReviews(context.Context, string) (*models.UserReviewsResponse, error)
}

func (rtr *router) createPR(w http.ResponseWriter, r *http.Request) {
	var req models.PRCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rtr.responseError(w, http.StatusBadRequest, ErrCodeBadRequest, "bad json request")
		return
	}

	pr, err := rtr.prService.CreatePR(r.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPRValidation):
			rtr.responseError(w, http.StatusBadRequest, ErrCodeValidation, errors.Unwrap(err).Error())
		case errors.Is(err, service.ErrPRAuthorNotFound), errors.Is(err, service.ErrPRTeamNotFound):
			rtr.responseError(w, http.StatusNotFound, ErrCodeNotFound, "resource not found")
		case errors.Is(err, service.ErrPRAlreadyExists):
			rtr.responseError(w, http.StatusConflict, ErrCodePRExists, "pull request already exists")
		default:
			rtr.responseError(w, http.StatusInternalServerError, ErrCodeInternal, "internal error")
		}
		return
	}

	rtr.responseJSON(w, http.StatusCreated, &models.PRResponse{PR: *pr})
}

func (rtr *router) getUserReviews(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	resp, err := rtr.prService.GetUserReviews(r.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPRValidation):
			rtr.responseError(w, http.StatusBadRequest, ErrCodeValidation, errors.Unwrap(err).Error())
		case errors.Is(err, service.ErrUserNotFound):
			rtr.responseError(w, http.StatusNotFound, ErrCodeNotFound, "resource not found")
		default:
			rtr.responseError(w, http.StatusInternalServerError, ErrCodeInternal, "internal error")
		}
		return
	}

	rtr.responseJSON(w, http.StatusOK, resp)
}
