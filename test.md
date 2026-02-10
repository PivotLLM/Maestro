# Maestro Test Suite Documentation

## Overview

The Maestro test suite (`test.sh`) provides comprehensive automated testing for all MCP tools exposed by the server. The test suite ensures that all operations work correctly, data integrity is maintained, and error conditions are handled appropriately.

## Test Infrastructure

### MCPProbe

The test suite uses **MCPProbe**, a command-line tool for testing MCP servers. MCPProbe:

- Connects to Maestro via stdio transport
- Performs the MCP protocol handshake automatically
- Sends tool calls with JSON parameters
- Returns structured output including success/failure status and response data

**Basic usage pattern:**
```bash
$PROBE -stdio "$MAESTRO --config $CONFIG" -env $ENV -call "tool_name" -params '{"param":"value"}'
```

### Test Configuration

The test suite uses a dedicated test configuration (`config-test.json`) that:

- Uses `~/.maestro-test` as the base directory (isolated from production)
- Enables chroot at `~/.maestro-test/data` for security testing
- Places all data (playbooks, projects, logs) within the chroot

This ensures tests run in complete isolation and verifies chroot functionality.

### Test Helpers

The test suite defines three helper functions:

1. **`run_test`** - Executes a tool call and verifies expected output is present
2. **`run_test_expect_fail`** - Executes a tool call expecting it to fail with a specific error
3. **`run_test_capture`** - Executes a tool call and captures output for later use

Each test reports PASS/FAIL with colored output and contributes to a final summary count.

### Test Flow

1. **Fresh Start**: Delete test directory and verify directory creation
2. **Setup**: Clean any leftover test data from previous runs
3. **Execute**: Run tests organized by functional area
4. **Security**: Test chroot boundary enforcement
5. **Cleanup**: Delete test project and verify cascade deletion
6. **Summary**: Report total passed/failed tests

---

## Test Sections

### Section 0: Fresh Start & Directory Creation

Tests that directories are created properly on first run:

- Removes test directory to simulate fresh start
- Runs health check to trigger directory creation
- Verifies chroot directory is created
- Verifies playbooks directory is created
- Verifies projects directory is created
- Verifies log file location is within chroot

### Section 1: Reference Tools (Read-Only, Embedded)

**Tools tested:** `reference_list`, `reference_get`, `reference_search`

**Coverage:**
- List all reference files
- List with prefix filter
- Get specific reference files (readme.md, phase documents, config-example.json)
- Get non-existent reference file (error handling)
- Byte range reading with max_bytes and byte_offset
- Search across reference files
- User-provided reference file handling
- Path traversal security tests

### Section 2: Playbook Tools

**Tools tested:** `playbook_list`, `playbook_create`, `playbook_rename`, `playbook_delete`, `playbook_file_*`, `playbook_search`

**Coverage:**
- Playbook CRUD operations
- File operations (put, get, list, rename, delete)
- File append operations
- File edit operations (single replacement, replace_all, delete text)
- Byte range reading
- Nested file support
- Search across playbooks
- Playbook rename
- Playbook delete
- Path traversal security tests

### Section 3: Project Management Tools

**Tools tested:** `project_create`, `project_get`, `project_update`, `project_list`, `project_delete`, `project_rename`

**Coverage:**
- Project CRUD operations
- Status field (pending, in_progress)
- Invalid name handling
- Project update (title, status, description)
- List filtering by status
- Pagination (limit/offset)
- Project rename
- Project delete

### Section 4: Project File Tools

**Tools tested:** `project_file_list`, `project_file_get`, `project_file_put`, `project_file_append`, `project_file_edit`, `project_file_rename`, `project_file_delete`, `project_file_search`

**Coverage:**
- File creation with content and metadata
- Nested file creation
- File listing with prefix filter
- File retrieval
- Byte range reading
- File update
- File append operations
- File edit operations (replacement, replace_all, delete)
- File rename
- File delete
- Search within project
- Path traversal security tests

### Section 5: Project Log Tools

**Tools tested:** `project_log_append`, `project_log_get`

**Coverage:**
- Append log entries
- Get log entries
- Pagination with limit/offset
- Error handling for non-existent projects

### Section 6: Task Set and Task Tools

**Tools tested:** `taskset_create`, `taskset_get`, `taskset_list`, `taskset_update`, `taskset_delete`, `task_create`, `task_get`, `task_list`, `task_update`, `task_delete`

**Coverage:**
- Task set CRUD operations
- Hierarchical paths (e.g., `analysis`, `analysis/security`)
- Task creation with title, type, and prompt
- Task listing and filtering by type
- Task update
- Task delete
- UUID and path/ID-based task retrieval

### Section 7: Task Execution Tools

**Tools tested:** `task_run`, `task_status`, `task_results`, `task_report`

**Coverage:**
- Task status reporting
- Task execution with path and type filters
- Task results retrieval
- Report generation (markdown, JSON)
- Task set deletion

### Section 8: LLM Tools

**Tools tested:** `llm_list`, `llm_dispatch`

**Coverage:**
- List configured LLMs with enabled status
- Dispatch to disabled LLM (error handling)
- Dispatch to non-existent LLM (error handling)
- Dispatch with empty prompt (error handling)

**Note:** LLM dispatch tests verify error handling; actual API calls require valid API keys.

### Section 9: System Tools

**Tools tested:** `health`, `file_copy`

**Coverage:**
- Health check returns status, base_dir, config_path, enabled_llms, healthy, first_run
- Cross-domain file copy (reference to project, project to playbook, playbook to project)
- File duplication within project
- Copy to read-only reference (error handling)
- Missing parameters (error handling)

### Section 10: Error Handling & Edge Cases

**Coverage:**
- Non-existent resource access
- Invalid names (spaces, leading hyphen, special characters)
- Path traversal prevention in files

### Section 11: List Management Tools

**Tools tested:** `list_list`, `list_create`, `list_get`, `list_get_summary`, `list_rename`, `list_delete`, `list_copy`, `list_item_add`, `list_item_get`, `list_item_update`, `list_item_rename`, `list_item_remove`, `list_item_search`, `list_create_tasks`

**Coverage:**
- List CRUD operations
- List in projects and playbooks
- List item operations (add, get, update, rename, remove)
- Auto-generated item IDs
- Item search by query, source_doc, section
- Complete field and filtering
- Playbook complete restrictions
- List rename
- List copy (within project, to playbook)
- Task creation from list items
- Path traversal security tests

### Section 12: Chroot Security Tests

**Coverage:**
- Project file chroot tests (path traversal, absolute paths, encoded paths, nested traversal)
- Playbook file chroot tests
- Reference file chroot tests
- List chroot tests
- Project/playbook name chroot tests
- File copy chroot tests
- Verification that no files are created outside chroot

### Section 13: Cleanup & Final Verification

**Coverage:**
- Verify task sets cleaned up
- Project deletion and verification
- Cascade deletion verification (files, tasks, log)
- Playbook deletion and verification

---

## Running the Tests

```bash
# Make executable (first time only)
chmod +x test.sh

# Run all tests
./test.sh
```

## Test Data Isolation

The test suite:
- Uses a dedicated test directory (`~/.maestro-test`) separate from production (`~/.maestro`)
- Uses a test configuration file (`config-test.json`) with chroot enabled
- Cleans up the test directory at the start of each run for consistent results
- Uses distinctively named test projects and playbooks to avoid collision
- Does not affect production data

## Exit Codes

- `0` - All tests passed
- `1` - One or more tests failed
