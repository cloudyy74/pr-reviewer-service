package models

type Team struct {
	Name    string  `json:"team_name"`
	Members []*User `json:"members"`
}

type User struct {
	ID       string `json:"user_id"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}

type TeamResponse struct {
	Team Team `json:"team"`
}
