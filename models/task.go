package models

type Task struct {
	Schema      map[string]interface{} `json:"schema,omitempty"`
	Name        string                 `json:"name" binding:"required"`
	Runner      string                 `json:"runner" binding:"required"`
	Command     string                 `json:"command" binding:"required"`
	Synchronous bool                   `json:"synchronous"`
}
