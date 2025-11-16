package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/cloudyy74/pr-reviewer-service/internal/models"
	"github.com/cloudyy74/pr-reviewer-service/internal/service"
)

type ResponseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (re ResponseError) Error() string {
	return re.Message
}

func newResponseError(code string, msg string) ResponseError {
	return ResponseError{
		Code:    code,
		Message: msg,
	}
}

func newInternalError(msg string, args ...any) ResponseError {
	return newResponseError(ErrCodeInternal, fmt.Sprintf(msg, args...))
}

func (rtr *router) handleError(w http.ResponseWriter, err error) {
	respErr := rtr.mapError(err)
	status := statusForCode(respErr.Code)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(&models.ErrorResponse{
		Error: models.Error{
			Code:    respErr.Code,
			Message: respErr.Message,
		},
	})
}

func (rtr *router) mapError(err error) ResponseError {
	var respErr ResponseError
	if errors.As(err, &respErr) {
		return respErr
	}

	switch {
	case errors.Is(err, service.ErrTeamValidation), errors.Is(err, service.ErrPRValidation), errors.Is(err, service.ErrUserValidation):
		return newResponseError(ErrCodeValidation, err.Error())
	case errors.Is(err, service.ErrTeamExists):
		return newResponseError(ErrCodeTeamExists, "team_name already exists")
	case errors.Is(err, service.ErrTeamNotFound), errors.Is(err, service.ErrPRTeamNotFound),
		errors.Is(err, service.ErrPRAuthorNotFound), errors.Is(err, service.ErrPRNotFound),
		errors.Is(err, service.ErrUserNotFound):
		return newResponseError(ErrCodeNotFound, "resource not found")
	case errors.Is(err, service.ErrPRAlreadyExists):
		return newResponseError(ErrCodePRExists, "pull request already exists")
	case errors.Is(err, service.ErrPRMerged):
		return newResponseError(ErrCodePRMerged, "cannot reassign on merged PR")
	case errors.Is(err, service.ErrReviewerNotAssigned):
		return newResponseError(ErrCodeNotAssigned, "reviewer is not assigned to this PR")
	case errors.Is(err, service.ErrNoReplacement):
		return newResponseError(ErrCodeNoCandidate, "no active replacement candidate in team")
	default:
		return newInternalError("internal error")
	}
}

func statusForCode(code string) int {
	switch code {
	case ErrCodeBadRequest, ErrCodeValidation, ErrCodeTeamExists:
		return http.StatusBadRequest
	case ErrCodeNotFound:
		return http.StatusNotFound
	case ErrCodePRExists, ErrCodePRMerged, ErrCodeNotAssigned, ErrCodeNoCandidate:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
