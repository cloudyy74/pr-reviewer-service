package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/cloudyy74/pr-reviewer-service/internal/models"
)

type router struct {
	teamService TeamService
	userService UserService
	prService   PRService
	log         *slog.Logger
}

func SetupRouter(
	mux *http.ServeMux,
	port string,
	teamService TeamService,
	userService UserService,
	prService PRService,
	log *slog.Logger,
) error {
	if port == "" {
		return errors.New("port cannot be empty")
	}
	if mux == nil {
		return errors.New("mux cannot be nil")
	}
	if teamService == nil {
		return errors.New("team service cannot be nil")
	}
	if userService == nil {
		return errors.New("user service cannot be nil")
	}
	if prService == nil {
		return errors.New("pr service cannot be nil")
	}
	if log == nil {
		return errors.New("logger cannot be nil")
	}
	r := router{
		teamService: teamService,
		userService: userService,
		prService:   prService,
		log:         log,
	}
	mux.HandleFunc("POST /team/add", r.createTeam)
	mux.HandleFunc("GET /team/get", r.getTeam)
	mux.HandleFunc("POST /users/setIsActive", r.setUserActive)
	mux.HandleFunc("GET /users/getReview", r.getUserReviews)
	mux.HandleFunc("POST /pullRequest/create", r.createPR)
	mux.HandleFunc("POST /pullRequest/merge", r.mergePR)
	mux.HandleFunc("POST /pullRequest/reassign", r.reassignPR)
	return nil
}

func (rtr *router) responseError(w http.ResponseWriter, statusCode int, errorCode string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	response := &models.ErrorResponse{
		Error: models.Error{
			Code:    errorCode,
			Message: message,
		},
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		rtr.log.Error("failed to encode response", slog.Any("error", err))
	}
}

func (rtr *router) responseJSON(w http.ResponseWriter, statusCode int, response any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		rtr.log.Error("failed to encode response", slog.Any("error", err))
	}
}
