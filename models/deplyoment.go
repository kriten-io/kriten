package models

type Deployment struct {
	Name      string                 `json:"name"`
	Owner     string                 `json:"owner"`
	Task      string                 `json:"task"`
	Replicas  int32                  `json:"replicas"`
	ExtraVars map[string]interface{} `json:"extra_vars,omitempty"`
}
