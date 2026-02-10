# Maestro Technical Reference

This document provides complete technical details for Maestro configuration, architecture, tools, and behavior.

---

## 1. Overview

Maestro is a single-user, stdio-based MCP (Model Context Protocol) server implemented in Go. It provides general-purpose orchestration capabilities for LLMs, enabling complex, multi-step analysis workflows.

### Core Capabilities

- **Three-Domain Architecture**: Reference (embedded, read-only), Playbooks (user knowledge), Projects (active work)
- **Project Management**: Projects with metadata, logs, files, lists, and task sets
- **Task Set Management**: Path-based hierarchical task organization with automated runner execution
- **QA Workflow**: Optional quality assurance phase for task verification
- **Lists**: Structured item collections with validated schemas across all domains
- **LLM Dispatch**: Multi-LLM configuration for delegated work
- **Reporting**: Generate markdown and JSON reports from task results

### Design Principles

| Principle | Description |
|-----------|-------------|
| **Persistence & Resumability** | All state written to disk (JSON/text) for session resumption and audit trails |
| **No Domain Assumptions** | General-purpose design suitable for evaluations, reviews, compliance assessments, or any systematic workflow |
| **LLM-Friendly APIs** | Stable request/response patterns with intuitive naming |
| **Path-Based Organization** | Tasks organized into hierarchical task sets using paths (e.g., `analysis/security`) |
| **List-First Pattern** | Parse documents once into lists, create tasks from list items |
| **Atomic Operations** | All file writes use temp-file-and-rename to prevent corruption |

---

## 2. Architecture

### Three Fixed Domains

The system organizes information into exactly three domains with distinct purposes:

| Domain | Purpose | Access |
|--------|---------|--------|
| **Reference** | Built-in documentation bundled at compile time | Read-only |
| **Playbooks** | User-owned, reusable knowledge that persists across projects | Read/Write |
| **Projects** | Where active work happens with full lifecycle support | Read/Write |

### System Components

```
                    +-----------------+
                    |   MCP Client    |
                    |  (Claude, etc)  |
                    +--------+--------+
                             |
                             | stdio (MCP protocol)
                             v
                    +--------+--------+
                    |     Maestro     |
                    |   MCP Server    |
                    +--------+--------+
                             |
        +--------------------+--------------------+
        |                    |                    |
        v                    v                    v
+-------+-------+    +-------+-------+    +------+------+
|   Reference   |    |   Playbooks   |    |   Projects  |
| (embedded FS) |    |  (file-based) |    | (file-based)|
+---------------+    +---------------+    +-------------+
```

### Storage Layout

```
<base_dir>/
├── playbooks/              # Reusable procedures (read/write)
│   └── <playbook-name>/
│       ├── files/
│       └── lists/
├── projects/               # Project workspaces (read/write)
│   └── <project-name>/
│       ├── project.json    # Project metadata
│       ├── log.txt         # Activity log
│       ├── files/          # Project documents
│       ├── lists/          # Structured item lists
│       ├── tasks/          # Task set files
│       │   ├── analysis.json
│       │   ├── analysis-security.json
│       │   └── qa.json
│       ├── results/        # Task execution results
│       └── reports/        # Auto-generated reports (append-only)
│           └── 20251219-1234-Security-Audit-Report.md
├── config.json             # Configuration file
└── maestro.log             # Application log
```

**Note**: Reference files are embedded in the executable and not stored on disk.

---

## 3. Configuration

Configuration is loaded from (in order of precedence):
1. `--config` CLI flag
2. `MAESTRO_CONFIG` environment variable
3. `~/.maestro/config.json` (default)

### Complete Configuration Schema

```json
{
  "version": 1,
  "base_dir": "~/.maestro",
  "chroot": "",
  "playbooks_dir": "playbooks",
  "projects_dir": "projects",
  "reference_dirs": [],
  "mark_non_destructive": false,
  "default_llm": "claude-code",
  "llms": [
    {
      "id": "claude-code",
      "display_name": "Claude Code",
      "type": "command",
      "command": "claude",
      "args": ["-p", "{{PROMPT}}"],
      "description": "Claude Code CLI with prompt on command line",
      "enabled": true
    }
  ],
  "runner": {
    "max_concurrent": 5,
    "limits": {
      "max_retries": 3,
      "max_worker": 2,
      "max_qa": 2
    },
    "retry_delay_seconds": 60,
    "rate_limit": {
      "max_requests": 10,
      "period_seconds": 60
    }
  },
  "logging": {
    "file": "maestro.log",
    "level": "INFO"
  }
}
```

### Configuration Options

#### Core Settings

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `version` | int | 1 | Configuration schema version |
| `base_dir` | string | `~/.maestro` | Root directory for all Maestro data |
| `chroot` | string | (empty) | Security boundary - all paths must be within this directory |
| `playbooks_dir` | string | `playbooks` | Directory for playbooks (relative to base_dir or absolute) |
| `projects_dir` | string | `projects` | Directory for projects (relative to base_dir or absolute) |
| `reference_dirs` | array | [] | External directories to mount in reference library. Each entry: `{"path": "/path/to/dir", "mount": "mountname"}` |
| `default_llm` | string | (empty) | Default LLM ID for task execution |

#### Security Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `chroot` | string | (empty) | When set, all configured directories must be within this path. Provides a security boundary preventing any file access outside the chroot. |
| `mark_non_destructive` | bool | false | When true, marks all write operations with `DestructiveHintAnnotation(false)` to signal that Maestro only modifies its own managed directories. |

**Chroot Example:**
```json
{
  "chroot": "~/.maestro/data",
  "base_dir": "~/.maestro/data",
  "playbooks_dir": "playbooks",
  "projects_dir": "projects"
}
```
With this configuration, all file operations are restricted to `~/.maestro/data` and its subdirectories.

#### MCP Tool Hints

All Maestro tools are annotated with MCP hints:
- **Read-only tools** (list, get, search operations): `ReadOnlyHintAnnotation(true)`
- **Write tools** (create, update, delete operations): Optionally marked with `DestructiveHintAnnotation(false)` when `mark_non_destructive` is enabled

These hints help MCP clients make informed decisions about tool permissions and user confirmations.

#### LLM Configuration

LLMs are configured as command-line executables. Maestro executes the command with the prompt either as a command-line argument (using `{{PROMPT}}` placeholder) or piped via stdin.

**Command with prompt in args:**
```json
{
  "id": "claude-code",
  "type": "command",
  "display_name": "Claude Code",
  "command": "claude",
  "args": ["-p", "{{PROMPT}}"],
  "description": "Claude Code CLI with prompt on command line",
  "enabled": true
}
```

**Command with prompt via stdin:**
```json
{
  "id": "claude-code-stdin",
  "type": "command",
  "display_name": "Claude Code (stdin)",
  "command": "claude",
  "args": ["-p"],
  "stdin": true,
  "description": "Claude Code CLI with prompt piped via stdin",
  "enabled": false
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `id` | Yes | Unique identifier for the LLM |
| `type` | No | Only `command` is supported (default) |
| `enabled` | No | Must be true for dispatch (default: false) |
| `display_name` | No | Human-readable name |
| `description` | No | Usage guidance for LLM selection |
| `command` | Yes | Executable path |
| `args` | No | Arguments; use `{{PROMPT}}` placeholder unless `stdin` is true |
| `stdin` | No | If true, prompt is piped to stdin instead of using `{{PROMPT}}` |
| `recovery` | No | Recovery configuration (see below) |

**LLM Recovery Configuration:**

```json
{
  "id": "claude-code",
  "command": "claude",
  "args": ["-p", "{{PROMPT}}"],
  "enabled": true,
  "recovery": {
    "rate_limit_patterns": ["rate limit", "quota exceeded", "429"],
    "test_prompt": "Respond with only: OK",
    "test_schedule_seconds": [30, 300, 900, 3600],
    "abort_after_seconds": 43200
  }
}
```

| Field | Default | Description |
|-------|---------|-------------|
| `rate_limit_patterns` | [] | Strings that indicate rate limiting in LLM output |
| `test_prompt` | "test" | Prompt to send when probing LLM availability |
| `test_schedule_seconds` | [30] | Escalating wait intervals between probes |
| `abort_after_seconds` | 43200 (12h) | Maximum recovery wait time before aborting |

When an LLM fails (non-zero exit code or infrastructure error) and has recovery configured, the runner enters **recovery mode**:
1. Pauses task dispatch to the failing LLM
2. Waits for the scheduled interval (escalating through `test_schedule_seconds`)
3. Probes the LLM with `test_prompt`
4. If probe succeeds, resumes normal task processing
5. If `abort_after_seconds` is exceeded, aborts the run (tasks remain in waiting status for future runs)

#### Runner Configuration

```json
{
  "runner": {
    "max_concurrent": 5,
    "max_rounds": 10,
    "round_delay_seconds": 30,
    "limits": {
      "max_retries": 3,
      "max_worker": 2,
      "max_qa": 2
    },
    "retry_delay_seconds": 60,
    "rate_limit": {
      "max_requests": 10,
      "period_seconds": 60
    },
    "default_disclaimer_template": "playbook-name/templates/disclaimer.md"
  }
}
```

| Option | Default | Description |
|--------|---------|-------------|
| `max_concurrent` | 5 | Maximum parallel task executions |
| `max_rounds` | 10 | Maximum processing rounds before giving up |
| `round_delay_seconds` | 0 | Delay between processing rounds (for rate limit recovery) |
| `limits.max_retries` | 3 | Infrastructure retry limit (network failures, no LLM cost) |
| `limits.max_worker` | 2 | Maximum worker LLM invocations per task (billable) |
| `limits.max_qa` | 2 | Maximum QA LLM invocations per task (billable) |
| `retry_delay_seconds` | 60 | Wait time between infrastructure retries |
| `rate_limit.max_requests` | 10 | Max requests per period |
| `rate_limit.period_seconds` | 60 | Rate limit period |
| `default_disclaimer_template` | (empty) | Path to disclaimer file (e.g., AI disclosure) inserted after report header |

**Note**: The limits distinguish between:
- **Retries**: Infrastructure failures (network timeouts, command failures) - no LLM cost
- **Worker invocations**: Actual LLM calls for work execution - billable
- **QA invocations**: Actual LLM calls for QA verification - billable

#### Logging

| Option | Default | Description |
|--------|---------|-------------|
| `file` | `maestro.log` | Log file name under base_dir |
| `level` | `INFO` | DEBUG, INFO, WARN, ERROR |

### Naming Rules

Use the canonical regex `^[a-zA-Z0-9][a-zA-Z0-9_-]*$` for identifiers that map to a directory or key (projects, playbooks). No dots, spaces, or other punctuation. Names are case-sensitive on disk.

### Path Rules (Task Sets)

Task set paths use lowercase alphanumeric characters with hyphens and underscores:
- Must match `^[a-z0-9][a-z0-9_-]*$` for each segment
- Segments separated by `/`
- Maximum depth of 5 levels
- Cannot start with hyphen or underscore

Examples: `analysis`, `analysis/security`, `qa/verification`

---

## 4. Reference Domain

Reference files are embedded in the executable at compile time using Go's embed package.

### Characteristics

- **Read-only**: No create, update, or delete operations
- **Immutable**: Content cannot be modified at runtime
- **Bundled**: Files come from the `docs/ai/` directory in source
- **External directories**: Optional additional directories via `reference_dirs` config

### Reference Tools

| Tool | Purpose |
|------|---------|
| `reference_list` | List all reference files (embedded + external) |
| `reference_get` | Read a reference file by path |
| `reference_search` | Search reference files by content |

External reference files appear with their configured mount prefix in paths (e.g., `user/file.md`, `standards/NIST.md`).

---

## 5. Playbooks Domain

Playbooks are user-created collections of reusable knowledge and procedures.

### Directory Structure

```
<playbooks_dir>/
  <playbook_name>/
    files/
      procedure.md
      templates/
        report.md
    lists/
      checklist.json
```

### Playbook Tools

| Tool | Purpose |
|------|---------|
| `playbook_list` | List all playbooks |
| `playbook_create` | Create a new playbook |
| `playbook_rename` | Rename a playbook |
| `playbook_delete` | Delete a playbook and all files |
| `playbook_file_list` | List files in a playbook |
| `playbook_file_get` | Read a file from a playbook |
| `playbook_file_put` | Create or update a file |
| `playbook_file_append` | Append content to a file |
| `playbook_file_edit` | Edit a file using find/replace |
| `playbook_file_rename` | Rename a file |
| `playbook_file_delete` | Delete a file |
| `playbook_search` | Search playbook files by content |

---

## 6. Projects Domain

Projects are where active work happens with full lifecycle support.

### Directory Structure

```
<projects_dir>/
  <project_name>/
    project.json          # Metadata
    log.txt               # Plain text audit log
    files/                # Project-specific files
    lists/                # Structured lists
    tasks/                # Task set JSON files
      analysis.json           # Task set at path "analysis"
      analysis-security.json  # Task set at path "analysis/security"
      qa.json                 # Task set at path "qa"
    results/              # Task execution results
      <uuid>.json             # Complete task result with history
```

### Task Result Files

Each completed task stores a comprehensive result file at `results/<uuid>.json`:

```json
{
  "task_id": 1,
  "task_uuid": "abc-123-def",
  "task_title": "Analyze REQ-001",
  "task_type": "analysis",
  "created_at": "2025-01-15T10:00:00Z",
  "completed_at": "2025-01-15T10:05:00Z",
  "worker": {
    "instructions_file": "playbook/instructions/worker.md",
    "full_prompt": "Complete prompt sent to LLM...",
    "response": "LLM response text...",
    "llm_model_id": "claude-sonnet",
    "invocations": 1,
    "status": "done"
  },
  "qa": {
    "full_prompt": "QA prompt...",
    "response": "QA response...",
    "verdict": "pass",
    "llm_model_id": "claude-opus",
    "invocations": 1,
    "status": "done"
  },
  "history": [
    {
      "timestamp": "2025-01-15T10:00:00Z",
      "role": "worker",
      "invocation": 1,
      "llm_model_id": "claude-sonnet",
      "prompt": "Full prompt text...",
      "exit_code": 0,
      "stdout": "LLM response...",
      "stderr": "",
      "response_size": 1234
    }
  ]
}
```

**Message History Fields:**

| Field | Description |
|-------|-------------|
| `timestamp` | When the message was recorded |
| `role` | "worker", "qa", or "system" |
| `invocation` | Which invocation number (1, 2, ...) |
| `llm_model_id` | LLM used for this invocation |
| `prompt` | Complete prompt sent to LLM |
| `exit_code` | Command exit code (0 = success) |
| `stdout` | Raw stdout from LLM command |
| `stderr` | Raw stderr from LLM command |
| `response_size` | Size of stdout in bytes |
| `error` | Infrastructure error message (if command couldn't execute) |

The history provides a complete audit trail of every LLM interaction, including failed attempts and retries.

### Project Metadata Schema

```json
{
  "uuid": "generated-uuid",
  "name": "project-name",
  "title": "Human-Readable Title",
  "description": "Project description",
  "status": "pending",
  "created_at": "2025-01-15T10:00:00Z",
  "updated_at": "2025-01-15T10:00:00Z",
  "default_templates": {
    "worker_response_template": "...",
    "worker_report_template": "...",
    "qa_response_template": "...",
    "qa_report_template": "..."
  }
}
```

### Project Status Values

| Status | Description |
|--------|-------------|
| `pending` | Not started |
| `in_progress` | Currently being worked on |
| `done` | Successfully completed |
| `failed` | Completed with errors |
| `cancelled` | Manually cancelled |

### Project Tools

| Tool | Purpose |
|------|---------|
| `project_create` | Create new project |
| `project_get` | Retrieve project metadata |
| `project_update` | Update project metadata |
| `project_list` | List all projects |
| `project_rename` | Rename a project |
| `project_delete` | Delete project and all contents |
| `project_file_list` | List files in a project |
| `project_file_get` | Read a file from a project |
| `project_file_put` | Create or update a file |
| `project_file_append` | Append content to a file |
| `project_file_edit` | Edit a file using find/replace |
| `project_file_rename` | Rename a file |
| `project_file_delete` | Delete a file |
| `project_file_search` | Search project files by content |
| `project_file_convert` | Convert PDF, DOCX, XLSX to Markdown |
| `project_file_extract` | Extract zip archives within project files |
| `project_log_append` | Add entry to project log |
| `project_log_get` | Retrieve log entries |

---

## 7. Task Set Architecture

Task sets organize tasks using hierarchical paths, replacing the previous subproject-based organization.

### Task Set Schema

```json
{
  "path": "analysis/security",
  "title": "Security Analysis",
  "description": "Security-focused evaluation tasks",
  "parallel": true,
  "limits": {
    "max_retries": 3,
    "max_worker": 2,
    "max_qa": 2
  },
  "worker_response_template": "...",
  "worker_report_template": "...",
  "qa_response_template": "...",
  "qa_report_template": "...",
  "created_at": "2025-01-15T10:00:00Z",
  "updated_at": "2025-01-15T10:00:00Z",
  "tasks": []
}
```

### Path-to-Filename Mapping

Task set paths are stored as files with `/` replaced by `-`:
- Path `analysis` → File `tasks/analysis.json`
- Path `analysis/security` → File `tasks/analysis-security.json`
- Path `qa/verification` → File `tasks/qa-verification.json`

### Task Set Tools

| Tool | Purpose |
|------|---------|
| `taskset_create` | Create a new task set at a path |
| `taskset_get` | Get a task set by path |
| `taskset_list` | List task sets (optionally by path prefix) |
| `taskset_update` | Update task set metadata |
| `taskset_delete` | Delete a task set and all its tasks |
| `taskset_reset` | Reset tasks to waiting status for re-execution |

### taskset_reset

Reset tasks in a task set for re-execution:

```
taskset_reset(
  project: "my-project",
  path: "analysis",
  mode: "all",           # Required: "all" or "failed"
  delete_results: true,  # Optional: delete result files (default: true)
  end_report: true       # Optional: end active report session (default: false)
)
```

| Parameter | Required | Description |
|-----------|----------|-------------|
| `project` | Yes | Project name |
| `path` | Yes | Task set path |
| `mode` | Yes | Reset mode: `"all"` (all tasks) or `"failed"` (only failed tasks) |
| `delete_results` | No | Delete result files from disk (default: true) |
| `end_report` | No | End active report session after reset (default: false) |

**Mode Explanation:**
- `"all"`: Resets all tasks to waiting status, regardless of current status
- `"failed"`: Only resets tasks with status `failed`, leaving `done` tasks unchanged

**When `end_report=true`:**
- The response includes a reminder to call `report_start` before running tasks
- Use this when you want to generate a fresh report with the re-run results

---

## 8. Task Management

Tasks are stored within task set JSON files and support automated execution with optional QA verification.

### Task Schema

Each task has separate Work and QA execution phases:

```json
{
  "id": 1,
  "uuid": "generated-uuid",
  "title": "Analyze requirement REQ-001",
  "type": "analysis",
  "created_at": "2025-01-15T10:00:00Z",
  "updated_at": "2025-01-15T10:00:00Z",
  "work": {
    "instructions_file": "instructions/analyze.md",
    "instructions_file_source": "playbook",
    "instructions_text": "Additional inline instructions",
    "prompt": "Analyze the following requirement...",
    "llm_model_id": "claude-sonnet",
    "status": "waiting",
    "result": "",
    "error": "",
    "invocations": 0,
    "last_attempt_at": null
  },
  "qa": {
    "enabled": true,
    "instructions_file": "instructions/qa.md",
    "instructions_file_source": "playbook",
    "instructions_text": "",
    "prompt": "Verify the following analysis...",
    "llm_model_id": "claude-opus",
    "status": "waiting",
    "result": "",
    "passed": false,
    "severity": "",
    "invocations": 0
  },
  "history": []
}
```

**Note**: The `invocations` field tracks the number of LLM calls used. Maximum invocations are controlled by the task set's `limits.max_worker` and `limits.max_qa` fields, which inherit from runner configuration if not set.

### Task History

Each task maintains a complete conversation history of all messages exchanged during execution. This provides full visibility into what happened during task processing, including prompts sent, responses received, and any validation errors.

#### History Entry Schema

```json
{
  "timestamp": "2025-01-15T10:00:00Z",
  "role": "worker",
  "type": "prompt",
  "content": "Full prompt content...",
  "llm_model_id": "claude",
  "invocation": 1
}
```

| Field | Description |
|-------|-------------|
| `timestamp` | When the message was recorded |
| `role` | Message source: `worker`, `qa`, or `system` |
| `type` | Message type: `prompt`, `response`, `error`, or `validation` |
| `content` | Full message content |
| `llm_model_id` | LLM used (if applicable) |
| `invocation` | Which invocation number this relates to |

#### Recorded Events

The following events are captured in task history:

| Role | Type | Description |
|------|------|-------------|
| `worker` | `prompt` | Full prompt sent to worker LLM |
| `worker` | `response` | Raw response from worker LLM (before JSON extraction) |
| `qa` | `prompt` | Full prompt sent to QA LLM |
| `qa` | `response` | Raw response from QA LLM (before JSON extraction) |
| `system` | `error` | LLM call failures, prompt build errors |
| `system` | `validation` | Schema validation failures with error details |

#### Example History

A task that fails validation and retries would have history like:

```json
{
  "history": [
    {"timestamp": "...", "role": "worker", "type": "prompt", "content": "...", "llm_model_id": "claude", "invocation": 1},
    {"timestamp": "...", "role": "worker", "type": "response", "content": "{malformed json}", "llm_model_id": "claude", "invocation": 1},
    {"timestamp": "...", "role": "system", "type": "validation", "content": "Schema validation failed: missing required field 'verdict'", "invocation": 1},
    {"timestamp": "...", "role": "worker", "type": "prompt", "content": "...", "llm_model_id": "claude", "invocation": 2},
    {"timestamp": "...", "role": "worker", "type": "response", "content": "{valid json}", "llm_model_id": "claude", "invocation": 2},
    {"timestamp": "...", "role": "qa", "type": "prompt", "content": "...", "llm_model_id": "claude", "invocation": 1},
    {"timestamp": "...", "role": "qa", "type": "response", "content": "{qa result}", "llm_model_id": "claude", "invocation": 1}
  ]
}
```

This history makes debugging straightforward - you can see exactly what was sent to each LLM and what came back, including any validation failures that caused retries.

### Execution Status Values

| Status | Description |
|--------|-------------|
| `waiting` | Ready to be executed |
| `running` | Currently executing |
| `done` | Completed successfully |
| `failed` | Failed after max attempts |

### Task Prompting Fields

Tasks support multiple prompting fields that are combined when sent to the LLM:

| Field | Purpose |
|-------|---------|
| `instructions_file` | Path to file with reusable instructions |
| `instructions_file_source` | Source: `project` (default), `playbook`, `reference` |
| `instructions_text` | Inline instructions text |
| `prompt` | Task-specific prompt (required) |

The `instructions_file_source` field determines where `instructions_file` is loaded from:
- `project`: File path within the project's files directory
- `playbook`: Path as `playbook-name/path/to/file.md` within playbooks
- `reference`: File path within the embedded reference documentation

### Task Tools

| Tool | Purpose |
|------|---------|
| `task_create` | Create a task in a task set |
| `task_get` | Get a task by UUID or path+ID |
| `task_list` | List tasks with optional filters |
| `task_update` | Update task metadata, instructions, or prompts |
| `task_delete` | Delete a task by UUID |
| `task_result_get` | Get single task result with schema for supervisor updates |

### Task Creation and Update Validation

When creating or updating tasks, Maestro validates that all referenced instruction files exist:

- **task_create**: Validates `instructions_file` and `qa_instructions_file` before creating the task
- **task_update**: Validates any instruction file being updated before applying changes
- **list_create_tasks**: Validates instruction files before creating any tasks from the list

If an instruction file does not exist, the operation returns an error immediately. This prevents tasks from being created with invalid file references that would fail at runtime.

### Updatable Task Fields (task_update)

The `task_update` tool supports updating the following fields:

| Field | Description |
|-------|-------------|
| `title` | Task title |
| `type` | Task type (for filtering) |
| `work_status` | Work execution status |
| `instructions_file` | Path to instructions file (validated) |
| `instructions_file_source` | Source: project, playbook, or reference |
| `instructions_text` | Inline instructions text |
| `prompt` | Task prompt |
| `llm_model_id` | LLM to use for execution |
| `qa_instructions_file` | QA instructions file (validated) |
| `qa_instructions_file_source` | QA source: project, playbook, or reference |
| `qa_instructions_text` | QA inline instructions |
| `qa_prompt` | QA prompt |
| `qa_llm_model_id` | LLM to use for QA |

### Task Execution Tools

| Tool | Purpose |
|------|---------|
| `task_run` | Execute eligible tasks in a task set |
| `task_status` | Get execution status and task counts |
| `task_results` | Retrieve completed task results |
| `task_report` | Generate markdown or JSON report |

---

## 9. QA Workflow

Tasks can include an optional QA phase for verification.

### QA Configuration

QA is configured per-task with these fields:

| Field | Purpose |
|-------|---------|
| `qa.enabled` | Enable QA for this task |
| `qa.instructions_file` | Instructions for QA |
| `qa.instructions_file_source` | Source (project, playbook, reference) |
| `qa.instructions_text` | Inline QA instructions |
| `qa.prompt` | QA-specific prompt |
| `qa.llm_model_id` | LLM to use for QA (can differ from work phase) |

**Note**: Maximum QA iterations are controlled by the task set's `limits.max_qa` field, not per-task.

### QA Results

| Field | Description |
|-------|-------------|
| `passed` | Whether QA verification passed |
| `severity` | Issue severity (low, medium, high, critical) |
| `result` | QA feedback and findings |
| `invocations` | Number of QA LLM invocations used |

### QA Execution Flow

1. **Work phase** executes first
2. If work succeeds and QA is enabled, **QA phase** executes
3. QA evaluates the work result and must return valid JSON matching the QA schema
4. If QA response fails schema validation, QA is retried with error feedback (up to `limits.max_qa`)
5. QA can pass or fail with severity rating
6. If QA fails and iterations remain, work can be revised
7. Task completes when both phases pass or max iterations reached

### Schema Validation Retry

Both worker and QA phases retry on schema validation failures:

- **Automatic retry**: When a response doesn't match the required schema, the LLM receives error feedback
- **User-friendly errors**: Validation errors are transformed to clear messages (e.g., "Missing required field: verdict")
- **Complete transaction log**: All prompts and responses (including failures) are recorded in the task's message history
- **Max invocations**: Retries count against `limits.max_worker` (worker) or `limits.max_qa` (QA)

**Error feedback includes:**
- Specific fields that failed validation
- Expected vs. actual types
- Missing required fields
- Unexpected additional fields

---

## 10. Lists

Lists are structured JSON files for managing collections of items.

### Purpose

- Parse source documents once during project setup
- Create structured lists with unique item identifiers
- Generate tasks from list items using `list_create_tasks`
- Store reusable item collections in playbooks

### List Schema

```json
{
  "version": "1",
  "name": "Human-readable list name",
  "description": "Optional description",
  "created_at": "2025-01-15T10:00:00Z",
  "updated_at": "2025-01-15T10:00:00Z",
  "items": [
    {
      "id": "REQ-001",
      "title": "Authentication Required",
      "content": "The system shall authenticate all users",
      "source_doc": "requirements.md",
      "section": "3.1 Security",
      "tags": ["security", "authentication"],
      "complete": false
    }
  ]
}
```

### Item Schema

| Field | Required | Description |
|-------|----------|-------------|
| `id` | Yes | Unique identifier within the list |
| `title` | Yes | Short display name |
| `content` | Yes | Main item content |
| `source_doc` | No | Reference to source document |
| `section` | No | Section within source document |
| `tags` | No | Array of classification tags |
| `complete` | No | Completion status (default: false) |

### List Tools

| Tool | Purpose |
|------|---------|
| `list_create` | Create a new list |
| `list_get` | Get full list with items |
| `list_get_summary` | Get list summary with truncation |
| `list_list` | List all lists |
| `list_rename` | Rename a list |
| `list_delete` | Delete a list |
| `list_copy` | Copy a list between sources |

### List Item Tools

| Tool | Purpose |
|------|---------|
| `list_item_add` | Add an item to a list |
| `list_item_get` | Get a specific item |
| `list_item_update` | Update an item |
| `list_item_rename` | Rename an item's ID |
| `list_item_remove` | Remove an item |
| `list_item_search` | Search items |

### Creating Tasks from Lists

The `list_create_tasks` tool generates one task per list item:

```
list_create_tasks(
  project: "my-project",
  list_project: "my-project",
  list: "requirements",
  path: "analysis",
  title_template: "Analyze {{id}}",
  type: "analysis",
  prompt: "Analyze the following requirement...",
  llm_model_id: "claude-sonnet",
  sample: 3
)
```

This creates a task in the `analysis` task set for each item in the requirements list.

#### Random Sampling

The optional `sample` parameter limits task creation to a random subset of list items:

| Parameter | Type | Description |
|-----------|------|-------------|
| `sample` | int | If specified, randomly select this many items from the list |

This is useful for:
- **Test audits**: Run the full workflow with a small subset (e.g., `sample=3`)
- **Pilot runs**: Validate playbook configuration before full execution
- **Cost estimation**: Process a sample to estimate time/cost for the full list

The sampling uses Fisher-Yates shuffle for unbiased random selection.

### Copying Lists

The `list_copy` tool copies a list between sources (playbooks and projects):

```
list_copy(
  from_source: "playbook",
  from_name: "security-audit",
  from_list: "controls",
  to_source: "project",
  to_name: "my-audit",
  to_list: "controls",
  sample: 5
)
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `from_source` | string | Yes | Source type: `playbook` or `project` |
| `from_name` | string | Yes | Source playbook or project name |
| `from_list` | string | Yes | Source list name |
| `to_source` | string | Yes | Destination type: `playbook` or `project` |
| `to_name` | string | Yes | Destination playbook or project name |
| `to_list` | string | Yes | Destination list name |
| `sample` | int | No | If specified, copy only this many randomly selected items |

The `sample` parameter enables copying a random subset for testing purposes.

---

## 11. Runner & Execution

The runner automates task execution for eligible tasks via `task_run`.

### Execution Flow

1. **Task Selection**: Find tasks with status `waiting` or `failed` (with retries remaining)
2. **Prompt Assembly**: Build prompt from instructions and prompt fields
3. **LLM Dispatch**: Send to configured LLM
4. **Result Storage**: Store result in task's `work.result` field
5. **Status Update**: Set `done` (success) or `failed` (after max attempts)
6. **QA Phase**: If enabled and work succeeded, run QA verification
7. **Auto-Report**: When all tasks complete, generate and save report to project files

### Auto-Report Generation

When `task_run` completes (all eligible tasks executed), the runner automatically:
1. Builds a report from all task sets (filtered by path if specified)
2. Generates hierarchical markdown format
3. Saves to project files with timestamped filename (e.g., `report_2025-01-15_10-30-00.md`)
4. Logs the report filename to the project log

This ensures a report is always available even when the orchestrating LLM is no longer monitoring the project.

### Prompt Assembly Order

The runner assembles the full prompt as:
1. Load and append `instructions_file` content from specified source (if set)
2. Append `instructions_text` (if set)
3. Append `=== TASK PROMPT ===` separator
4. Append `prompt` text

### Concurrency Control

- When `parallel=true` on a task set, up to `runner.max_concurrent` tasks run simultaneously
- When `parallel=false` (default), tasks run sequentially with dependency assumptions
- The `parallel` setting can be overridden at runtime: `task_run(..., parallel="true")`
- Rate limiting prevents API overload

### Round-Robin Execution Model

The runner processes tasks in **rounds** rather than retrying individual tasks in place:

**Round Processing:**
1. Each round iterates through all tasks needing work (waiting or retry status)
2. After each round, tasks in waiting/retry status are queued for the next round
3. Processing continues until all tasks complete or `max_rounds` is reached

**Sequential Mode Behavior:**
- Tasks are assumed to have dependencies on previous tasks
- If any task doesn't complete (fails, remains in waiting), the round ends immediately
- Remaining tasks in that round are deferred to the next round
- This preserves task ordering for dependent workflows

**Parallel Mode Behavior:**
- Tasks are independent and can run concurrently
- If a task fails, other tasks in the round continue
- Failed tasks are retried in subsequent rounds

**Round Delays:**
- Configure `round_delay_seconds` in runner config to add delays between rounds
- Useful for rate limit recovery or throttling overnight batch jobs

### Recovery Mode

When an LLM fails and has `recovery` configured, the runner enters recovery mode:

**Trigger Conditions:**
- Non-zero exit code from LLM command
- Infrastructure error (command execution failed)
- Rate limit patterns detected in output

**Recovery Behavior:**
1. Runner pauses new task dispatch
2. Waits according to `test_schedule_seconds` (escalating intervals)
3. Probes LLM with `test_prompt`
4. On success: exits recovery mode, resumes processing
5. On failure: advances to next schedule interval and waits again
6. After `abort_after_seconds`: aborts run, tasks remain in waiting status

**Why Tasks Remain in Waiting (Not Failed):**
- Tasks that weren't attempted due to recovery timeout aren't "failed" - they never ran
- Leaving them in waiting status allows a future `task_run` to pick them up
- This enables graceful recovery from extended outages

**Example:**
```json
{
  "recovery": {
    "rate_limit_patterns": ["rate limit", "429"],
    "test_prompt": "Respond with only: OK",
    "test_schedule_seconds": [30, 300, 900, 3600],
    "abort_after_seconds": 43200
  }
}
```

This configuration:
- Probes after 30 seconds, then 5 minutes, 15 minutes, 1 hour
- Aborts after 12 hours of waiting
- Detects rate limits via "rate limit" or "429" in LLM output

### Graceful Shutdown

Maestro ensures all running tasks complete before exiting, even if the calling process (e.g., Claude Code) terminates:

**Behavior on shutdown signal or EOF:**
1. Maestro catches SIGINT, SIGTERM, SIGHUP, or stdin EOF
2. If the runner is active, Maestro waits for all tasks to complete
3. Reports are generated and saved
4. Only then does Maestro exit

**Why this matters:**
- In multi-tier workflows, an orchestrating LLM may start a task runner and then exit
- The runner continues processing all tasks to completion
- Reports are always generated, even if the calling session ends

This ensures reliability for long-running task sets and prevents work loss due to session timeouts.

### Invocation and Retry Logic

The system distinguishes between **infrastructure errors** and **LLM errors**:

| Error Type | Examples | Limit | Cost |
|------------|----------|-------|------|
| **Infrastructure** | Command not found, timeout, permission denied | `max_retries` | No LLM cost |
| **LLM Error** | Non-zero exit code, validation failure | `max_worker` / `max_qa` | Billable |

**Tracking fields:**

| Field | Location | Purpose |
|-------|----------|---------|
| `work.infra_retries` | Task | Infrastructure failures in work phase |
| `work.invocations` | Task | LLM calls in work phase (any exit code) |
| `qa.infra_retries` | Task | Infrastructure failures in QA phase |
| `qa.invocations` | Task | LLM calls in QA phase (any exit code) |

**Behavior:**
- Infrastructure failures (command couldn't execute) retry up to `limits.max_retries` with no LLM cost
- LLM invocations (command executed, any exit code) count against `limits.max_worker` or `limits.max_qa`
- After limits exhausted, task marked as `failed`
- All prompts and responses are recorded in the task's message history for debugging

### Budget Safeguard

When `task_run` is invoked, the runner calculates a safety budget to prevent runaway costs:

```
budget = task_count × (max_worker + max_qa) × 1.10
```

The 10% buffer accounts for retries. If total LLM calls exceed this budget, the runner halts with an error. This prevents infinite loops or misconfigured tasks from causing unexpected costs.

Example: 100 tasks with `max_worker=2` and `max_qa=2` → budget = 100 × 4 × 1.10 = 440 calls

### Worker LLM Requirements

If a worker task needs to call Maestro tools (create lists, read files, etc.), the configured LLM must have MCP access to Maestro. Use command-type LLMs like Claude Code or OpenAI Codex in headless mode with `--mcp-config` pointing to a Maestro configuration.

If the LLM does not have MCP access to Maestro, the task prompt must be self-contained with all necessary context injected upfront.

---

## 12. Schema Validation & Report Templates

Worker responses must conform to JSON schemas for validation and automated report generation.

### Validation Requirement

**All worker tasks must return JSON that conforms to a defined schema.** Maestro validates every worker response and rejects non-conforming responses, returning an error to the worker for correction.

### Schema Resolution Order

Schemas are resolved in this order:
1. **Task set level**: `worker_response_template` / `qa_response_template` fields
2. **Project level**: `default_templates` in project.json
3. **Playbook schemas**: Files in `playbook-name/schemas/` directory

### Required Playbook Files

```
playbook-name/
├── schemas/
│   ├── worker_response.json    # JSON Schema (draft-07) for worker output
│   └── qa_response.json        # JSON Schema (draft-07) for QA output
├── templates/
│   ├── worker_response.md      # Go template for worker report output
│   └── qa_response.md          # Go template for QA report output
└── instructions/
    ├── worker_instructions.md  # Must instruct worker to return JSON
    └── qa_instructions.md      # Must instruct QA to return JSON
```

### JSON Schema Format

Schemas use JSON Schema draft-07:

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "$id": "playbook-name/schemas/worker_response",
  "title": "Worker Response Schema",
  "type": "object",
  "properties": {
    "item_id": {
      "type": "string",
      "description": "Identifier from task context"
    },
    "status": {
      "type": "string",
      "enum": ["complete", "information required", "review required"],
      "description": "Assessment status"
    },
    "test": {
      "type": "string",
      "description": "Brief description of testing methodology"
    },
    "evidence": {
      "type": "array",
      "description": "Structured list of evidence documents",
      "items": {
        "type": "object",
        "required": ["num", "document"],
        "properties": {
          "num": { "type": "integer" },
          "document": { "type": "string" },
          "section": { "type": "string" },
          "path": { "type": "string", "description": "Full path for QA verification" }
        }
      }
    },
    "observations": {
      "type": "string",
      "description": "Brief observations about the finding"
    },
    "summary": {
      "type": "string",
      "description": "Professional summary for external reporting"
    },
    "rationale": {
      "type": "string",
      "description": "Internal reasoning with evidence path references for QA"
    }
  },
  "required": ["item_id", "status", "summary", "rationale"],
  "additionalProperties": true
}
```

### Markdown Template Format

Templates use Go's `text/template` syntax with field names matching the JSON schema:

```markdown
### {{.item_id}}

**Status**: {{.status}}

**Test**
{{.test}}

**Evidence**
{{range .evidence}}
{{.num}}. {{.document}}, {{.section}}
{{end}}

**Observations**
{{.observations}}

{{.summary}}

**Evidence Paths** (for QA verification)
{{range .evidence}}
{{.num}}. {{.path}}, {{.section}}
{{end}}
```

### Field Matching Requirement

**JSON schema field names must exactly match markdown template placeholders**:

| Schema Field | Template Reference |
|--------------|-------------------|
| `item_id` | `{{.item_id}}` |
| `status` | `{{.status}}` |
| `evidence[].num` | `{{.num}}` (in range) |
| `evidence[].document` | `{{.document}}` (in range) |
| `summary` | `{{.summary}}` |

Mismatched names cause template rendering failures.

### Validation Flow

```
┌─────────────┐     ┌──────────────┐     ┌────────────────┐
│   Worker    │────▶│   Validate   │────▶│ Store Result   │
│  Response   │     │   vs Schema  │     │ (if valid)     │
└─────────────┘     └──────┬───────┘     └────────────────┘
                          │
                          │ Invalid
                          ▼
                   ┌──────────────┐     ┌────────────────┐
                   │ Return Error │────▶│ Worker Retries │
                   │  to Worker   │     │ with Feedback  │
                   └──────────────┘     └────────────────┘
```

### Validation Behavior

1. **Extraction**: Maestro extracts JSON from worker response (handles markdown code blocks)
2. **Validation**: JSON validated against schema
3. **On Success**: Result stored, task marked done
4. **On Failure**: Error returned to worker with specific validation failures
5. **Retry**: Worker attempts to fix and resubmit (up to `limits.max_worker` invocations)
6. **Final Failure**: After max invocations exhausted, task marked failed with last error

### Validation Error Examples

Schema violations return specific error messages:

```
Validation failed:
- $.verdict: value "maybe" is not one of: Pass, Fail, Partial, N/A
- $.summary: required field missing
```

### Worker Instruction Requirements

Worker instructions must explicitly require JSON output:

```markdown
## Output Format

Return your response as a single JSON object matching the schema.

Required fields:
- item_id: Copy from task context
- verdict: One of "Pass", "Fail", "Partial", "N/A"
- summary: 1-3 sentences, professional tone
- rationale: Internal explanation of your reasoning

Example:
{
  "item_id": "REQ-001",
  "verdict": "Pass",
  "summary": "The requirement is fully satisfied...",
  "rationale": "Evidence found in section 3.2..."
}
```

### QA Schema Pattern

QA schemas must include a standardized `verdict` field for workflow control. The verdict must be one of:
- `pass`: Work is acceptable, no further action
- `fail`: Work needs revision, send back to worker (if retries remain)
- `escalate`: Cannot be resolved by QA, flag for escalation

Maestro validates QA schemas at task set creation time to ensure they include the required verdict field with these values (case-insensitive).

QA schemas typically also include document verification to ensure worker evidence is accurate:

```json
{
  "item_id": "string - Reference to the item being verified",
  "verdict": "pass | fail | escalate",
  "document_verification": [
    {
      "evidence_num": 1,
      "document": "Document name from worker evidence",
      "section": "Section referenced",
      "path": "Path to document",
      "exists": true,
      "supports_finding": true,
      "notes": "Verification notes"
    }
  ],
  "comments": "Required summary of QA work (must always be provided)",
  "issues": [
    {
      "type": "evidence_not_found | unsupported_claim | example_value_used | missing_path",
      "evidence_num": 1,
      "description": "Description of issue",
      "severity": "critical | major | minor"
    }
  ]
}
```

**Automatic QA failure conditions**:
- Document cited in evidence cannot be found at the specified path
- Section does not contain content supporting the worker's claim
- Worker used example/placeholder values (e.g., "DocumentName.docx", "Section 42")
- Evidence paths are missing or incomplete

### Template Conditional Logic

Templates support conditional rendering:

```markdown
{{if eq .verdict "fail"}}
⚠️ **FAILED**: {{.summary}}
{{else if eq .verdict "escalate"}}
⚡ **ESCALATED**: {{.summary}}
{{else}}
✓ {{.summary}}
{{end}}

{{if .issues}}
### Issues Found
{{range .issues}}
- **{{.severity}}**: {{.description}}
{{end}}
{{end}}
```

### Report Generation

When `task_report` is called:

1. Each task's validated JSON result is loaded
2. The appropriate markdown template is applied
3. Rendered markdown is concatenated into the report
4. Report includes summary statistics and any QA findings

---

## 13. Reporting

Reports in Maestro are **auto-generated** when the runner completes task sets. Additionally, manual reports can be generated using `task_report`.

### Auto-Generated Reports

When the runner completes a task set, reports are automatically appended to the project's main report file:
- Location: `<project>/reports/<prefix>Report.md`
- Prefix format: `YYYYMMDD-HHMM-<title>-` (e.g., `20251219-1234-Security-Audit-`)
- Each task's results are rendered using configured templates

### Report Header Format

When a report file is first created, Maestro automatically adds:

```markdown
# <Report Title>

**Issued:** YYYY-MM-DD

<Optional intro paragraph>

<Disclaimer template content>
```

- **Title**: From `report_start` title parameter
- **Issued date**: Captured when `report_start` is called (not when content is appended)
- **Intro**: Optional introductory paragraph from `report_start`
- **Disclaimer**: Loaded from project's `disclaimer_template` field (mandatory)

This ensures the issued date reflects when the report session began, not when the final content was written.

### Disclaimer Template (Mandatory)

Every project must specify a `disclaimer_template`. This field is validated at:

1. **Project creation** - Must be provided and not empty
2. **Runner start** - Must point to an existing file (or be `"none"`)

**Valid values:**

| Value | Behavior |
|-------|----------|
| `"playbook-name/path/to/file.md"` | Loads disclaimer from playbook's files directory |
| `"none"` | No disclaimer included in reports |

**Path format**: `playbook-name/relative/path.md`
- First segment is the playbook name
- Remaining path is relative to the playbook's root directory
- Example: `"iso-27001/templates/disclaimer.md"` → loads from `<playbooks>/iso-27001/templates/disclaimer.md`

**Validation:**

- Empty string or omitting the field causes an error at project creation
- Invalid path format (missing playbook name) causes an error
- Non-existent file causes an error when the runner starts
- Using `"none"` skips file validation and produces reports without disclaimers

**Recommended content:**

Disclaimers typically include:
- AI disclosure: Statement that AI tools assisted with the work
- Methodology: Brief explanation of how evaluations were conducted
- Scope: What was and wasn't covered
- Limitations: Any constraints on how findings should be interpreted

**Example:**

```markdown
## Disclaimer

This assessment was conducted using Maestro, an AI-assisted analysis tool.
While AI technology was used to help structure and process the evaluation,
all findings were reviewed and validated by qualified personnel.

This report is based on information available at the time of assessment
and should not be relied upon for purposes beyond its intended scope.
```

### Multi-Template Reports

For projects requiring multiple report variants (e.g., client-facing and internal), use a **JSON manifest** instead of a single markdown template.

**Manifest format** (`templates/worker-reports.json`):
```json
[
  {"suffix": "Report", "file": "worker-report-client.md"},
  {"suffix": "Internal", "file": "worker-report-internal.md"}
]
```

**QA manifest format** (`templates/qa-reports.json`):
```json
[
  {"suffix": "Report", "file": "qa-report-client.md"},
  {"suffix": "Internal", "file": "qa-report-internal.md"}
]
```

**How it works:**
- If `worker_report_template` or `qa_report_template` ends in `.json`, Maestro parses it as a manifest
- If it ends in `.md`, Maestro uses it as a single template (backwards compatible)
- Each manifest entry produces a separate report file: `<prefix>Report.md`, `<prefix>Internal.md`, etc.
- Template file paths are relative to the manifest location

**Use case:**
- `Report.md` - Clean client-facing report (no internal notes, no QA details)
- `Internal.md` - Full details including rationale, evidence paths, QA verification

### Report Tools

| Tool | Purpose |
|------|---------|
| `report_start` | Start a new report session with a prefix |
| `report_append` | Append content to a report |
| `report_end` | End the current report session |
| `report_list` | List all reports in a project |
| `report_read` | Read a specific report |
| `report_create` | Generate reports from task results (same as runner auto-report) |

**Starting a Report Session**
```
report_start(
  project: "my-project",
  title: "Security Audit",
  intro: "This report documents the audit findings."
)
```

**Appending to Reports**
```
report_append(
  project: "my-project",
  content: "## Executive Summary\n\n..."
)
```

**Listing Reports**
```
report_list(project: "my-project")
```

**Generating Reports from Task Results**
```
report_create(
  project: "my-project",
  path: "analysis"  # Optional: filter by task set path
)
```

The `report_create` tool uses the same report generation logic as the runner's auto-report feature. It:
- Adds each task set to the report manifest
- Generates report content using configured templates
- Appends to the current report session (or auto-initializes one)
- Returns a list of generated report filenames

### Supervisor Tools

The supervisor tools enable human review and modification of AI-generated task results.

| Tool | Purpose |
|------|---------|
| `supervisor_update` | Replace worker response with supervisor's content |
| `task_result_get` | Get single task result with schema (see Task Tools) |

**Getting Task Results for Review**

Use `task_result_get` to retrieve a single task result along with the schema needed for supervisor updates:

```
task_result_get(
  project: "my-project",
  uuid: "abc123-..."
)
```

Returns:
- `worker_response` - The JSON response from the worker
- `worker_response_template` - Path to the schema file
- `worker_response_schema` - **Full schema content** (no need to fetch separately)
- `qa_response`, `qa_verdict`, `qa_status` - QA verification results
- `supervisor_override` - Whether already updated by supervisor
- `completed_at` - When the task was completed

**Supervisor Update**

The `supervisor_update` tool allows a human supervisor to replace an AI worker's response with their own corrected version:

```
supervisor_update(
  project: "my-project",
  uuid: "abc123-...",
  response: "{...supervisor's JSON response...}"
)
```

Key behaviors:
- **Audit trail**: The supervisor's response is appended to task history (never modifies existing entries)
- **SupervisorOverride flag**: Set to `true` in the task result
- **Template validation**: Response must match `worker_response_template` if defined
- **QA data cleared**: Previous QA verification is removed (no longer relevant to updated response)
- **QA status set to "superseded"**: Indicates QA was invalidated by supervisor action
- **QA verdict set to "N/A"**: Reports will show "QA: N/A" instead of stale pass/fail
- **Task status**: Set to "done"

This is useful for:
- Correcting AI responses that are incomplete or inaccurate
- Adding domain expertise the AI may not have
- Adjusting responses for organizational context

**After Supervisor Updates**

After applying supervisor updates, regenerate reports to reflect the changes:

```
report_create(project: "my-project")
```

This creates **new report files** with a fresh timestamp prefix. Updated findings will show:
- The supervisor's corrected response
- "QA: N/A" in the header (since QA was superseded)
- No stale QA verification data

### Manual Reports (task_report)

The `task_report` tool generates custom reports from task results.

### Report Formats

**Markdown Report**
```
task_report(
  project: "my-project",
  format: "markdown"
)
```

Produces a hierarchical markdown document with:
- Project summary and statistics
- Results organized by task set path
- Worker results formatted via `worker_report_template`
- QA review for ALL QA-enabled tasks (not just failures)

**JSON Report**
```
task_report(
  project: "my-project",
  format: "json"
)
```

Produces structured JSON for programmatic processing.

### Template Rendering

Reports use templates configured at the task set level:

| Template | Purpose |
|----------|---------|
| `worker_report_template` | Format worker JSON results for the report |
| `qa_report_template` | Format QA JSON results for the report |

Templates receive the parsed JSON from worker/QA responses. Field names in templates must match the JSON schema fields exactly.

### QA in Reports

For each QA-enabled task, the report includes:
- **QA status**: Pass/Fail indicator
- **QA feedback**: Notes or feedback from QA (extracted from `notes` or `feedback` fields)
- **Issues list**: Any issues identified (from `issues` array)
- **Template output**: If `qa_report_template` is configured, QA results are rendered through it

QA results appear for ALL QA-enabled tasks, providing visibility into verification even when QA passes.

### Filtering Options

| Parameter | Description |
|-----------|-------------|
| `path` | Task set path prefix |
| `status` | Execution status filter |
| `type` | Task type filter |
| `qa_passed` | QA pass/fail filter |
| `qa_severity` | Minimum severity level |

---

## 14. LLM Dispatch

### LLM Tools

| Tool | Purpose |
|------|---------|
| `llm_list` | List configured LLMs with status |
| `llm_dispatch` | Send a prompt to a specific LLM |
| `llm_test` | Test if an LLM is available and responding |

### llm_test

Test LLM availability before starting tasks:

```
llm_test(llm_id: "claude-sonnet")
# Returns: { "available": true } or { "available": false, "error": "..." }
```

### Pre-flight LLM Check

Before `task_run` executes any tasks, Maestro automatically tests all LLMs that will be used (worker + QA LLMs). If any LLM is unavailable, execution fails immediately before wasting time or resources.

### Error Handling

Maestro distinguishes between two types of errors:

| Error Type | Examples | Counter | No LLM Cost |
|------------|----------|---------|-------------|
| **Infrastructure** | Command not found, timeout, permission denied | `max_retries` | ✓ |
| **LLM Error** | Non-zero exit code, validation failure | `max_worker` / `max_qa` | ✗ |

Infrastructure errors don't consume your LLM invocation budget, while LLM errors count against the worker/QA limits.

### Rate Limit Configuration

LLMs can be configured with rate limit detection patterns:

```json
{
  "llms": [{
    "id": "claude-sonnet",
    "rate_limit": {
      "patterns": ["rate limit", "quota exceeded", "429"],
      "test_prompt": "Respond with only OK"
    }
  }]
}
```

When rate limit patterns are detected in LLM output, Maestro can identify the issue and handle it appropriately.

### Dispatch Parameters

```
llm_dispatch(
  llm_id: "claude-sonnet",
  prompt: "Your prompt here",
  timeout: 300
)
```

| Parameter | Required | Description |
|-----------|----------|-------------|
| `llm_id` | Yes | LLM identifier from config |
| `prompt` | Yes | Prompt to send |
| `timeout` | No | Timeout in seconds (60-900, default: 300) |

---

## 15. System Tools

### health

Check system health and configuration status.

```
Parameters: none
Returns:
  - Base directory status (exists, writable)
  - Configuration path
  - Number of enabled LLMs
  - Chroot status (if configured)
  - Any issues requiring attention
```

### file_copy

Copy files between domains (playbooks, projects).

```
Parameters:
  source_type: string - 'playbook' or 'project'
  source_name: string - Playbook or project name
  source_path: string - File path in source
  dest_type: string - 'playbook' or 'project'
  dest_name: string - Playbook or project name
  dest_path: string - File path in destination
```

### file_import

Import external files from anywhere on the filesystem into a project.

```
Parameters:
  project: string - Target project name
  source: string - Absolute path to file or directory
  recursive: boolean - Import directories recursively (default: false)
  convert: boolean - Convert PDF, DOCX, XLSX to Markdown (default: false)

Returns:
  project: string - Project name
  source: string - Source path
  recursive: boolean - Whether recursive import was used
  files_imported: int - Number of files imported
  links_imported: int - Number of symlinks imported
  links_removed: int - Number of unsafe symlinks removed
  imported_to: string - Relative path in project files (always "imported/...")
  converted: int - Files converted to Markdown (if convert=true)
  convert_skipped: int - Files already converted/unsupported
  convert_failed: int - Conversion failures
```

**Security**: Symlinks that point outside the imported folder are automatically removed. This prevents path traversal attacks through symbolic links.

### project_file_extract

Extract zip archives within a project's files directory.

```
Parameters:
  project: string - Project name
  path: string - Path to .zip file within project files
  overwrite: boolean - Overwrite existing files (default: false)
  convert: boolean - Convert extracted files to Markdown (default: false)

Returns:
  project: string - Project name
  path: string - Zip file path
  extracted_to: string - Directory where files were extracted
  files_extracted: int - Number of files extracted
  files_skipped: int - Files skipped (already exist, overwrite=false)
  converted: int - Files converted (if convert=true)
  convert_skipped: int - Conversion skips
  convert_failed: int - Conversion failures
```

Use `project_file_delete` to remove the archive after extraction if desired.

**Extraction behavior**:
- Archives are extracted in place: `foo.zip` → `foo/` in the same directory
- Path traversal attacks in zip entries (e.g., `../etc/passwd`) are blocked
- Symlinks in extracted content that escape the project are removed

### project_file_convert

Convert document files to Markdown format.

```
Parameters:
  project: string - Project name
  path: string - Path to file or directory
  recursive: boolean - Convert directories recursively (default: false)

Returns:
  project: string - Project name
  path: string - Conversion target path
  converted: int - Files successfully converted
  skipped: int - Files skipped (already converted or unsupported)
  failed: int - Conversion failures
```

**Supported formats**: PDF, DOCX, XLSX

**Note**: The x2md conversion library is optimized for LLM consumption, not human reading. Due to the inherent limitations of Markdown as a format, complex document layouts, tables, images, and formatting may not be preserved with full fidelity. The converted output is intended to make document content accessible to LLMs for analysis, not for redistribution or human review.

---

## 16. Persistence & Concurrency

### Atomic Writes

All file modifications follow the temp-file-and-rename pattern:
1. Write content to temporary file
2. Rename temporary file to target path

This prevents partial writes and corruption.

### File Locking

- File-level locking using flock for task set operations
- Per-project mutex for atomic operations
- Cross-process safety for multi-instance deployments

### Persistence Guarantees

- Changes written to disk immediately
- No in-memory caching of file contents
- All state survives process restart
- Sessions can be resumed after interruption

---

## 17. Error Handling

### Project Errors
- `project not found: <name>`
- `invalid project name: <reason>`
- `project already exists: <name>`

### Task Set Errors
- `task set not found: <path>`
- `task set already exists: <path>`
- `invalid path: <reason>`

### Task Errors
- `task not found: <uuid>`
- `at least one prompt field is required`
- `task set does not exist for path: <path>`

### List Errors
- `list not found: <name>`
- `item already exists: <id>`
- `item not found: <id>`

### LLM Errors
- `llm not found: <id>`
- `llm disabled: <id>`
- `llm dispatch failed: <error>`

### Configuration Errors
- `chroot path must be absolute: <path>`
- `<directory> is outside chroot <path>`

---

## 18. CLI Reference

```bash
# Run with default config (~/.maestro/config.json)
./maestro

# Run with custom config
./maestro --config /path/to/config.json

# Show version
./maestro --version

# Show help
./maestro --help
```

### Environment Variables

| Variable | Purpose |
|----------|---------|
| `MAESTRO_CONFIG` | Path to configuration file |

---

## 19. MCP Tool Summary

### Reference Tools (3) - Read-Only
`reference_list`, `reference_get`, `reference_search`

### Playbook Tools (12)
`playbook_list`, `playbook_create`, `playbook_rename`, `playbook_delete`
`playbook_file_list`, `playbook_file_get`, `playbook_file_put`, `playbook_file_append`, `playbook_file_edit`, `playbook_file_rename`, `playbook_file_delete`, `playbook_search`

### Project Tools (18)
`project_create`, `project_get`, `project_update`, `project_list`, `project_rename`, `project_delete`
`project_file_list`, `project_file_get`, `project_file_put`, `project_file_append`, `project_file_edit`, `project_file_rename`, `project_file_delete`, `project_file_search`, `project_file_convert`, `project_file_extract`
`project_log_append`, `project_log_get`

### Task Set Tools (6)
`taskset_create`, `taskset_get`, `taskset_list`, `taskset_update`, `taskset_delete`, `taskset_reset`

### Task Tools (10)
`task_create`, `task_get`, `task_list`, `task_update`, `task_delete`, `task_result_get`
`task_run`, `task_status`, `task_results`, `task_report`

### List Tools (14)
`list_create`, `list_get`, `list_get_summary`, `list_list`, `list_rename`, `list_delete`, `list_copy`
`list_item_add`, `list_item_get`, `list_item_update`, `list_item_rename`, `list_item_remove`, `list_item_search`
`list_create_tasks`

### Report Tools (6)
`report_list`, `report_read`, `report_start`, `report_append`, `report_end`, `report_create`

### Supervisor Tools (1)
`supervisor_update`

### LLM Tools (3)
`llm_list`, `llm_dispatch`, `llm_test`

### System Tools (3)
`health`, `file_copy`, `file_import`

**Total: 76 MCP Tools**
