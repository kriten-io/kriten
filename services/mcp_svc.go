package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/kriten-io/kriten/models"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	uuid "github.com/satori/go.uuid"
)

const (
	mcpStdoutLimit    = 64 * 1024
	mcpMaxWaitSeconds = 30
	mcpPollInterval   = 2 * time.Second
)

type callerContextKey struct{}

// WithCaller stores the authenticated caller on the request context. The MCP
// server is built per request, so tool handlers act as this caller.
func WithCaller(ctx context.Context, meta models.AuditMetadata) context.Context {
	return context.WithValue(ctx, callerContextKey{}, meta)
}

func callerFromContext(ctx context.Context) (models.AuditMetadata, bool) {
	meta, ok := ctx.Value(callerContextKey{}).(models.AuditMetadata)
	return meta, ok
}

type McpService interface {
	Handler() http.Handler
}

type McpServiceImpl struct {
	taskService TaskService
	jobService  JobService
	authService AuthService
	handler     http.Handler
}

// AgentTask is the subset of a Task exposed to MCP agents. Commands, runner
// details and secrets are deliberately left out.
type AgentTask struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Schema      map[string]any `json:"schema"`
	Synchronous bool           `json:"synchronous"`
}

type AgentTaskList struct {
	Tasks []AgentTask `json:"tasks"`
}

type AgentTaskIDInput struct {
	ID string `json:"id" jsonschema:"the task id"`
}

type AgentRunTaskInput struct {
	ID        string         `json:"id" jsonschema:"the task id"`
	ExtraVars map[string]any `json:"extra_vars,omitempty" jsonschema:"extra vars as a JSON object matching the task schema"`
}

type AgentGetJobInput struct {
	ID          string `json:"id" jsonschema:"the job id returned by run_task"`
	WaitSeconds int    `json:"wait_seconds,omitempty" jsonschema:"seconds to wait for a running job to finish, between 0 and 30 (default 0)"`
}

type AgentJobResult struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	TaskName        string         `json:"task_name"`
	Status          string         `json:"status"`
	Completed       int32          `json:"completed"`
	Failed          int32          `json:"failed"`
	FailReason      string         `json:"failed_reason,omitempty"`
	StartTime       string         `json:"start_time,omitempty"`
	CompletionTime  string         `json:"completion_time,omitempty"`
	Stdout          string         `json:"stdout,omitempty"`
	StdoutTruncated bool           `json:"stdout_truncated,omitempty"`
	JsonData        map[string]any `json:"json_data,omitempty"`
}

const mcpServerName = "kriten"

func NewMcpService(ts TaskService, js JobService, as AuthService) McpService {
	s := &McpServiceImpl{
		taskService: ts,
		jobService:  js,
		authService: as,
	}

	s.handler = mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server { return s.newServer(r.Context()) },
		&mcp.StreamableHTTPOptions{
			Stateless:    true,
			JSONResponse: true,
		},
	)

	return s
}

func (m *McpServiceImpl) Handler() http.Handler {
	return m.handler
}

func (m *McpServiceImpl) newServer(ctx context.Context) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: mcpServerName, Version: "v1.0.0"}, &mcp.ServerOptions{
		Instructions: "Discover tasks with list_tasks and get_task. Each task's schema describes the extra vars to pass to run_task. Only tasks with a schema are available. run_task returns a job id; call get_job with that id to read the result.",
	})

	caller, authenticated := callerFromContext(ctx)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_tasks",
		Description: "List tasks available to you that have a schema, including their descriptions and input schemas.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, AgentTaskList, error) {
		if !authenticated {
			return nil, AgentTaskList{}, ErrSvcForbidden
		}
		tasks, err := m.listTasks(caller)
		if err != nil {
			return nil, AgentTaskList{}, err
		}
		return nil, AgentTaskList{Tasks: tasks}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_task",
		Description: "Get one task with its description and input schema. Use the schema to build extra_vars for run_task.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in AgentTaskIDInput) (*mcp.CallToolResult, AgentTask, error) {
		if !authenticated {
			return nil, AgentTask{}, ErrSvcForbidden
		}
		task, err := m.getTask(caller, in.ID)
		if err != nil {
			return nil, AgentTask{}, err
		}
		return nil, task, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "run_task",
		Description: "Run a task with extra_vars that must match the task's input schema. Returns the created job id and status. Use get_job with the id to read the result. Synchronous tasks wait for completion before returning.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in AgentRunTaskInput) (*mcp.CallToolResult, AgentJobResult, error) {
		if !authenticated {
			return nil, AgentJobResult{}, ErrSvcForbidden
		}
		job, err := m.runTask(caller, in.ID, in.ExtraVars)
		if err != nil {
			return nil, AgentJobResult{}, err
		}
		return nil, job, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_job",
		Description: "Get the status and result of a job started by run_task: status, completed and failed counts, failure reason, stdout and json_data. Pass wait_seconds (up to 30) to wait for a running job to finish.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in AgentGetJobInput) (*mcp.CallToolResult, AgentJobResult, error) {
		if !authenticated {
			return nil, AgentJobResult{}, ErrSvcForbidden
		}
		job, err := m.getJob(ctx, caller, in.ID, in.WaitSeconds)
		if err != nil {
			return nil, AgentJobResult{}, err
		}
		return nil, job, nil
	})

	return server
}

// listTasks returns the caller's tasks that have a schema. Visibility follows
// the same RBAC list as GET /tasks.
func (m *McpServiceImpl) listTasks(caller models.AuditMetadata) ([]AgentTask, error) {
	authList, err := m.authService.GetAuthorizationList(&models.Authorization{
		UserID:   caller.UserID,
		Provider: caller.Provider,
		Resource: "tasks",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get task authorisations: %w", err)
	}

	tasks, _, err := m.taskService.ListTasks(authList, models.TaskQueryParams{})
	if err != nil {
		return nil, err
	}

	result := []AgentTask{}
	for _, task := range tasks {
		if len(task.Schema) == 0 {
			continue
		}
		result = append(result, toAgentTask(task))
	}
	return result, nil
}

func (m *McpServiceImpl) getTask(caller models.AuditMetadata, rawID string) (AgentTask, error) {
	id, err := uuid.FromString(rawID)
	if err != nil {
		return AgentTask{}, fmt.Errorf("invalid task id format")
	}

	if err := m.authorise(caller, id, "read"); err != nil {
		return AgentTask{}, err
	}

	task, err := m.taskService.GetTask(id)
	if err != nil {
		return AgentTask{}, err
	}
	if len(task.Schema) == 0 {
		return AgentTask{}, fmt.Errorf("task '%s': %w", task.Name, ErrSvcTaskNoSchema)
	}

	return toAgentTask(task), nil
}

func (m *McpServiceImpl) runTask(caller models.AuditMetadata, rawID string, extraVars map[string]any) (AgentJobResult, error) {
	id, err := uuid.FromString(rawID)
	if err != nil {
		return AgentJobResult{}, fmt.Errorf("invalid task id format")
	}

	if err := m.authorise(caller, id, "execute"); err != nil {
		return AgentJobResult{}, err
	}

	task, err := m.taskService.GetTask(id)
	if err != nil {
		return AgentJobResult{}, err
	}
	if len(task.Schema) == 0 {
		return AgentJobResult{}, fmt.Errorf("task '%s': %w", task.Name, ErrSvcTaskNoSchema)
	}

	if extraVars == nil {
		extraVars = map[string]any{}
	}
	payload, err := json.Marshal(extraVars)
	if err != nil {
		return AgentJobResult{}, fmt.Errorf("invalid extra vars: %w", err)
	}

	job, err := m.taskService.RunTask(caller, caller.UserName, id, string(payload))
	if err != nil {
		return AgentJobResult{}, err
	}

	return toAgentJobResult(job), nil
}

// getJob returns a job's current state. Running jobs are refreshed from the
// cluster, and when waitSeconds is set the call polls until the job finishes or
// the wait expires. Read access follows GET /jobs/:id: read or execute on the
// job's task.
func (m *McpServiceImpl) getJob(ctx context.Context, caller models.AuditMetadata, rawID string, waitSeconds int) (AgentJobResult, error) {
	if waitSeconds < 0 || waitSeconds > mcpMaxWaitSeconds {
		return AgentJobResult{}, fmt.Errorf("wait_seconds must be between 0 and %d", mcpMaxWaitSeconds)
	}

	id, err := uuid.FromString(rawID)
	if err != nil {
		return AgentJobResult{}, fmt.Errorf("invalid job id format")
	}

	job, err := m.jobService.GetJob(id)
	if err != nil {
		return AgentJobResult{}, err
	}

	allowed, err := m.authService.IsAutorised(&models.Authorization{
		UserID:       caller.UserID,
		Provider:     caller.Provider,
		Resource:     "jobs",
		ResourceName: job.TaskID.String(),
		Access:       "read",
	})
	if err != nil {
		return AgentJobResult{}, fmt.Errorf("job '%s' authorisation: %w", id, err)
	}
	if !allowed {
		return AgentJobResult{}, fmt.Errorf("job '%s': %w", id, ErrSvcForbidden)
	}

	deadline := time.Now().Add(time.Duration(waitSeconds) * time.Second)
	for job.Status == "running" {
		if err := m.jobService.RefreshJob(id); err != nil {
			log.Printf("mcp get_job: failed to refresh job '%s': %v", id, err)
		}
		job, err = m.jobService.GetJob(id)
		if err != nil {
			return AgentJobResult{}, err
		}
		if job.Status != "running" || !time.Now().Before(deadline) {
			break
		}

		select {
		case <-ctx.Done():
			return toAgentJobResult(job), nil
		case <-time.After(min(mcpPollInterval, time.Until(deadline))):
		}
	}

	return toAgentJobResult(job), nil
}

// authorise mirrors the REST route checks for a single task.
func (m *McpServiceImpl) authorise(caller models.AuditMetadata, id uuid.UUID, access string) error {
	allowed, err := m.authService.IsAutorised(&models.Authorization{
		UserID:       caller.UserID,
		Provider:     caller.Provider,
		Resource:     "tasks",
		ResourceName: id.String(),
		Access:       access,
	})
	if err != nil {
		return fmt.Errorf("task '%s' authorisation: %w", id, err)
	}
	if !allowed {
		return fmt.Errorf("task '%s': %w", id, ErrSvcForbidden)
	}
	return nil
}

func toAgentTask(task *models.Task) AgentTask {
	return AgentTask{
		ID:          task.ID.String(),
		Name:        task.Name,
		Description: task.Description,
		Schema:      task.Schema,
		Synchronous: task.Synchronous,
	}
}

func toAgentJobResult(job models.Job) AgentJobResult {
	stdout, truncated := capStdout(job.Stdout)
	return AgentJobResult{
		ID:              job.ID.String(),
		Name:            job.Name,
		TaskName:        job.TaskName,
		Status:          job.Status,
		Completed:       job.Completed,
		Failed:          job.Failed,
		FailReason:      job.FailReason,
		StartTime:       job.StartTime,
		CompletionTime:  job.CompletionTime,
		Stdout:          stdout,
		StdoutTruncated: truncated,
		JsonData:        job.JsonData,
	}
}

// capStdout trims output to mcpStdoutLimit bytes without splitting a UTF-8
// character.
func capStdout(s string) (string, bool) {
	if len(s) <= mcpStdoutLimit {
		return s, false
	}
	cut := mcpStdoutLimit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut], true
}
