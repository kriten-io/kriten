package models

type Task struct {
	Schema      map[string]any `json:"schema,omitempty"`
	Name        string         `json:"name" binding:"required"`
	Runner      string         `json:"runner" binding:"required"`
	Command     string         `json:"command" binding:"required"`
	Synchronous bool           `json:"synchronous"`
}

type TaskQueryParams struct {
	Limit  int    `form:"limit" binding:"omitempty,min=0"`
	Offset int    `form:"offset" binding:"omitempty,min=0"`
	Name   string `form:"name" binding:"omitempty"`
}
