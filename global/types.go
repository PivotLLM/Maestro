/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package global

import "time"

// Project represents a project with its metadata
type Project struct {
	UUID               string                `json:"uuid"`
	Name               string                `json:"name"`
	Title              string                `json:"title"`
	Description        string                `json:"description,omitempty"`
	Context            string                `json:"context,omitempty"` // Global context included in all task prompts
	Status             string                `json:"status"`            // pending, in_progress, done, cancelled
	CreatedAt          time.Time             `json:"created_at"`
	UpdatedAt          time.Time             `json:"updated_at"`
	DefaultTemplates   *DefaultTemplates     `json:"default_templates,omitempty"`
	ReportPrefix       string                `json:"report_prefix,omitempty"`       // Active report session prefix (e.g., "20251219-1234-ISO-Audit-")
	ReportStartedAt    *time.Time            `json:"report_started_at,omitempty"`   // When report session started
	ReportTitle        string                `json:"report_title,omitempty"`        // Report title for L1 header
	ReportIntro        string                `json:"report_intro,omitempty"`        // Optional intro paragraph after title
	ReportDate         string                `json:"report_date,omitempty"`         // Report date (YYYY-MM-DD) captured at session start
	DisclaimerTemplate string                `json:"disclaimer_template,omitempty"` // Path to disclaimer MD file (e.g., "playbook/templates/disclaimer.md")
	ReportManifest     []ReportManifestEntry `json:"report_manifest,omitempty"`     // Ordered list of tasksets contributing to report
	ReportSequence     int                   `json:"report_sequence,omitempty"`     // Counter for manifest ordering
}

// ReportManifestEntry represents a taskset's contribution to the report
type ReportManifestEntry struct {
	Path     string `json:"path"`     // Taskset path (e.g., "assessment")
	Sequence int    `json:"sequence"` // Order in report (lower = earlier)
}

// DefaultTemplates holds project-level default templates for task sets
type DefaultTemplates struct {
	WorkerResponseTemplate string `json:"worker_response_template,omitempty"`
	WorkerReportTemplate   string `json:"worker_report_template,omitempty"`
	QAResponseTemplate     string `json:"qa_response_template,omitempty"`
	QAReportTemplate       string `json:"qa_report_template,omitempty"`
}

// ReportTemplateConfig defines a single report template in a multi-report manifest.
// When a template path ends in .json, it's parsed as []ReportTemplateConfig.
// When it ends in .md, it's treated as a single template with suffix "Report".
type ReportTemplateConfig struct {
	Suffix string `json:"suffix"` // Report suffix (e.g., "Report", "Internal", "Summary")
	File   string `json:"file"`   // Template file path relative to manifest location
}

// Limits controls execution limits for tasks
// MaxRetries: Infrastructure retries (network failures, command timeouts) - no LLM cost
// MaxWorker: Maximum worker LLM invocations per task (billable)
// MaxQA: Maximum QA iterations per task (billable)
type Limits struct {
	MaxRetries int `json:"max_retries,omitempty"`
	MaxWorker  int `json:"max_worker,omitempty"`
	MaxQA      int `json:"max_qa,omitempty"`
}

// WithDefaults returns a copy of Limits with defaults applied for zero values
func (l Limits) WithDefaults() Limits {
	result := l
	if result.MaxRetries == 0 {
		result.MaxRetries = DefaultMaxRetries
	}
	if result.MaxWorker == 0 {
		result.MaxWorker = DefaultMaxWorker
	}
	if result.MaxQA == 0 {
		result.MaxQA = DefaultMaxQA
	}
	return result
}

// TaskSet represents a collection of tasks at a specific path
type TaskSet struct {
	Path                   string    `json:"path"`
	Title                  string    `json:"title"`
	Description            string    `json:"description,omitempty"`
	WorkerResponseTemplate string    `json:"worker_response_template,omitempty"`
	WorkerReportTemplate   string    `json:"worker_report_template,omitempty"`
	QAResponseTemplate     string    `json:"qa_response_template,omitempty"`
	QAReportTemplate       string    `json:"qa_report_template,omitempty"`
	Parallel               bool      `json:"parallel"`
	Limits                 Limits    `json:"limits,omitempty"` // Execution limits for tasks in this set
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
	Tasks                  []Task    `json:"tasks"`
}

// Task represents a unit of work within a task set
// Note: Results and history are stored in results/<uuid>.json files, not in tasks.json
type Task struct {
	ID        int           `json:"id"`
	UUID      string        `json:"uuid"`
	Title     string        `json:"title"`
	Type      string        `json:"type,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	Work      WorkExecution `json:"work"`
	QA        QAExecution   `json:"qa"`
}

// Message represents a single message in the task execution history
// This is a complete transaction record containing prompt + response/error
type Message struct {
	// Metadata
	Timestamp  time.Time `json:"timestamp"`
	Role       string    `json:"role"`                   // "worker", "qa", "system"
	Invocation int       `json:"invocation,omitempty"`   // Which invocation number
	LLMModelID string    `json:"llm_model_id,omitempty"` // Which LLM was used

	// Request
	Prompt string `json:"prompt,omitempty"` // Full prompt sent to LLM (for prompt messages)

	// Response - present when LLM was invoked (any exit code)
	// Note: No omitempty on these fields - we want to see them even when empty for debugging
	ExitCode     *int   `json:"exit_code,omitempty"` // Command exit code (nil if not applicable, omitempty so prompts don't show it)
	Stdout       string `json:"stdout"`              // Raw stdout from LLM
	Stderr       string `json:"stderr"`              // Raw stderr from LLM
	ResponseSize int    `json:"response_size"`       // Size of stdout

	// Infrastructure error - present when command couldn't execute
	Error string `json:"error,omitempty"` // Infrastructure error message

	// Legacy fields (for backwards compatibility with existing result files)
	Type    string `json:"type,omitempty"`    // "prompt", "response", "error", "validation" (deprecated)
	Content string `json:"content,omitempty"` // The actual message content (deprecated - use Prompt/Stdout)
}

// WorkExecution tracks the work phase of task execution
// Note: Full result is stored in results/<uuid>.json, not here
type WorkExecution struct {
	InstructionsFile       string     `json:"instructions_file,omitempty"`
	InstructionsFileSource string     `json:"instructions_file_source,omitempty"`
	InstructionsText       string     `json:"instructions_text,omitempty"`
	Prompt                 string     `json:"prompt,omitempty"`
	LLMModelID             string     `json:"llm_model_id,omitempty"`
	Status                 string     `json:"status"`
	Error                  string     `json:"error,omitempty"`
	Invocations            int        `json:"invocations"`               // Number of worker LLM invocations (any exit code)
	InfraRetries           int        `json:"infra_retries,omitempty"`   // Infrastructure failures (couldn't execute)
	LastAttemptAt          *time.Time `json:"last_attempt_at,omitempty"` // For retry delay calculation
}

// QAExecution tracks the QA phase of task execution
// Note: Full result is stored in results/<uuid>.json, not here
type QAExecution struct {
	Enabled                bool   `json:"enabled"`
	InstructionsFile       string `json:"instructions_file,omitempty"`
	InstructionsFileSource string `json:"instructions_file_source,omitempty"`
	InstructionsText       string `json:"instructions_text,omitempty"`
	Prompt                 string `json:"prompt,omitempty"`
	LLMModelID             string `json:"llm_model_id,omitempty"`
	Status                 string `json:"status,omitempty"`
	Error                  string `json:"error,omitempty"`         // Error message if status is "error"
	Verdict                string `json:"verdict,omitempty"`       // QA verdict: "pass", "fail", "escalate"
	Invocations            int    `json:"invocations,omitempty"`   // Number of QA LLM invocations (any exit code)
	InfraRetries           int    `json:"infra_retries,omitempty"` // Infrastructure failures (couldn't execute)
}

// ListRef references an item within a list file
type ListRef struct {
	Role     string `json:"role"`      // Purpose of this reference (e.g., "source", "target", "reference")
	ListFile string `json:"list_file"` // Path to the list file
	ItemID   string `json:"item_id"`   // ID of the item within the list
}

// TaskResult represents the complete audit record for a completed task
// Stored in results/<uuid>.json
type TaskResult struct {
	// Identity
	TaskID    int    `json:"task_id"`
	TaskUUID  string `json:"task_uuid"`
	TaskTitle string `json:"task_title"`
	TaskType  string `json:"task_type,omitempty"`

	// Timing
	CreatedAt   time.Time `json:"created_at"`
	CompletedAt time.Time `json:"completed_at"`

	// Worker execution (complete audit)
	Worker WorkerResult `json:"worker"`

	// QA execution (omitted if QA not enabled)
	QA *QAResult `json:"qa,omitempty"`

	// Complete message history
	History []Message `json:"history,omitempty"`

	// Supervisor override - when true, supervisor has provided the response
	// and this task should not be sent to a worker again (except on reset)
	SupervisorOverride bool `json:"supervisor_override"`
}

// WorkerResult contains the complete audit trail for worker execution
type WorkerResult struct {
	// Snapshot of what was configured (for audit - may differ from current task if edited)
	InstructionsFile       string `json:"instructions_file,omitempty"`
	InstructionsFileSource string `json:"instructions_file_source,omitempty"`
	InstructionsText       string `json:"instructions_text,omitempty"`
	TaskPrompt             string `json:"task_prompt,omitempty"`

	// What was actually sent/received
	FullPrompt  string `json:"full_prompt"` // Complete constructed prompt sent to LLM
	Response    string `json:"response"`    // Full LLM response
	LLMModelID  string `json:"llm_model_id"`
	Invocations int    `json:"invocations"`
	Status      string `json:"status"` // done/failed
	Error       string `json:"error,omitempty"`
}

// QAResult contains the complete audit trail for QA execution
type QAResult struct {
	// Snapshot of what was configured
	InstructionsFile       string `json:"instructions_file,omitempty"`
	InstructionsFileSource string `json:"instructions_file_source,omitempty"`
	InstructionsText       string `json:"instructions_text,omitempty"`
	TaskPrompt             string `json:"task_prompt,omitempty"`

	// What was actually sent/received
	FullPrompt  string `json:"full_prompt"` // Complete QA prompt sent to LLM
	Response    string `json:"response"`    // Full QA LLM response
	Verdict     string `json:"verdict"`     // pass/fail/escalate
	LLMModelID  string `json:"llm_model_id"`
	Invocations int    `json:"invocations"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
}

// RunRequest represents a request to run tasks via the runner
type RunRequest struct {
	Project  string `json:"project"`
	Path     string `json:"path,omitempty"`
	Type     string `json:"type,omitempty"`    // Filter by task type
	Parallel *bool  `json:"parallel"`          // Override taskset parallel setting (nil = use taskset setting)
	Timeout  int    `json:"timeout,omitempty"` // LLM call timeout in seconds (min: 60, max: 900, default: 300)
	Wait     bool   `json:"wait,omitempty"`    // Wait for all tasks to complete before returning
}

// RunResult represents the result of a runner execution
type RunResult struct {
	Project        string `json:"project"`
	Path           string `json:"path,omitempty"`
	TasksFound     int    `json:"tasks_found"`
	TasksExecuted  int    `json:"tasks_executed"`
	TasksSucceeded int    `json:"tasks_succeeded"`
	TasksFailed    int    `json:"tasks_failed"`
	TasksSkipped   int    `json:"tasks_skipped"` // Max attempts reached or retry delay not elapsed
	Message        string `json:"message,omitempty"`
}

// ResultsRequest represents a request to get task results
type ResultsRequest struct {
	Project       string `json:"project"`
	Path          string `json:"path,omitempty"`
	TaskID        *int   `json:"task_id,omitempty"` // If provided, return single task result
	Offset        int    `json:"offset,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	Status        string `json:"status,omitempty"`         // Filter by status
	Summary       bool   `json:"summary,omitempty"`        // If true, return only task_id, title, work_status
	WorkerPattern string `json:"worker_pattern,omitempty"` // Regex pattern to match against worker response
	QAPattern     string `json:"qa_pattern,omitempty"`     // Regex pattern to match against QA response
}

// ResultsResponse represents the response for aggregated results
type ResultsResponse struct {
	Project       string              `json:"project"`
	Path          string              `json:"path,omitempty"`
	TotalCount    int                 `json:"total_count"`
	ReturnedCount int                 `json:"returned_count"`
	Offset        int                 `json:"offset"`
	Results       []TaskResult        `json:"results"`             // Full results (when summary=false)
	Summaries     []TaskResultSummary `json:"summaries,omitempty"` // Summary results (when summary=true)
}

// TaskResultSummary represents a minimal task result with only Maestro core fields
type TaskResultSummary struct {
	TaskID     int    `json:"task_id"`
	TaskUUID   string `json:"task_uuid"`
	TaskTitle  string `json:"task_title"`
	WorkStatus string `json:"work_status"`
}

// SingleResultResponse represents the response for a single task result
type SingleResultResponse struct {
	TaskID      int       `json:"task_id"`
	TaskTitle   string    `json:"task_title"`
	Status      string    `json:"status"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	Result      string    `json:"result,omitempty"`
	Error       string    `json:"error,omitempty"`
}

// TaskResultGetResponse represents the response for task_result_get
// Contains only the essential finding information without history or prompts
type TaskResultGetResponse struct {
	// Task identity
	TaskID    int    `json:"task_id"`
	TaskUUID  string `json:"task_uuid"`
	TaskTitle string `json:"task_title"`
	TaskType  string `json:"task_type,omitempty"`
	TaskPath  string `json:"task_path"`

	// Template info for supervisor updates
	WorkerResponseTemplate string `json:"worker_response_template,omitempty"`
	WorkerResponseSchema   string `json:"worker_response_schema,omitempty"` // Actual schema content for supervisor updates

	// Worker result
	WorkerStatus   string `json:"worker_status"`
	WorkerResponse string `json:"worker_response"`
	WorkerError    string `json:"worker_error,omitempty"`

	// QA result (if enabled)
	QAEnabled  bool   `json:"qa_enabled"`
	QAStatus   string `json:"qa_status,omitempty"`
	QAVerdict  string `json:"qa_verdict,omitempty"`
	QAResponse string `json:"qa_response,omitempty"`
	QAError    string `json:"qa_error,omitempty"`

	// Supervisor info
	SupervisorOverride bool `json:"supervisor_override"`

	// Timing
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

// List represents a structured list file
type List struct {
	Version     string            `json:"version"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Templates   *DefaultTemplates `json:"templates,omitempty"` // List-level templates for task creation
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Items       []ListItem        `json:"items"`
}

// ListItem represents a single item in a list
type ListItem struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Content   string   `json:"content"`
	SourceDoc string   `json:"source_doc,omitempty"`
	Section   string   `json:"section,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Complete  bool     `json:"complete"`
}

// ListSummary represents metadata about a list (for list_list responses)
type ListSummary struct {
	Filename    string    `json:"filename"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	ItemCount   int       `json:"item_count"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ListItemSummary represents a truncated item (for list_get_summary responses)
type ListItemSummary struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Content  string `json:"content"` // Truncated to 100 chars
	Complete bool   `json:"complete"`
}

// ListListResponse represents the response for list_list
type ListListResponse struct {
	Lists         []ListSummary `json:"lists"`
	TotalCount    int           `json:"total_count"`
	ReturnedCount int           `json:"returned_count"`
	Offset        int           `json:"offset"`
}

// ListGetSummaryResponse represents the response for list_get_summary
type ListGetSummaryResponse struct {
	Name          string            `json:"name"`
	Description   string            `json:"description,omitempty"`
	ItemCount     int               `json:"item_count"`
	Items         []ListItemSummary `json:"items"`
	ReturnedCount int               `json:"returned_count"`
	Offset        int               `json:"offset"`
}

// ListItemSearchResponse represents the response for list_item_search
type ListItemSearchResponse struct {
	Items         []ListItem `json:"items"`
	TotalCount    int        `json:"total_count"`
	ReturnedCount int        `json:"returned_count"`
	Offset        int        `json:"offset"`
}

// ListCreateTasksResponse represents the response for list_create_tasks
type ListCreateTasksResponse struct {
	TasksCreated int    `json:"tasks_created"`
	ListName     string `json:"list_name"`
	ItemCount    int    `json:"item_count"`
	TaskIDs      []int  `json:"task_ids"`
}
