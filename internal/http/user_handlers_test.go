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

type fakeUserService struct {
	setFn func(ctx context.Context, userID string, isActive bool) (*models.UserWithTeam, error)
}

func (f *fakeUserService) SetUserActive(ctx context.Context, userID string, isActive bool) (*models.UserWithTeam, error) {
	if f.setFn == nil {
		return nil, errors.New("not implemented")
	}
	return f.setFn(ctx, userID, isActive)
}

func newTestRouterWithUserService(svc UserService) *router {
	return &router{
		userService: svc,
		log:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestSetUserActive_Success(t *testing.T) {
	wantUser := &models.UserWithTeam{
		User: models.User{
			ID:       "user-123",
			Username: "bob",
			IsActive: true,
		},
		TeamName: "backend",
	}
	called := false
	svc := &fakeUserService{
		setFn: func(ctx context.Context, userID string, isActive bool) (*models.UserWithTeam, error) {
			called = true
			if userID != "user-123" {
				t.Fatalf("expected userID user-123, got %s", userID)
			}
			if !isActive {
				t.Fatalf("expected isActive true")
			}
			return wantUser, nil
		},
	}
	rtr := newTestRouterWithUserService(svc)

	body := `{"user_id":"user-123","is_active":true}`
	req := httptest.NewRequest(http.MethodPost, "/users/setIsActive", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	rtr.setUserActive(rec, req)

	if !called {
		t.Fatalf("expected service to be called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var got models.UserWithTeam
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got != *wantUser {
		t.Fatalf("unexpected response: %+v", got)
	}
}

func TestSetUserActive_BadJSON(t *testing.T) {
	svc := &fakeUserService{
		setFn: func(context.Context, string, bool) (*models.UserWithTeam, error) {
			t.Fatalf("service should not be called")
			return nil, nil
		},
	}
	rtr := newTestRouterWithUserService(svc)

	req := httptest.NewRequest(http.MethodPost, "/users/setIsActive", strings.NewReader("{not json"))
	rec := httptest.NewRecorder()

	rtr.setUserActive(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
	var resp models.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if resp.Error.Code != ErrCodeBadRequest {
		t.Fatalf("expected code %s, got %s", ErrCodeBadRequest, resp.Error.Code)
	}
	if resp.Error.Message != "bad json request" {
		t.Fatalf("unexpected message: %s", resp.Error.Message)
	}
}

func TestSetUserActive_UserNotFound(t *testing.T) {
	svc := &fakeUserService{
		setFn: func(context.Context, string, bool) (*models.UserWithTeam, error) {
			return nil, service.ErrUserNotFound
		},
	}
	rtr := newTestRouterWithUserService(svc)

	req := httptest.NewRequest(http.MethodPost, "/users/setIsActive", bytes.NewBufferString(`{"user_id":"u1","is_active":false}`))
	rec := httptest.NewRecorder()

	rtr.setUserActive(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
	var resp models.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if resp.Error.Code != ErrCodeNotFound {
		t.Fatalf("expected code %s, got %s", ErrCodeNotFound, resp.Error.Code)
	}
	if resp.Error.Message != "resource not found" {
		t.Fatalf("unexpected message: %s", resp.Error.Message)
	}
}

func TestSetUserActive_ValidationError(t *testing.T) {
	errValidation := fmt.Errorf("%w: user_id is required", service.ErrUserValidation)
	svc := &fakeUserService{
		setFn: func(context.Context, string, bool) (*models.UserWithTeam, error) {
			return nil, errValidation
		},
	}
	rtr := newTestRouterWithUserService(svc)

	req := httptest.NewRequest(http.MethodPost, "/users/setIsActive", bytes.NewBufferString(`{"user_id":"","is_active":false}`))
	rec := httptest.NewRecorder()

	rtr.setUserActive(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
	var resp models.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if resp.Error.Code != ErrCodeBadRequest {
		t.Fatalf("expected code %s, got %s", ErrCodeBadRequest, resp.Error.Code)
	}
	if resp.Error.Message != errValidation.Error() {
		t.Fatalf("expected message %s, got %s", errValidation.Error(), resp.Error.Message)
	}
}

func TestSetUserActive_InternalError(t *testing.T) {
	internalErr := errors.New("db offline")
	svc := &fakeUserService{
		setFn: func(context.Context, string, bool) (*models.UserWithTeam, error) {
			return nil, internalErr
		},
	}
	rtr := newTestRouterWithUserService(svc)

	req := httptest.NewRequest(http.MethodPost, "/users/setIsActive", bytes.NewBufferString(`{"user_id":"u1","is_active":true}`))
	rec := httptest.NewRecorder()

	rtr.setUserActive(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
	var resp models.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if resp.Error.Code != ErrCodeInternal {
		t.Fatalf("expected code %s, got %s", ErrCodeInternal, resp.Error.Code)
	}
	if resp.Error.Message != internalErr.Error() {
		t.Fatalf("expected message %s, got %s", internalErr.Error(), resp.Error.Message)
	}
}
