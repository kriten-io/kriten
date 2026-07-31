package models

type Runner struct {
	Secret map[string]string `json:"secret,omitempty"`
	Name   string            `json:"name" binding:"required"`
	Image  string            `json:"image" binding:"required"`
	GitURL string            `json:"gitURL" binding:"required"`
	Token  string            `json:"token,omitempty"`
	Branch string            `json:"branch"`
}

type RunnerQueryParams struct {
	Limit  int    `form:"limit" binding:"omitempty,min=0"`
	Offset int    `form:"offset" binding:"omitempty,min=0"`
	Name   string `form:"name" binding:"omitempty"`
}
