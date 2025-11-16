package models

type Team struct {
	Name    string  `json:"team_name"`
	Members []*User `json:"members"`
}

type TeamResponse struct {
	Team Team `json:"team"`
}

type TeamDeactivateRequest struct {
	TeamName string `json:"team_name"`
}

type TeamDeactivateResponse struct {
	TeamName         string `json:"team_name"`
	DeactivatedCount int    `json:"deactivated_count"`
}
