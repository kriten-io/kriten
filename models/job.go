package models

type Job struct {
	Name           string                 `json:"name"`
	Owner          string                 `json:"owner"`
	StartTime      string                 `json:"start_time,omitempty"`
	CompletionTime string                 `json:"completion_time,omitempty"`
	Failed         int32                  `json:"failed"`
	Completed      int32                  `json:"completed"`
	Status         string                 `json:"status,omitempty"`
	FailReason     string                 `json:"failed_reason,omitempty"`
	Stdout         string                 `json:"stdout,omitempty"`
	JsonData       map[string]interface{} `json:"json_data,omitempty"`
}

type JobQueryParams struct {
	Limit  int    `form:"limit" binding:"omitempty,min=0"`
	Offset int    `form:"offset" binding:"omitempty,min=0"`
	Owner  string `form:"owner" binding:"omitempty"`
	Status string `form:"status" binding:"omitempty,oneof=running completed failed"`
	Name   string `form:"name" binding:"omitempty"`
}

type JobMessage struct {
	Message string `json:"message"`
	JobName string `json:"name"`
}
