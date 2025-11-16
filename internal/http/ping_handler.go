package http

import (
	"net/http"

	"github.com/cloudyy74/pr-reviewer-service/internal/models"
)

func (rtr *router) ping(w http.ResponseWriter, r *http.Request) {
	rtr.responseJSON(w, http.StatusOK, models.PingResponse{Status: "ok", Message: "pong"})
}
