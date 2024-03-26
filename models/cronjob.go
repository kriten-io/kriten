package models

type CronJob struct {
	ID        string                 `json:"id"`
	Owner     string                 `json:"owner"`
	Task      string                 `json:"task"`
	Schedule  string                 `json:"schedule"`
	Disable   bool                   `json:"disable"`
	ExtraVars map[string]interface{} `json:"extra_vars,omitempty"`
}
