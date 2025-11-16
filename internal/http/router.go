package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
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
	mux.HandleFunc("GET /ping", r.panicMiddleware(r.loggingMiddleware(r.ping)))
	mux.HandleFunc("POST /team/add", r.panicMiddleware(r.loggingMiddleware(r.createTeam)))
	mux.HandleFunc("GET /team/get", r.panicMiddleware(r.loggingMiddleware(r.getTeam)))
	mux.HandleFunc("POST /team/deactivate", r.panicMiddleware(r.loggingMiddleware(r.deactivateTeamUsers)))
	mux.HandleFunc("POST /users/setIsActive", r.panicMiddleware(r.loggingMiddleware(r.setUserActive)))
	mux.HandleFunc("GET /users/getReview", r.panicMiddleware(r.loggingMiddleware(r.getUserReviews)))
	mux.HandleFunc("POST /pullRequest/create", r.panicMiddleware(r.loggingMiddleware(r.createPR)))
	mux.HandleFunc("POST /pullRequest/merge", r.panicMiddleware(r.loggingMiddleware(r.mergePR)))
	mux.HandleFunc("POST /pullRequest/reassign", r.panicMiddleware(r.loggingMiddleware(r.reassignPR)))
	mux.HandleFunc("GET /stats/assignments", r.panicMiddleware(r.loggingMiddleware(r.getAssignmentsStats)))
	return nil
}

func (rtr *router) responseJSON(w http.ResponseWriter, statusCode int, response any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		rtr.log.Error("failed to encode response", slog.Any("error", err))
	}
}
