package models

type Task struct {
	Name        string                 `json:"name" binding:"required"`
	Runner      string                 `json:"runner" binding:"required"`
	Command     string                 `json:"command" binding:"required"`
	Synchronous bool                   `json:"synchronous"`
	Secret      map[string]string      `json:"secret,omitempty"`
	Schema      map[string]interface{} `json:"schema,omitempty"`
}
