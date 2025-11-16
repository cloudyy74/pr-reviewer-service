package models

import "time"

const (
	StatusOpen   = "OPEN"
	StatusMerged = "MERGED"
)

type PullRequest struct {
	ID        string     `json:"pull_request_id"`
	Title     string     `json:"pull_request_name"`
	AuthorID  string     `json:"author_id"`
	Status    string     `json:"status"`
	Reviewers []string   `json:"assigned_reviewers"`
	MergedAt  *time.Time `json:"mergedAt,omitempty"`
}

type PullRequestShort struct {
	ID       string `json:"pull_request_id"`
	Title    string `json:"pull_request_name"`
	AuthorID string `json:"author_id"`
	Status   string `json:"status"`
}

type PRCreateRequest struct {
	ID       string `json:"pull_request_id"`
	Title    string `json:"pull_request_name"`
	AuthorID string `json:"author_id"`
}

type PRResponse struct {
	PR PullRequest `json:"pr"`
}

type UserReviewsResponse struct {
	UserID       string              `json:"user_id"`
	PullRequests []*PullRequestShort `json:"pull_requests"`
}

type PRMergeRequest struct {
	ID string `json:"pull_request_id"`
}

type PRReassignRequest struct {
	ID            string `json:"pull_request_id"`
	OldReviewerID string `json:"old_reviewer_id"`
}

type PRReassignResponse struct {
	PR         PullRequest `json:"pr"`
	ReplacedBy string      `json:"replaced_by"`
}

type UserAssignmentsStat struct {
	UserID      string `json:"user_id"`
	Assignments int    `json:"assignments_count"`
}

type PRAssignmentsStat struct {
	PullRequestID string `json:"pull_request_id"`
	Reviewers     int    `json:"reviewers_count"`
}

type AssignmentsStatsResponse struct {
	ByUser []*UserAssignmentsStat `json:"assignments_by_user"`
	ByPR   []*PRAssignmentsStat   `json:"assignments_by_pr"`
}
