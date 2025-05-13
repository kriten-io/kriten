package models

type Job struct {
	ID             string                 `json:"id"`
	Owner          string                 `json:"owner"`
	StartTime      string                 `json:"start_time,omitempty"`
	CompletionTime string                 `json:"completion_time,omitempty"`
	Failed         int32                  `json:"failed"`
	Completed      int32                  `json:"completed"`
	Stdout         string                 `json:"stdout"`
	JsonData       map[string]interface{} `json:"json_data"`
}
