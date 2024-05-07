package models

type CronJob struct {
	Name      string                 `json:"name"`
	Owner     string                 `json:"owner"`
	Task      string                 `json:"task"`
	Schedule  string                 `json:"schedule"`
	Disable   bool                   `json:"disable"`
	ExtraVars map[string]interface{} `json:"extra_vars,omitempty"`
}
