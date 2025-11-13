package models

type Team struct {
	Name    string  `json:"team_name"`
	Members []*User `json:"members"`
}

type TeamResponse struct {
	Team Team `json:"team"`
}
