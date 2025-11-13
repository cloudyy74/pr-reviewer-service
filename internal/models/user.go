package models

type User struct {
	ID       string `json:"user_id"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}

type UserWithTeam struct {
	User
	TeamName string `json:"team_name"`
}

type SetActiveRequest struct {
	ID       string `json:"user_id"`
	IsActive bool   `json:"is_active"`
}
