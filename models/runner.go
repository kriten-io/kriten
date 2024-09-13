package models

type Runner struct {
	Secret map[string]string `json:"secret,omitempty"`
	Name   string            `json:"name" binding:"required"`
	Image  string            `json:"image" binding:"required"`
	GitURL string            `json:"gitURL" binding:"required"`
	Token  string            `json:"token"`
	Branch string            `json:"branch"`
}
