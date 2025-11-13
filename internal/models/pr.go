package models

const (
	StatusOpen   = "OPEN"
	StatusMerged = "MERGED"
)

type PullRequest struct {
	ID                string   `json:"pull_request_id"`
	Title             string   `json:"pull_request_name"`
	AuthorID          string   `json:"author_id"`
	Status            string   `json:"status"`
	Reviewers         []string `json:"assigned_reviewers"`
	NeedMoreReviewers bool     `json:"need_more_reviewers"`
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
	UserID        string               `json:"user_id"`
	PullRequests []*PullRequestShort   `json:"pull_requests"`
}
