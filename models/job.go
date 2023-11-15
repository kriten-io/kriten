package models

type Job struct {
	ID             string                 `json:"id"`
	Owner          string                 `json:"owner"`
	StartTime      string                 `json:"startTime,omitempty"`
	CompletionTime string                 `json:"completionTime,omitempty"`
	Failed         int32                  `json:"failed"`
	Completed      int32                  `json:"completed"`
	Stdout         string                 `json:"stdout"`
	JsonData       map[string]interface{} `json:"json_data"`
}
