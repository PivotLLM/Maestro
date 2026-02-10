/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package global

import "fmt"

//goland:noinspection GoCommentStart,GoUnusedConst,GoUnusedConst,GoUnusedConst
const (
	// Configuration constants
	ConfigEnvVar          = "MAESTRO_CONFIG"
	DefaultBaseDir        = "~/.maestro"
	DefaultConfigFileName = "config.json"
	DefaultPlaybooksDir   = "playbooks"
	DefaultProjectsDir    = "projects"

	// Fixed category names
	CategoryReference = "reference"
	CategoryPlaybooks = "playbooks"
	CategoryProjects  = "projects"

	// MCP Tool Names - Reference (read-only, embedded)
	ToolReferenceList   = "reference_list"
	ToolReferenceGet    = "reference_get"
	ToolReferenceSearch = "reference_search"

	// MCP Tool Names - Playbook
	ToolPlaybookList       = "playbook_list"
	ToolPlaybookCreate     = "playbook_create"
	ToolPlaybookRename     = "playbook_rename"
	ToolPlaybookDelete     = "playbook_delete"
	ToolPlaybookFileList   = "playbook_file_list"
	ToolPlaybookFileGet    = "playbook_file_get"
	ToolPlaybookFilePut    = "playbook_file_put"
	ToolPlaybookFileAppend = "playbook_file_append"
	ToolPlaybookFileEdit   = "playbook_file_edit"
	ToolPlaybookFileRename = "playbook_file_rename"
	ToolPlaybookFileDelete = "playbook_file_delete"
	ToolPlaybookSearch     = "playbook_search"

	// MCP Tool Names - Project
	ToolProjectCreate      = "project_create"
	ToolProjectGet         = "project_get"
	ToolProjectUpdate      = "project_update"
	ToolProjectList        = "project_list"
	ToolProjectRename      = "project_rename"
	ToolProjectDelete      = "project_delete"
	ToolProjectFileList    = "project_file_list"
	ToolProjectFileGet     = "project_file_get"
	ToolProjectFilePut     = "project_file_put"
	ToolProjectFileAppend  = "project_file_append"
	ToolProjectFileEdit    = "project_file_edit"
	ToolProjectFileRename  = "project_file_rename"
	ToolProjectFileDelete  = "project_file_delete"
	ToolProjectFileSearch  = "project_file_search"
	ToolProjectFileConvert = "project_file_convert"
	ToolProjectFileExtract = "project_file_extract"

	// MCP Tool Names - Project Log
	ToolProjectLogAppend = "project_log_append"
	ToolProjectLogGet    = "project_log_get"

	// MCP Tool Names - Task Sets
	ToolTaskSetCreate = "taskset_create"
	ToolTaskSetGet    = "taskset_get"
	ToolTaskSetList   = "taskset_list"
	ToolTaskSetUpdate = "taskset_update"
	ToolTaskSetDelete = "taskset_delete"
	ToolTaskSetReset  = "taskset_reset"

	// MCP Tool Names - Tasks
	ToolTaskCreate    = "task_create"
	ToolTaskGet       = "task_get"
	ToolTaskList      = "task_list"
	ToolTaskUpdate    = "task_update"
	ToolTaskDelete    = "task_delete"
	ToolTaskRun       = "task_run"
	ToolTaskStatus    = "task_status"
	ToolTaskResults   = "task_results"
	ToolTaskResultGet = "task_result_get"
	ToolTaskReport    = "task_report"

	// MCP Tool Names - Supervisor
	ToolSupervisorUpdate = "supervisor_update"

	// MCP Tool Names - Report Generation
	ToolReportCreate = "report_create"

	// MCP Tool Names - LLM
	ToolLLMList     = "llm_list"
	ToolLLMDispatch = "llm_dispatch"
	ToolLLMTest     = "llm_test"

	// MCP Tool Names - List Management
	ToolListList       = "list_list"
	ToolListGet        = "list_get"
	ToolListGetSummary = "list_get_summary"
	ToolListCreate     = "list_create"
	ToolListDelete     = "list_delete"
	ToolListRename     = "list_rename"
	ToolListCopy       = "list_copy"

	// MCP Tool Names - List Item Management
	ToolListItemAdd    = "list_item_add"
	ToolListItemUpdate = "list_item_update"
	ToolListItemRemove = "list_item_remove"
	ToolListItemRename = "list_item_rename"
	ToolListItemGet    = "list_item_get"
	ToolListItemSearch = "list_item_search"

	// MCP Tool Names - List Task Creation
	ToolListCreateTasks = "list_create_tasks"

	// MCP Tool Names - File Operations (Cross-Domain)
	ToolFileCopy   = "file_copy"
	ToolFileImport = "file_import"

	// MCP Tool Names - Reports (read-only domain with controlled write)
	ToolReportList   = "report_list"
	ToolReportRead   = "report_read"
	ToolReportStart  = "report_start"
	ToolReportAppend = "report_append"
	ToolReportEnd    = "report_end"

	// MCP Tool Names - System
	ToolHealth = "health"

	// Project Status Constants
	ProjectStatusPending    = "pending"
	ProjectStatusInProgress = "in_progress"
	ProjectStatusDone       = "done"
	ProjectStatusCancelled  = "cancelled"

	// Task Status Constants
	TaskStatusPending    = "pending"
	TaskStatusInProgress = "in_progress"
	TaskStatusDone       = "done"
	TaskStatusFailed     = "failed"
	TaskStatusCancelled  = "cancelled"

	// Work/QA Execution Status Constants
	ExecutionStatusWaiting    = "waiting"
	ExecutionStatusProcessing = "processing"
	ExecutionStatusRetry      = "retry"
	ExecutionStatusFailed     = "failed"
	ExecutionStatusError      = "error" // Schema validation or parsing errors (response saved for audit)
	ExecutionStatusDone       = "done"

	// QA Verdict Constants (standardized values for all playbooks)
	QAVerdictPass     = "pass"     // Work is acceptable, no further action
	QAVerdictFail     = "fail"     // Work needs revision, send back to worker
	QAVerdictEscalate = "escalate" // Cannot be resolved by QA, flag for escalation

	// Path Constants
	MaxTaskPathDepth  = 3
	TaskPathSeparator = "/"
	ListPathSeparator = "__" // Double underscore to avoid conflict with hyphens in path segment names

	// Response Format Constants
	ResponseFormatText = "text"
	ResponseFormatJSON = "json"

	// File Constants
	ProjectFileName = "project.json"
	ProjectLogName  = "log.txt"
	MetaSuffix      = ".meta.json"
	ListsDir        = "lists"
	TasksDir        = "tasks"
	FilesDir        = "files"
	LogsDir         = "logs"
	ReportsDir      = "reports"

	// List Schema Version
	ListSchemaVersion = "1.0"

	// Default Values
	DefaultLimit            = 50
	DefaultLogLimit         = 100
	DefaultContextSizeLimit = 256 * 1024 // 256 KB
	DefaultTimeout          = 600        // seconds
	MinTimeout              = 60         // seconds
	MaxTimeout              = 1200       // seconds

	// Limits: Infrastructure Retries (network failures, command timeouts - no LLM cost)
	DefaultMaxRetries = 3  // Default retries for infrastructure failures
	MaxRetriesLimit   = 10 // Upper limit for retries

	// Limits: Worker Iterations (billable LLM invocations for work phase)
	DefaultMaxWorker = 2 // Default worker LLM invocations per task
	MaxWorkerLimit   = 5 // Upper limit for worker invocations

	// Limits: QA Iterations (billable LLM invocations for QA phase)
	DefaultMaxQA = 2 // Default QA iterations per task
	MaxQALimit   = 5 // Upper limit for QA iterations

	// Runner Default Values
	DefaultMaxConcurrent     = 5
	DefaultMaxRounds         = 5 // Max retry rounds per run
	DefaultRetryDelaySeconds = 60
	DefaultRateLimitRequests = 10
	DefaultRateLimitPeriod   = 60

	// Project Name Constraints
	DefaultProjectNameMaxLen = 64

	// Log Levels
	LogLevelDebug = "DEBUG"
	LogLevelInfo  = "INFO"
	LogLevelWarn  = "WARN"
	LogLevelError = "ERROR"
	LogLevelFatal = "FATAL"

	// API Key Prefix
	EnvKeyPrefix = "env:"
)

// ValidateTimeout validates and normalizes a timeout value.
// Returns the validated timeout or an error if out of bounds.
// If timeout is 0, returns DefaultTimeout.
func ValidateTimeout(timeout int) (int, error) {
	if timeout == 0 {
		return DefaultTimeout, nil
	}
	if timeout < MinTimeout {
		return 0, fmt.Errorf("timeout must be at least %d seconds", MinTimeout)
	}
	if timeout > MaxTimeout {
		return 0, fmt.Errorf("timeout must be at most %d seconds", MaxTimeout)
	}
	return timeout, nil
}

// ValidateMaxWorker validates and normalizes max_worker value.
// Returns the validated value or an error if out of bounds.
// If value is 0, returns DefaultMaxWorker.
func ValidateMaxWorker(maxWorker int) (int, error) {
	if maxWorker == 0 {
		return DefaultMaxWorker, nil
	}
	if maxWorker < 1 {
		return 0, fmt.Errorf("max_worker must be at least 1")
	}
	if maxWorker > MaxWorkerLimit {
		return 0, fmt.Errorf("max_worker must be at most %d", MaxWorkerLimit)
	}
	return maxWorker, nil
}

// ValidateMaxQA validates and normalizes max_qa value.
// Returns the validated value or an error if out of bounds.
// If value is 0, returns DefaultMaxQA.
func ValidateMaxQA(maxQA int) (int, error) {
	if maxQA == 0 {
		return DefaultMaxQA, nil
	}
	if maxQA < 1 {
		return 0, fmt.Errorf("max_qa must be at least 1")
	}
	if maxQA > MaxQALimit {
		return 0, fmt.Errorf("max_qa must be at most %d", MaxQALimit)
	}
	return maxQA, nil
}

// ValidateMaxRetries validates and normalizes max_retries value.
// Returns the validated value or an error if out of bounds.
// If value is 0, returns DefaultMaxRetries.
func ValidateMaxRetries(maxRetries int) (int, error) {
	if maxRetries == 0 {
		return DefaultMaxRetries, nil
	}
	if maxRetries < 1 {
		return 0, fmt.Errorf("max_retries must be at least 1")
	}
	if maxRetries > MaxRetriesLimit {
		return 0, fmt.Errorf("max_retries must be at most %d", MaxRetriesLimit)
	}
	return maxRetries, nil
}
