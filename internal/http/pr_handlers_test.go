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
	"strings"
	"testing"

	"github.com/cloudyy74/pr-reviewer-service/internal/models"
	"github.com/cloudyy74/pr-reviewer-service/internal/service"
)

type fakePRService struct {
	createFn   func(ctx context.Context, req *models.PRCreateRequest) (*models.PullRequest, error)
	reviewsFn  func(ctx context.Context, userID string) (*models.UserReviewsResponse, error)
	mergeFn    func(ctx context.Context, req *models.PRMergeRequest) (*models.PullRequest, error)
	reassignFn func(ctx context.Context, req *models.PRReassignRequest) (*models.PRReassignResponse, error)
	statsFn    func(ctx context.Context) (*models.AssignmentsStatsResponse, error)
}

func (f *fakePRService) CreatePR(ctx context.Context, req *models.PRCreateRequest) (*models.PullRequest, error) {
	if f.createFn == nil {
		return nil, errors.New("not implemented")
	}
	return f.createFn(ctx, req)
}

func (f *fakePRService) GetUserReviews(ctx context.Context, userID string) (*models.UserReviewsResponse, error) {
	if f.reviewsFn == nil {
		return nil, errors.New("not implemented")
	}
	return f.reviewsFn(ctx, userID)
}

func (f *fakePRService) MergePR(ctx context.Context, req *models.PRMergeRequest) (*models.PullRequest, error) {
	if f.mergeFn == nil {
		return nil, errors.New("not implemented")
	}
	return f.mergeFn(ctx, req)
}

func (f *fakePRService) ReassignReviewer(ctx context.Context, req *models.PRReassignRequest) (*models.PRReassignResponse, error) {
	if f.reassignFn == nil {
		return nil, errors.New("not implemented")
	}
	return f.reassignFn(ctx, req)
}

func (f *fakePRService) GetAssignmentsStats(ctx context.Context) (*models.AssignmentsStatsResponse, error) {
	if f.statsFn == nil {
		return nil, errors.New("not implemented")
	}
	return f.statsFn(ctx)
}

func newTestRouterWithPRService(svc PRService) *router {
	return &router{
		prService: svc,
		log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestCreatePR_Success(t *testing.T) {
	want := &models.PullRequest{
		ID:       "123",
		Title:    "Fix bug",
		AuthorID: "u1",
		Status:   models.StatusOpen,
	}
	svc := &fakePRService{
		createFn: func(ctx context.Context, req *models.PRCreateRequest) (*models.PullRequest, error) {
			if req.ID != "123" {
				t.Fatalf("expected ID 123, got %s", req.ID)
			}
			return want, nil
		},
	}
	rtr := newTestRouterWithPRService(svc)

	body := `{"pull_request_id":"123","pull_request_name":"Fix bug","author_id":"u1"}`
	req := httptest.NewRequest(http.MethodPost, "/pullRequest/create", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	rtr.createPR(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}
	var resp models.PRResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.PR.ID != want.ID {
		t.Fatalf("unexpected PR ID %s", resp.PR.ID)
	}
}

func TestCreatePR_BadJSON(t *testing.T) {
	svc := &fakePRService{
		createFn: func(context.Context, *models.PRCreateRequest) (*models.PullRequest, error) {
			t.Fatalf("service should not be called")
			return nil, nil
		},
	}
	rtr := newTestRouterWithPRService(svc)

	req := httptest.NewRequest(http.MethodPost, "/pullRequest/create", strings.NewReader("{bad json"))
	rec := httptest.NewRecorder()

	rtr.createPR(rec, req)

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

func TestCreatePR_ValidationError(t *testing.T) {
	valErr := fmt.Errorf("%w: pull_request_id is required", service.ErrPRValidation)
	svc := &fakePRService{
		createFn: func(context.Context, *models.PRCreateRequest) (*models.PullRequest, error) {
			return nil, valErr
		},
	}
	rtr := newTestRouterWithPRService(svc)

	req := httptest.NewRequest(http.MethodPost, "/pullRequest/create", bytes.NewBufferString(`{"pull_request_id":""}`))
	rec := httptest.NewRecorder()

	rtr.createPR(rec, req)

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
	if resp.Error.Message != valErr.Error() {
		t.Fatalf("unexpected message: %s", resp.Error.Message)
	}
}

func TestCreatePR_NotFoundErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{name: "author not found", err: service.ErrPRAuthorNotFound},
		{name: "team not found", err: service.ErrPRTeamNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakePRService{
				createFn: func(context.Context, *models.PRCreateRequest) (*models.PullRequest, error) {
					return nil, tc.err
				},
			}
			rtr := newTestRouterWithPRService(svc)

			req := httptest.NewRequest(http.MethodPost, "/pullRequest/create", bytes.NewBufferString(`{"pull_request_id":"1","pull_request_name":"a","author_id":"u1"}`))
			rec := httptest.NewRecorder()

			rtr.createPR(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("expected status 404, got %d", rec.Code)
			}
		})
	}
}

func TestCreatePR_AlreadyExists(t *testing.T) {
	svc := &fakePRService{
		createFn: func(context.Context, *models.PRCreateRequest) (*models.PullRequest, error) {
			return nil, service.ErrPRAlreadyExists
		},
	}
	rtr := newTestRouterWithPRService(svc)

	req := httptest.NewRequest(http.MethodPost, "/pullRequest/create", bytes.NewBufferString(`{"pull_request_id":"1","pull_request_name":"a","author_id":"u1"}`))
	rec := httptest.NewRecorder()

	rtr.createPR(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", rec.Code)
	}
	var resp models.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Error.Code != ErrCodePRExists {
		t.Fatalf("expected code %s, got %s", ErrCodePRExists, resp.Error.Code)
	}
	if resp.Error.Message != "pull request already exists" {
		t.Fatalf("unexpected message: %s", resp.Error.Message)
	}
}

func TestCreatePR_InternalError(t *testing.T) {
	svc := &fakePRService{
		createFn: func(context.Context, *models.PRCreateRequest) (*models.PullRequest, error) {
			return nil, errors.New("db down")
		},
	}
	rtr := newTestRouterWithPRService(svc)

	req := httptest.NewRequest(http.MethodPost, "/pullRequest/create", bytes.NewBufferString(`{"pull_request_id":"1","pull_request_name":"a","author_id":"u1"}`))
	rec := httptest.NewRecorder()

	rtr.createPR(rec, req)

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

func TestGetUserReviews_Success(t *testing.T) {
	want := &models.UserReviewsResponse{
		UserID: "u1",
		PullRequests: []*models.PullRequestShort{
			{ID: "pr1"},
		},
	}
	svc := &fakePRService{
		reviewsFn: func(ctx context.Context, userID string) (*models.UserReviewsResponse, error) {
			if userID != "u1" {
				t.Fatalf("expected user u1, got %s", userID)
			}
			return want, nil
		},
	}
	rtr := newTestRouterWithPRService(svc)

	req := httptest.NewRequest(http.MethodGet, "/users/getReview?user_id=u1", nil)
	rec := httptest.NewRecorder()

	rtr.getUserReviews(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var resp models.UserReviewsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.UserID != want.UserID {
		t.Fatalf("unexpected user id %s", resp.UserID)
	}
}

func TestGetUserReviews_ValidationError(t *testing.T) {
	valErr := fmt.Errorf("%w: user_id is required", service.ErrPRValidation)
	svc := &fakePRService{
		reviewsFn: func(context.Context, string) (*models.UserReviewsResponse, error) {
			return nil, valErr
		},
	}
	rtr := newTestRouterWithPRService(svc)

	req := httptest.NewRequest(http.MethodGet, "/users/getReview?user_id=", nil)
	rec := httptest.NewRecorder()

	rtr.getUserReviews(rec, req)

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
	if resp.Error.Message != valErr.Error() {
		t.Fatalf("unexpected message: %s", resp.Error.Message)
	}
}

func TestGetUserReviews_UserNotFound(t *testing.T) {
	svc := &fakePRService{
		reviewsFn: func(context.Context, string) (*models.UserReviewsResponse, error) {
			return nil, service.ErrUserNotFound
		},
	}
	rtr := newTestRouterWithPRService(svc)

	req := httptest.NewRequest(http.MethodGet, "/users/getReview?user_id=u1", nil)
	rec := httptest.NewRecorder()

	rtr.getUserReviews(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestGetUserReviews_InternalError(t *testing.T) {
	svc := &fakePRService{
		reviewsFn: func(context.Context, string) (*models.UserReviewsResponse, error) {
			return nil, errors.New("db down")
		},
	}
	rtr := newTestRouterWithPRService(svc)

	req := httptest.NewRequest(http.MethodGet, "/users/getReview?user_id=u1", nil)
	rec := httptest.NewRecorder()

	rtr.getUserReviews(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
}

func TestMergePR_Success(t *testing.T) {
	pr := &models.PullRequest{ID: "pr1"}
	svc := &fakePRService{
		mergeFn: func(ctx context.Context, req *models.PRMergeRequest) (*models.PullRequest, error) {
			if req.ID != "pr1" {
				t.Fatalf("expected pr1, got %s", req.ID)
			}
			return pr, nil
		},
	}
	rtr := newTestRouterWithPRService(svc)

	req := httptest.NewRequest(http.MethodPost, "/pullRequest/merge", bytes.NewBufferString(`{"pull_request_id":"pr1"}`))
	rec := httptest.NewRecorder()

	rtr.mergePR(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var resp models.PRResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.PR.ID != "pr1" {
		t.Fatalf("unexpected PR ID %s", resp.PR.ID)
	}
}

func TestMergePR_BadJSON(t *testing.T) {
	svc := &fakePRService{
		mergeFn: func(context.Context, *models.PRMergeRequest) (*models.PullRequest, error) {
			t.Fatalf("service should not be called")
			return nil, nil
		},
	}
	rtr := newTestRouterWithPRService(svc)

	req := httptest.NewRequest(http.MethodPost, "/pullRequest/merge", strings.NewReader("{bad json"))
	rec := httptest.NewRecorder()

	rtr.mergePR(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestMergePR_ValidationError(t *testing.T) {
	valErr := fmt.Errorf("%w: pull_request_id is required", service.ErrPRValidation)
	svc := &fakePRService{
		mergeFn: func(context.Context, *models.PRMergeRequest) (*models.PullRequest, error) {
			return nil, valErr
		},
	}
	rtr := newTestRouterWithPRService(svc)

	req := httptest.NewRequest(http.MethodPost, "/pullRequest/merge", bytes.NewBufferString(`{"pull_request_id":""}`))
	rec := httptest.NewRecorder()

	rtr.mergePR(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestMergePR_NotFound(t *testing.T) {
	svc := &fakePRService{
		mergeFn: func(context.Context, *models.PRMergeRequest) (*models.PullRequest, error) {
			return nil, service.ErrPRNotFound
		},
	}
	rtr := newTestRouterWithPRService(svc)

	req := httptest.NewRequest(http.MethodPost, "/pullRequest/merge", bytes.NewBufferString(`{"pull_request_id":"missing"}`))
	rec := httptest.NewRecorder()

	rtr.mergePR(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestMergePR_InternalError(t *testing.T) {
	svc := &fakePRService{
		mergeFn: func(context.Context, *models.PRMergeRequest) (*models.PullRequest, error) {
			return nil, errors.New("db down")
		},
	}
	rtr := newTestRouterWithPRService(svc)

	req := httptest.NewRequest(http.MethodPost, "/pullRequest/merge", bytes.NewBufferString(`{"pull_request_id":"pr1"}`))
	rec := httptest.NewRecorder()

	rtr.mergePR(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
}

func TestReassignPR_Success(t *testing.T) {
	resp := &models.PRReassignResponse{
		PR:         models.PullRequest{ID: "pr1"},
		ReplacedBy: "u2",
	}
	svc := &fakePRService{
		reassignFn: func(ctx context.Context, req *models.PRReassignRequest) (*models.PRReassignResponse, error) {
			if req.ID != "pr1" || req.OldReviewerID != "u1" {
				t.Fatalf("unexpected request: %+v", req)
			}
			return resp, nil
		},
	}
	rtr := newTestRouterWithPRService(svc)

	req := httptest.NewRequest(http.MethodPost, "/pullRequest/reassign", bytes.NewBufferString(`{"pull_request_id":"pr1","old_user_id":"u1"}`))
	rec := httptest.NewRecorder()

	rtr.reassignPR(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var got models.PRReassignResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ReplacedBy != resp.ReplacedBy {
		t.Fatalf("unexpected replaced by %s", got.ReplacedBy)
	}
}

func TestReassignPR_BadJSON(t *testing.T) {
	svc := &fakePRService{
		reassignFn: func(context.Context, *models.PRReassignRequest) (*models.PRReassignResponse, error) {
			t.Fatalf("service should not be called")
			return nil, nil
		},
	}
	rtr := newTestRouterWithPRService(svc)

	req := httptest.NewRequest(http.MethodPost, "/pullRequest/reassign", strings.NewReader("{bad json"))
	rec := httptest.NewRecorder()

	rtr.reassignPR(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestReassignPR_ValidationError(t *testing.T) {
	valErr := fmt.Errorf("%w: pull_request_id is required", service.ErrPRValidation)
	svc := &fakePRService{
		reassignFn: func(context.Context, *models.PRReassignRequest) (*models.PRReassignResponse, error) {
			return nil, valErr
		},
	}
	rtr := newTestRouterWithPRService(svc)

	req := httptest.NewRequest(http.MethodPost, "/pullRequest/reassign", bytes.NewBufferString(`{"pull_request_id":""}`))
	rec := httptest.NewRecorder()

	rtr.reassignPR(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestReassignPR_NotFoundCases(t *testing.T) {
	errorsToTest := []error{
		service.ErrPRNotFound,
		service.ErrUserNotFound,
		service.ErrPRTeamNotFound,
	}
	for _, errCase := range errorsToTest {
		t.Run(errCase.Error(), func(t *testing.T) {
			svc := &fakePRService{
				reassignFn: func(context.Context, *models.PRReassignRequest) (*models.PRReassignResponse, error) {
					return nil, errCase
				},
			}
			rtr := newTestRouterWithPRService(svc)

			req := httptest.NewRequest(http.MethodPost, "/pullRequest/reassign", bytes.NewBufferString(`{"pull_request_id":"pr1","old_user_id":"u1"}`))
			rec := httptest.NewRecorder()

			rtr.reassignPR(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("expected status 404, got %d", rec.Code)
			}
		})
	}
}

func TestReassignPR_ConflictCases(t *testing.T) {
	tests := []struct {
		err     error
		code    int
		errCode string
		message string
	}{
		{err: service.ErrPRMerged, code: http.StatusConflict, errCode: ErrCodePRMerged, message: "cannot reassign on merged PR"},
		{err: service.ErrReviewerNotAssigned, code: http.StatusConflict, errCode: ErrCodeNotAssigned, message: "reviewer is not assigned to this PR"},
		{err: service.ErrNoReplacement, code: http.StatusConflict, errCode: ErrCodeNoCandidate, message: "no active replacement candidate in team"},
	}
	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			svc := &fakePRService{
				reassignFn: func(context.Context, *models.PRReassignRequest) (*models.PRReassignResponse, error) {
					return nil, tc.err
				},
			}
			rtr := newTestRouterWithPRService(svc)

			req := httptest.NewRequest(http.MethodPost, "/pullRequest/reassign", bytes.NewBufferString(`{"pull_request_id":"pr1","old_user_id":"u1"}`))
			rec := httptest.NewRecorder()

			rtr.reassignPR(rec, req)

			if rec.Code != tc.code {
				t.Fatalf("expected status %d, got %d", tc.code, rec.Code)
			}
			var resp models.ErrorResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if resp.Error.Code != tc.errCode {
				t.Fatalf("expected code %s, got %s", tc.errCode, resp.Error.Code)
			}
			if resp.Error.Message != tc.message {
				t.Fatalf("unexpected message: %s", resp.Error.Message)
			}
		})
	}
}

func TestReassignPR_InternalError(t *testing.T) {
	svc := &fakePRService{
		reassignFn: func(context.Context, *models.PRReassignRequest) (*models.PRReassignResponse, error) {
			return nil, errors.New("db down")
		},
	}
	rtr := newTestRouterWithPRService(svc)

	req := httptest.NewRequest(http.MethodPost, "/pullRequest/reassign", bytes.NewBufferString(`{"pull_request_id":"pr1","old_user_id":"u1"}`))
	rec := httptest.NewRecorder()

	rtr.reassignPR(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
}

func TestGetAssignmentsStats_Success(t *testing.T) {
	want := &models.AssignmentsStatsResponse{
		ByUser: []*models.UserAssignmentsStat{
			{UserID: "u1", Assignments: 2},
		},
		ByPR: []*models.PRAssignmentsStat{
			{PullRequestID: "pr1", Reviewers: 2},
		},
	}
	svc := &fakePRService{
		statsFn: func(context.Context) (*models.AssignmentsStatsResponse, error) {
			return want, nil
		},
	}
	rtr := newTestRouterWithPRService(svc)

	req := httptest.NewRequest(http.MethodGet, "/stats/assignments", nil)
	rec := httptest.NewRecorder()

	rtr.getAssignmentsStats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var resp models.AssignmentsStatsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.ByUser) != 1 || resp.ByUser[0].UserID != "u1" {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestGetAssignmentsStats_Error(t *testing.T) {
	svc := &fakePRService{
		statsFn: func(context.Context) (*models.AssignmentsStatsResponse, error) {
			return nil, errors.New("db error")
		},
	}
	rtr := newTestRouterWithPRService(svc)

	req := httptest.NewRequest(http.MethodGet, "/stats/assignments", nil)
	rec := httptest.NewRecorder()

	rtr.getAssignmentsStats(rec, req)

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
}
