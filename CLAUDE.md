# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Maestro is a Go-based MCP (Model Context Protocol) server that provides general-purpose orchestration capabilities for LLMs. It implements a file-backed library system, project-scoped task management, and multi-LLM dispatch functionality for complex, multi-step analysis workflows.

**Current Status**: ✅ **IMPLEMENTED** - Full working implementation with all core features complete. Ready for testing and deployment.

## Architecture

This is a **single-user, stdio-based MCP server** implemented in Go using the `github.com/mark3labs/mcp-go` library. The system is designed around three core components:

### Core Components
1. **File-backed Library**: Category-organized object store with configurable read-only and writable sections
2. **Project Management System**: Projects with metadata, logs, and hierarchical task lists (main + sublists)
3. **LLM Dispatch**: Multi-LLM configuration and delegation system for specialized work
4. **Lists**: Structured item collections with validated schemas across all domains (projects, playbooks, reference)

### Key Design Principles
- **Persistence & Resumability**: All state written to disk (JSON/text) for LLM session resumption
- **No Domain Assumptions**: General-purpose design suitable for evaluations, reviews, analyses
- **LLM-friendly APIs**: Stable request/response patterns with intuitive naming

## Project Structure

The project follows a flat package structure:
```
main.go                      # CLI entry point with config handling

config/
  config.go                  # Configuration loading and validation
  config_test.go            # Configuration tests

library/
  library.go                 # File-backed object store operations
  operations.go              # Additional library operations

projects/
  projects.go                # Project management (metadata, logs, sublists)

tasks/
  tasks.go                   # Task management (works with projects package)

lists/
  service.go                 # Structured list management across all domains

llm/
  client.go                  # LLM dispatch and HTTP client handling

server/
  server.go                  # MCP server integration and tool registration
  handlers.go                # MCP tool handler implementations

logging/
  logging.go                 # Structured logging setup

global/
  version.go                 # Program name and version constants
  constants.go               # Shared string constants and tool names
```

## Configuration

The server reads JSON configuration from:
1. `--config` CLI flag
2. `MAESTRO_CONFIG` environment variable
3. `~/.maestro/config.json` (default)

Configuration defines:
- **Categories**: Named logical namespaces mapped to directories (reference, projects, docs)
- **Internal**: Directory for project metadata storage
- **LLMs**: Named endpoints with API credentials and usage descriptions

## Key Implementation Requirements

### MCP Tools
- `library.*`: get_item, put_item, append_item, list_items, search_items, rename_item, delete_item, list_categories
- `projects.*`: create, get, update, list, delete, append_log, get_log
- `tasks.*`: create_task, get_task, list_tasks, update_task, append_log, delete_list
- `list_*`: list_list, list_get, list_get_summary, list_create, list_delete, list_rename (6 tools)
- `list_item_*`: list_item_add, list_item_update, list_item_remove, list_item_rename, list_item_get, list_item_search (6 tools)
- `list_create_tasks`: Create tasks from list items
- `llm_list`: List configured LLMs with enabled status
- `llm_dispatch`: Call configured LLMs with context injection (only enabled LLMs)
- `health`: Check system health (base dir, enabled LLMs, etc.)

### Critical Security Features
- Path traversal prevention for all file operations
- Read-only category enforcement
- UTF-8 text-only library (no binary file handling via API)
- API key environment variable resolution

### Concurrency & Reliability
- Per-path mutex for atomic file operations
- Per-project mutex for tasks.json updates
- Atomic writes via temp files + rename
- HTTP connection pooling with retries for LLM calls

## Development Commands

```bash
# Build the server
go build -o bin/maestro .

# Run with custom config
./bin/maestro --config ./example-config.json

# Run tests
go test ./...

# Format code
go fmt ./...

# Vet code
go vet ./...

# Install dependencies
go mod tidy

# Show version
./bin/maestro --version

# Show help
./bin/maestro --help
```

## Testing Strategy

✅ **IMPLEMENTED**:
- Unit tests for config parsing, library operations, task management
- Integration tests with test config and mock LLM endpoints
- Path traversal prevention tests
- Concurrency safety tests for task updates
- Aim for ~80% coverage in core packages

## LLM Orchestration Pattern

The system is designed for LLMs to:
1. Read orchestration guidance from `reference/start.md`
2. Create project metadata and task plans
3. Process items with full coverage (no sampling)
4. Maintain audit trails via per-item outputs + task logs
5. Support resumption across interruptions

## Key Files for Implementation

- `IMPLEMENT.md`: Complete technical specification (74KB document)
- Defines all MCP tool schemas, error handling, and orchestration patterns
- Contains appendices for configuration examples and LLM guidance
- Specifies OpenAI-compatible API integration for LLM dispatch

## Dependencies

✅ **CONFIGURED**:
- `github.com/mark3labs/mcp-go@v0.43.1`: MCP server framework
- Standard library for file operations, HTTP client, JSON handling
- No additional external dependencies required

## Notes

- This project emphasizes defensive programming and atomic operations
- All logs must use format: `YYYY-MM-DD HH:MM:SS [PRIORITY] <message>`
- String constants should be defined in global package to avoid typos
- Error handling must map to appropriate MCP error codes
- Backward compatibility is not required for any aspect of Maestro. This is unreleased code and should be clean. No legacy code, no backwards compat. We want to arrive a nice clean well-implemented product we are proud of.
- Always compile maestro and use the test script. If you are compiling it into bin, make sure the test script is using the binary you think it is. My preference is to compile to the project route instead of the bin directory since there is only one binary.
- DO NOT kill running Maestro instances - they are likely in use by a MCP client.
- Always run test.sh after code changes
- **NEVER delete test projects** until the user has had an opportunity to examine the files. Test projects contain valuable data for debugging and verification.

### STDIO MCP Server Architecture

Maestro is a **STDIO-based MCP server**. This means:
- The MCP client (Claude or others) starts the Maestro process when the session begins
- Communication happens via STDIN/STDOUT pipes
- The running process stays in memory for the entire session
- **Rebuilding the binary does NOT update the running server** - the old code is already loaded in memory
- To use a new version of Maestro, **Claude must restart** (i.e., start a new session) so it spawns a fresh Maestro process with the updated binary