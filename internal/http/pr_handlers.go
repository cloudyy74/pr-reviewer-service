package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/cloudyy74/pr-reviewer-service/internal/models"
)

type PRService interface {
	CreatePR(context.Context, *models.PRCreateRequest) (*models.PullRequest, error)
	GetUserReviews(context.Context, string) (*models.UserReviewsResponse, error)
	MergePR(context.Context, *models.PRMergeRequest) (*models.PullRequest, error)
	ReassignReviewer(context.Context, *models.PRReassignRequest) (*models.PRReassignResponse, error)
	GetAssignmentsStats(context.Context) (*models.AssignmentsStatsResponse, error)
}

func (rtr *router) createPR(w http.ResponseWriter, r *http.Request) {
	var req models.PRCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rtr.handleError(w, newResponseError(ErrCodeBadRequest, "bad json request"))
		return
	}

	pr, err := rtr.prService.CreatePR(r.Context(), &req)
	if err != nil {
		rtr.handleError(w, err)
		return
	}

	rtr.responseJSON(w, http.StatusCreated, &models.PRResponse{PR: *pr})
}

func (rtr *router) getUserReviews(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	resp, err := rtr.prService.GetUserReviews(r.Context(), userID)
	if err != nil {
		rtr.handleError(w, err)
		return
	}

	rtr.responseJSON(w, http.StatusOK, resp)
}

func (rtr *router) mergePR(w http.ResponseWriter, r *http.Request) {
	var req models.PRMergeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rtr.handleError(w, newResponseError(ErrCodeBadRequest, "bad json request"))
		return
	}

	pr, err := rtr.prService.MergePR(r.Context(), &req)
	if err != nil {
		rtr.handleError(w, err)
		return
	}

	rtr.responseJSON(w, http.StatusOK, &models.PRResponse{PR: *pr})
}

func (rtr *router) reassignPR(w http.ResponseWriter, r *http.Request) {
	var req models.PRReassignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rtr.handleError(w, newResponseError(ErrCodeBadRequest, "bad json request"))
		return
	}

	resp, err := rtr.prService.ReassignReviewer(r.Context(), &req)
	if err != nil {
		rtr.handleError(w, err)
		return
	}

	rtr.responseJSON(w, http.StatusOK, resp)
}

func (rtr *router) getAssignmentsStats(w http.ResponseWriter, r *http.Request) {
	stats, err := rtr.prService.GetAssignmentsStats(r.Context())
	if err != nil {
		rtr.handleError(w, err)
		return
	}
	rtr.responseJSON(w, http.StatusOK, stats)
}
