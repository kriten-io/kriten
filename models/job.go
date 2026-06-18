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

type JobQueryParams struct {
	Limit   int    `form:"limit" binding:"omitempty,min=0"`
	Offset  int    `form:"offset" binding:"omitempty,min=0"`
	Owner   string `form:"owner" binding:"omitempty"`
	Status  string `form:"status" binding:"omitempty,oneof=running completed failed"`
	JobName string `form:"job_name" binding:"omitempty"`
}
