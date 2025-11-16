package models

type PingResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
