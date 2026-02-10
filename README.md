# Maestro

Development status: Beta

A Go-based MCP (Model Context Protocol) server that provides general-purpose orchestration capabilities for LLMs. Maestro implements a three-domain architecture (Reference, Playbooks, Projects), project-scoped task management with automated runner execution, and multi-LLM dispatch functionality for complex, multi-step analysis workflows.

Stdio MCP transport is used to simplify local use and concurrency.

## Quick Start

1. Build: `go build -o maestro`
2. Run: `./maestro` (creates default config on first run)
3. Configure your MCP client to use Maestro as a stdio server
4. Instruct the LLM: `Please use the Maestro reference tool to read readme.md and follow the instructions.`

## Features

- **Three-Domain Architecture**: Reference (embedded docs + optional user files, read-only), Playbooks (user knowledge), Projects (active work)
- **Project Management**: Projects with subprojects (one level), metadata, logs, task lists, and file storage
- **Playbooks**: User-created collections of reusable procedures and knowledge
- **Structured Lists**: JSON-based item collections with validated schemas across all domains
- **Automated Runner**: Execute tasks automatically with configurable concurrency, rate limiting, and retry logic
- **LLM Dispatch**: Multi-LLM configuration and delegation for specialized work
- **Index-First Pattern**: Parse documents once, create indexes, reference items by ID across tasks
- **Persistence & Resumability**: All state written to disk for session resumption

## Configuration

Maestro attempts to read a JSON configuration as follows:
1. `--config` CLI flag
2. `MAESTRO_CONFIG` environment variable
3. `~/.maestro/config.json` (default)

On first run, Maestro creates the `~/.maestro` directory and a starter configuration.

See `reference/config-example.json` for a complete example with all options.

NOTE: You must enable at least once cli-based LLM in the config file for Maestro to function.

### Key Configuration Sections

```json
{
  "version": 1,
  "base_dir": "~/.maestro",
  "playbooks_dir": "playbooks",
  "projects_dir": "projects",
  "reference_dirs": [
    {"path": "data/reference", "mount": "user"},
    {"path": "/opt/standards", "mount": "standards"}
  ],
  "llms": [...],
  "runner": {
    "max_concurrent": 5,
    "max_attempts": 3,
    "retry_delay_seconds": 60,
    "rate_limit": {
      "max_requests": 10,
      "period_seconds": 60
    }
  },
  "logging": { "file": "maestro.log", "level": "INFO" }
}
```

**Optional**: Use `reference_dirs` to mount external directories into the reference library. Each entry has a `path` (filesystem location) and `mount` (prefix in reference library). Files appear as `mount/filename.md`.

## Use

Maestro is intended to be invoked by your API client as a stdio MCP server.

## MCP Tools (76 total)

### System Tools (1)
- `health` - Check system health status

### File Tools (2)
Cross-domain file operations.
- `file_copy` - Copy files within or between domains (reference, playbooks, projects)
- `file_import` - Import external files/directories into a project (preserves symlinks)

### Reference Tools (3) - Read-Only
Built-in documentation embedded in the executable, plus optional user-provided files.
- `reference_list` - List reference files (embedded + user-provided under `user/` prefix)
- `reference_get` - Read a reference file
- `reference_search` - Search reference documentation

**Note**: External files appear under their configured mount prefix (e.g., `user/ISO-27001.pdf`, `standards/NIST.md`). If no `reference_dirs` are configured, only embedded files are available.

### Playbook Tools (12)
User-created collections of reusable procedures and knowledge.

**Playbook Management (4):**
- `playbook_list`, `playbook_create`, `playbook_rename`, `playbook_delete`

**Playbook Files (7):**
- `playbook_file_list`, `playbook_file_get`, `playbook_file_put`
- `playbook_file_append`, `playbook_file_edit`, `playbook_file_rename`, `playbook_file_delete`

**Playbook Search (1):**
- `playbook_search` - Search playbook files by filename or content

### Project Tools (18)
Where active work happens with full project lifecycle support.

**Project Management (6):**
- `project_create` - Create project (use `parent` param for subprojects)
- `project_get` - Get project metadata and tasks
- `project_update` - Update project metadata
- `project_list` - List root projects, or subprojects if `project` param provided
- `project_delete` - Delete project and all contents
- `project_rename` - Rename a project or subproject

**Project Files (9):**
- `project_file_list`, `project_file_get`, `project_file_put`
- `project_file_append`, `project_file_edit`, `project_file_rename`, `project_file_delete`
- `project_file_convert` - Convert files (PDF, DOCX, XLSX) to Markdown
- `project_file_extract` - Extract zip archives within project files
- `project_file_search` - Search project files by filename or content

**Project Logs (2):**
- `project_log_append` - Add entry to project log
- `project_log_get` - Retrieve log entries

**Note**: Project tasks have been reorganized into dedicated Task and Taskset tools (see below).

### Task Tools (10)
Task management for projects with automated runner support.

**Task Operations (7):**
- `task_create` - Create a new task within a task set
- `task_get` - Get a task by UUID or by path and ID
- `task_list` - List tasks, optionally filtered by path, status, or type
- `task_update` - Update task metadata, instructions, or prompts
- `task_delete` - Delete a task by UUID
- `task_run` - Run eligible tasks for a project
- `task_status` - Get current status of tasks in a project

**Task Results (3):**
- `task_results` - Get task execution results
- `task_result_get` - Get a single task result by UUID
- `task_report` - Generate a report from task results

### Taskset Tools (6)
Hierarchical task organization within projects.
- `taskset_create` - Create a new task set at a given path
- `taskset_get` - Get a task set by path, including all its tasks
- `taskset_list` - List task sets in a project
- `taskset_update` - Update a task set's metadata
- `taskset_delete` - Delete a task set and all its tasks
- `taskset_reset` - Reset tasks in a task set to waiting status

### Report Tools (6)
Automated report generation from task results.
- `report_start` - Start a report session for a project
- `report_append` - Append content to a report
- `report_end` - End the report session and clear the prefix
- `report_create` - Generate reports from task results
- `report_list` - List all reports in a project
- `report_read` - Read a report from a project

### LLM Tools (3)
Multi-LLM configuration and dispatch.
- `llm_list` - List configured LLMs with enabled status
- `llm_dispatch` - Send prompt to a configured LLM
- `llm_test` - Test if an LLM is available and responding

### List Tools (15)
Structured item collections available in all three domains.

**List Management (7):**
- `list_list` - List all lists in a domain
- `list_get` - Get full list contents
- `list_get_summary` - Get list metadata with paginated items
- `list_create` - Create a new list
- `list_delete` - Delete a list
- `list_rename` - Rename a list file
- `list_copy` - Copy a list from one location to another

**Item Management (7):**
- `list_item_add` - Add item to a list
- `list_item_update` - Update existing item
- `list_item_remove` - Remove item from list
- `list_item_rename` - Rename item ID
- `list_item_get` - Get single item
- `list_item_search` - Search items with filters

**Task Creation (1):**
- `list_create_tasks` - Create one task per list item

### Supervisor Tools (1)
Advanced task workflow control.
- `supervisor_update` - Allows a supervisor to replace worker response with their own content

## Project Structure

Projects use a directory-based structure:

```
<projects_dir>/
  <project_name>/
    project.json          # Metadata + tasks
    log.txt               # Plain text log
    files/                # Project-specific files (see below)
    lists/                # Structured list files
    results/
      <task_uuid>.json    # Individual task results
    subprojects/
      <subproject_name>/
        project.json
        log.txt
        files/
        lists/
        results/
```

### Project Files Directory

The `files/` directory is automatically created when a project is created. This is where you should place any files you want the LLM to access during the project:

- Documents to analyze (PDFs, text files, etc.)
- Evidence files
- Configuration files
- Data files
- Any other project-specific content

The LLM can access these files using the `project_file_get` tool with the appropriate path relative to the `files/` directory.

**Example**: If you place a file at `~/.maestro/projects/my-project/files/requirements.pdf`, the LLM can access it using:
```
project_file_get(project="my-project", path="requirements.pdf")
```

### Importing External Files

Use `file_import` to copy files from anywhere on the filesystem into a project:

```
file_import(source="/path/to/evidence", project="my-project", recursive=true)
```

This imports files into `files/imported/` and preserves symlinks. Imported files are accessible via:
```
project_file_get(project="my-project", path="imported/document.md")
```

## Runner Workflow

1. Create tasks with `task_create` and configure LLM model ID
2. Configure prompting using four fields (combined when sent to LLM):
   - `instructions_file`: Path to a file containing reusable instructions
   - `instructions_file_source`: Where to load the file from (`project`, `playbook`, or `reference`)
   - `instructions_text`: Inline instructions text
   - `prompt`: Task-specific prompt (appended with `=== TASK PROMPT ===` separator)
3. Optionally enable QA phase for quality control and validation
4. Call `task_run` to execute eligible tasks
5. Use `task_results` or `task_report` to retrieve outputs

**Important**: The runner is **synchronous** - `task_run` blocks until all eligible tasks complete. For large batches, consider using the `type` or `path` parameters to process tasks in groups.

The runner handles rate limiting, retries, and result aggregation automatically. Activity is logged to both the application log and the project log.

## Documentation

- `reference/readme.md` - LLM workflow guidance
- `docs/technical.md` - Full technical reference

## Development

```bash
go test ./...           # Run tests
go fmt ./...            # Format code
go vet ./...            # Check for issues
./build-signed.sh       # Build with code signing (macOS)
./test.sh               # Run comprehensive MCP tool tests
```

**Note**: Use `./build-signed.sh` instead of `go build` on macOS to maintain consistent permissions across rebuilds.

## Architecture

```
main.go           # CLI entry point
config/           # Configuration loading
reference_svc/    # Read-only embedded reference files
playbooks/        # User-created playbook management
projects/         # Project/subproject management with files
tasks/            # Task management
lists/            # Structured list management
runner/           # Automated task execution
llm/              # LLM dispatch client
server/           # MCP server integration
logging/          # Structured logging
global/           # Constants, types, version
```

## Copyright and license

Copyright (c) 2025-2026 by Tenebris Technologies Inc. This software is licensed under the MIT License. Please see LICENSE for details.

## No Warranty (zilch, none, void, nil, null, "", {}, 0x00, 0b00000000, EOF)

THIS SOFTWARE IS PROVIDED “AS IS,” WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE, AND NON-INFRINGEMENT. IN NO EVENT SHALL THE COPYRIGHT HOLDERS OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

Made in Canada