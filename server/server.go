/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package server

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/PivotLLM/Maestro/config"
	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/lists"
	"github.com/PivotLLM/Maestro/llm"
	"github.com/PivotLLM/Maestro/logging"
	"github.com/PivotLLM/Maestro/playbooks"
	"github.com/PivotLLM/Maestro/projects"
	"github.com/PivotLLM/Maestro/reference"
	"github.com/PivotLLM/Maestro/runner"
	"github.com/PivotLLM/Maestro/tasks"
)

// Server wraps the MCP server with our services
type Server struct {
	config             *config.Config
	logger             *logging.Logger
	reference          *reference.Service
	playbooks          *playbooks.Service
	projects           *projects.Service
	tasks              *tasks.Service
	lists              *lists.Service
	llm                *llm.Service
	runner             *runner.Runner
	mcpServer          *server.MCPServer
	markNonDestructive bool
}

// New creates a new server instance
func New(cfg *config.Config, logger *logging.Logger) (*Server, error) {
	// Convert config reference dirs to reference service format
	var externalDirs []reference.ExternalDir
	for _, refDir := range cfg.ReferenceDirs() {
		externalDirs = append(externalDirs, reference.ExternalDir{
			Path:  refDir.Path,
			Mount: refDir.Mount,
		})
	}

	// Create services
	referenceService := reference.NewService(
		reference.WithEmbeddedFS(cfg.EmbeddedFS()),
		reference.WithExternalDirs(externalDirs),
		reference.WithLogger(logger),
	)
	playbooksService := playbooks.NewService(cfg.PlaybooksDir(), logger)
	projectsService := projects.NewService(cfg, logger)
	tasksService := tasks.NewService(cfg, projectsService, logger)
	listsService := lists.NewService(
		lists.WithProjectsDir(cfg.ProjectsDir()),
		lists.WithPlaybooksDir(cfg.PlaybooksDir()),
		lists.WithEmbeddedFS(cfg.EmbeddedFS()),
		lists.WithLogger(logger),
	)
	llmService := llm.NewService(cfg, logger, nil) // No longer using library for context
	runnerService := runner.New(cfg, logger, nil, playbooksService, referenceService, llmService, tasksService, projectsService)

	// Create MCP server
	mcpServer := server.NewMCPServer(
		global.ProgramName,
		global.Version,
		server.WithToolCapabilities(true),
		server.WithLogging(),
	)

	srv := &Server{
		config:             cfg,
		logger:             logger,
		reference:          referenceService,
		playbooks:          playbooksService,
		projects:           projectsService,
		tasks:              tasksService,
		lists:              listsService,
		llm:                llmService,
		runner:             runnerService,
		mcpServer:          mcpServer,
		markNonDestructive: cfg.MarkNonDestructive(),
	}

	// Register tools
	if err := srv.registerTools(); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}

	return srv, nil
}

// readOnlyTool creates a tool with read-only annotations
// ReadOnly: true, Destructive: false, OpenWorld: false
func (s *Server) readOnlyTool(name string, opts ...mcp.ToolOption) mcp.Tool {
	opts = append(opts, mcp.WithToolAnnotation(mcp.ToolAnnotation{
		ReadOnlyHint:    mcp.ToBoolPtr(true),
		DestructiveHint: mcp.ToBoolPtr(false),
		OpenWorldHint:   mcp.ToBoolPtr(false),
	}))
	return mcp.NewTool(name, opts...)
}

// defaultTool creates a tool with default annotations (non-destructive)
// ReadOnly: false, Destructive: false, OpenWorld: false
func (s *Server) defaultTool(name string, opts ...mcp.ToolOption) mcp.Tool {
	opts = append(opts, mcp.WithToolAnnotation(mcp.ToolAnnotation{
		ReadOnlyHint:    mcp.ToBoolPtr(false),
		DestructiveHint: mcp.ToBoolPtr(false),
		OpenWorldHint:   mcp.ToBoolPtr(false),
	}))
	return mcp.NewTool(name, opts...)
}

// destructiveTool creates a tool with destructive annotations
// ReadOnly: false, Destructive: true (unless markNonDestructive config is set), OpenWorld: false
func (s *Server) destructiveTool(name string, opts ...mcp.ToolOption) mcp.Tool {
	destructive := true
	if s.markNonDestructive {
		destructive = false
	}
	opts = append(opts, mcp.WithToolAnnotation(mcp.ToolAnnotation{
		ReadOnlyHint:    mcp.ToBoolPtr(false),
		DestructiveHint: mcp.ToBoolPtr(destructive),
		OpenWorldHint:   mcp.ToBoolPtr(false),
	}))
	return mcp.NewTool(name, opts...)
}

// registerTools registers all MCP tools
func (s *Server) registerTools() error {
	// Reference tools (read-only, embedded)
	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolReferenceList,
			mcp.WithDescription("List all files in the built-in reference documentation. **Start by reading 'start.md' for orchestration guidance.** The reference section contains guidance on how to use Maestro effectively."),
			mcp.WithString("prefix",
				mcp.Description("Optional path prefix filter"),
			),
		), s.handleReferenceList)

	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolReferenceGet,
			mcp.WithDescription("Read a file from the built-in reference documentation. **New to Maestro? Read 'start.md' first for complete orchestration guidance.** Note: To copy a reference file to a project or playbook, use file_copy instead of get+put - it's more efficient and doesn't load content into the conversation."),
			mcp.WithString("path",
				mcp.Description("Path to the reference file. Start with 'start.md' for orchestration guidance, then explore phase-specific docs in 'phases/'"),
				mcp.Required(),
			),
			mcp.WithNumber("byte_offset",
				mcp.Description("Byte position to start reading from, for chunked reading of large files (default: 0)"),
			),
			mcp.WithNumber("max_bytes",
				mcp.Description("Maximum bytes to return in this chunk, for chunked reading of large files (default: 0 = entire file)"),
			),
		), s.handleReferenceGet)

	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolReferenceSearch,
			mcp.WithDescription("Search reference documentation by filename or content."),
			mcp.WithString("query",
				mcp.Description("Search query string"),
				mcp.Required(),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results"),
			),
			mcp.WithNumber("offset",
				mcp.Description("Number of results to skip"),
			),
		), s.handleReferenceSearch)

	// Playbook tools (user-owned, read-write)
	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolPlaybookList,
			mcp.WithDescription("List all playbooks. Playbooks are user-created collections of reusable knowledge and procedures."),
		), s.handlePlaybookList)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolPlaybookCreate,
			mcp.WithDescription("Create a new playbook."),
			mcp.WithString("name",
				mcp.Description("Playbook name (alphanumeric, hyphens, underscores)"),
				mcp.Required(),
			),
		), s.handlePlaybookCreate)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolPlaybookRename,
			mcp.WithDescription("Rename a playbook."),
			mcp.WithString("name",
				mcp.Description("Current playbook name"),
				mcp.Required(),
			),
			mcp.WithString("new_name",
				mcp.Description("New playbook name"),
				mcp.Required(),
			),
		), s.handlePlaybookRename)

	s.mcpServer.AddTool(
		s.destructiveTool(global.ToolPlaybookDelete,
			mcp.WithDescription("Delete a playbook and all its files."),
			mcp.WithString("name",
				mcp.Description("Playbook name"),
				mcp.Required(),
			),
		), s.handlePlaybookDelete)

	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolPlaybookFileList,
			mcp.WithDescription("List files in a playbook."),
			mcp.WithString("playbook",
				mcp.Description("Playbook name"),
				mcp.Required(),
			),
			mcp.WithString("prefix",
				mcp.Description("Optional path prefix filter"),
			),
		), s.handlePlaybookFileList)

	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolPlaybookFileGet,
			mcp.WithDescription("Read a file from a playbook. Note: To copy a file, use file_copy instead of get+put - it's more efficient and doesn't load content into the conversation."),
			mcp.WithString("playbook",
				mcp.Description("Playbook name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("File path within the playbook"),
				mcp.Required(),
			),
			mcp.WithNumber("byte_offset",
				mcp.Description("Byte position to start reading from, for chunked reading of large files (default: 0)"),
			),
			mcp.WithNumber("max_bytes",
				mcp.Description("Maximum bytes to return in this chunk, for chunked reading of large files (default: 0 = entire file)"),
			),
		), s.handlePlaybookFileGet)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolPlaybookFilePut,
			mcp.WithDescription("Create or update a file in a playbook. Note: To copy a file, use file_copy instead of get+put - it's more efficient and doesn't load content into the conversation."),
			mcp.WithString("playbook",
				mcp.Description("Playbook name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("File path within the playbook"),
				mcp.Required(),
			),
			mcp.WithString("content",
				mcp.Description("File content (text only)"),
				mcp.Required(),
			),
			mcp.WithString("summary",
				mcp.Description("Optional summary description"),
			),
		), s.handlePlaybookFilePut)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolPlaybookFileAppend,
			mcp.WithDescription("Append content to a file in a playbook. If the file exists, content is added to the end. If the file doesn't exist, it is created with the provided content."),
			mcp.WithString("playbook",
				mcp.Description("Playbook name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("File path within the playbook"),
				mcp.Required(),
			),
			mcp.WithString("content",
				mcp.Description("Content to append (text only)"),
				mcp.Required(),
			),
			mcp.WithString("summary",
				mcp.Description("Optional summary description"),
			),
		), s.handlePlaybookFileAppend)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolPlaybookFileEdit,
			mcp.WithDescription("Edit a file in a playbook using search-and-replace. The old_string must exist in the file exactly as specified. If it appears multiple times, use replace_all=true."),
			mcp.WithString("playbook",
				mcp.Description("Playbook name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("File path within the playbook"),
				mcp.Required(),
			),
			mcp.WithString("old_string",
				mcp.Description("Exact text to find and replace (must exist in file)"),
				mcp.Required(),
			),
			mcp.WithString("new_string",
				mcp.Description("Text to replace it with (can be empty string to delete)"),
				mcp.Required(),
			),
			mcp.WithBoolean("replace_all",
				mcp.Description("Replace all occurrences (default: false - fails if old_string appears multiple times)"),
			),
		), s.handlePlaybookFileEdit)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolPlaybookFileRename,
			mcp.WithDescription("Rename or move a file within a playbook."),
			mcp.WithString("playbook",
				mcp.Description("Playbook name"),
				mcp.Required(),
			),
			mcp.WithString("from_path",
				mcp.Description("Current file path"),
				mcp.Required(),
			),
			mcp.WithString("to_path",
				mcp.Description("New file path"),
				mcp.Required(),
			),
		), s.handlePlaybookFileRename)

	s.mcpServer.AddTool(
		s.destructiveTool(global.ToolPlaybookFileDelete,
			mcp.WithDescription("Delete a file from a playbook."),
			mcp.WithString("playbook",
				mcp.Description("Playbook name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("File path within the playbook"),
				mcp.Required(),
			),
		), s.handlePlaybookFileDelete)

	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolPlaybookSearch,
			mcp.WithDescription("Search files in playbooks by filename or content."),
			mcp.WithString("query",
				mcp.Description("Search query string"),
				mcp.Required(),
			),
			mcp.WithString("playbook",
				mcp.Description("Playbook name (optional, searches all if omitted)"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results"),
			),
			mcp.WithNumber("offset",
				mcp.Description("Number of results to skip"),
			),
		), s.handlePlaybookSearch)

	// Project tools
	s.mcpServer.AddTool(
		s.defaultTool(global.ToolProjectCreate,
			mcp.WithDescription("Create a new project with metadata."),
			mcp.WithString("name",
				mcp.Description("Project name (alphanumeric, hyphens, underscores)"),
				mcp.Required(),
			),
			mcp.WithString("title",
				mcp.Description("Human-readable project title"),
				mcp.Required(),
			),
			mcp.WithString("description",
				mcp.Description("Project description"),
			),
			mcp.WithString("context",
				mcp.Description("Global context included in all task prompts (e.g., audit period, customer info)"),
			),
			mcp.WithString("status",
				mcp.Description("Initial status (pending, in_progress, done, cancelled)"),
			),
			mcp.WithString("disclaimer_template",
				mcp.Description("Path to disclaimer file for reports (e.g., 'playbook-name/templates/disclaimer.md') or 'none'. This text appears at the top of generated reports. Use it to disclose AI assistance."),
				mcp.Required(),
			),
		), s.handleProjectCreate)

	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolProjectGet,
			mcp.WithDescription("Get project metadata including tasks."),
			mcp.WithString("name",
				mcp.Description("Project name"),
				mcp.Required(),
			),
		), s.handleProjectGet)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolProjectUpdate,
			mcp.WithDescription("Update project metadata."),
			mcp.WithString("name",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("title",
				mcp.Description("New title (optional)"),
			),
			mcp.WithString("description",
				mcp.Description("New description (optional)"),
			),
			mcp.WithString("context",
				mcp.Description("Global context included in all task prompts (optional)"),
			),
			mcp.WithString("status",
				mcp.Description("New status (optional)"),
			),
			mcp.WithString("disclaimer_template",
				mcp.Description("Path to disclaimer MD file for reports (optional)"),
			),
		), s.handleProjectUpdate)

	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolProjectList,
			mcp.WithDescription("List all projects."),
			mcp.WithString("status",
				mcp.Description("Filter by status (optional)"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of projects to return"),
			),
			mcp.WithNumber("offset",
				mcp.Description("Number of projects to skip"),
			),
		), s.handleProjectList)

	s.mcpServer.AddTool(
		s.destructiveTool(global.ToolProjectDelete,
			mcp.WithDescription("Delete a project and all its contents."),
			mcp.WithString("name",
				mcp.Description("Project name"),
				mcp.Required(),
			),
		), s.handleProjectDelete)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolProjectRename,
			mcp.WithDescription("Rename a project."),
			mcp.WithString("name",
				mcp.Description("Current project name"),
				mcp.Required(),
			),
			mcp.WithString("new_name",
				mcp.Description("New project name"),
				mcp.Required(),
			),
		), s.handleProjectRename)

	// Project file tools
	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolProjectFileList,
			mcp.WithDescription("List files in a project's files directory."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("prefix",
				mcp.Description("Optional path prefix filter"),
			),
		), s.handleProjectFileList)

	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolProjectFileGet,
			mcp.WithDescription("Read a file from a project. Note: To copy a file, use file_copy instead of get+put - it's more efficient and doesn't load content into the conversation."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("File path within the project"),
				mcp.Required(),
			),
			mcp.WithNumber("byte_offset",
				mcp.Description("Byte position to start reading from, for chunked reading of large files (default: 0)"),
			),
			mcp.WithNumber("max_bytes",
				mcp.Description("Maximum bytes to return in this chunk, for chunked reading of large files (default: 0 = entire file)"),
			),
		), s.handleProjectFileGet)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolProjectFilePut,
			mcp.WithDescription("Create or update a file in a project. Note: To copy a file, use file_copy instead of get+put - it's more efficient and doesn't load content into the conversation."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("File path within the project"),
				mcp.Required(),
			),
			mcp.WithString("content",
				mcp.Description("File content (text only)"),
				mcp.Required(),
			),
			mcp.WithString("summary",
				mcp.Description("Optional summary description"),
			),
		), s.handleProjectFilePut)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolProjectFileAppend,
			mcp.WithDescription("Append content to a file in a project. If the file exists, content is added to the end. If the file doesn't exist, it is created with the provided content."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("File path within the project"),
				mcp.Required(),
			),
			mcp.WithString("content",
				mcp.Description("Content to append (text only)"),
				mcp.Required(),
			),
			mcp.WithString("summary",
				mcp.Description("Optional summary description"),
			),
		), s.handleProjectFileAppend)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolProjectFileEdit,
			mcp.WithDescription("Edit a file in a project using search-and-replace. The old_string must exist in the file exactly as specified. If it appears multiple times, use replace_all=true."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("File path within the project"),
				mcp.Required(),
			),
			mcp.WithString("old_string",
				mcp.Description("Exact text to find and replace (must exist in file)"),
				mcp.Required(),
			),
			mcp.WithString("new_string",
				mcp.Description("Text to replace it with (can be empty string to delete)"),
				mcp.Required(),
			),
			mcp.WithBoolean("replace_all",
				mcp.Description("Replace all occurrences (default: false - fails if old_string appears multiple times)"),
			),
		), s.handleProjectFileEdit)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolProjectFileRename,
			mcp.WithDescription("Rename or move a file within a project."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("from_path",
				mcp.Description("Current file path"),
				mcp.Required(),
			),
			mcp.WithString("to_path",
				mcp.Description("New file path"),
				mcp.Required(),
			),
		), s.handleProjectFileRename)

	s.mcpServer.AddTool(
		s.destructiveTool(global.ToolProjectFileDelete,
			mcp.WithDescription("Delete a file from a project."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("File path within the project"),
				mcp.Required(),
			),
		), s.handleProjectFileDelete)

	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolProjectFileSearch,
			mcp.WithDescription("Search files in projects by filename or content."),
			mcp.WithString("query",
				mcp.Description("Search query string"),
				mcp.Required(),
			),
			mcp.WithString("project",
				mcp.Description("Project name (optional, searches all if omitted)"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results"),
			),
			mcp.WithNumber("offset",
				mcp.Description("Number of results to skip"),
			),
		), s.handleProjectFileSearch)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolProjectFileConvert,
			mcp.WithDescription("Convert files in a project to Markdown. Supports PDF, DOCX, and XLSX files."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("Path within project files directory. Must be a file if recursive=false, or a directory if recursive=true."),
				mcp.Required(),
			),
			mcp.WithBoolean("recursive",
				mcp.Description("If true, recursively convert all files in directory. If false, convert single file. Default: false."),
			),
		), s.handleProjectFileConvert)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolProjectFileExtract,
			mcp.WithDescription("Extract a zip archive within a project's files directory. Extracts to a directory with the same name as the archive (without .zip extension) in the same location."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("Path to the .zip file within the project files directory"),
				mcp.Required(),
			),
			mcp.WithBoolean("overwrite",
				mcp.Description("If true, overwrite existing files during extraction. Default: false (skip existing files)."),
			),
			mcp.WithBoolean("convert",
				mcp.Description("If true, convert extracted files (PDF, DOCX, XLSX) to Markdown after extraction. Default: false."),
			),
		), s.handleProjectFileExtract)

	// Project log tools
	s.mcpServer.AddTool(
		s.defaultTool(global.ToolProjectLogAppend,
			mcp.WithDescription("Append a message to a project log."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("message",
				mcp.Description("Log message"),
				mcp.Required(),
			),
		), s.handleProjectLogAppend)

	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolProjectLogGet,
			mcp.WithDescription("Get log entries from a project."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of entries to return"),
			),
			mcp.WithNumber("offset",
				mcp.Description("Number of entries to skip"),
			),
		), s.handleProjectLogGet)

	// LLM tools
	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolLLMList,
			mcp.WithDescription("List all configured LLMs with their IDs, names, and descriptions."),
		), s.handleLLMList)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolLLMDispatch,
			mcp.WithDescription("Send a prompt to a configured LLM."),
			mcp.WithString("llm_id",
				mcp.Description("ID of the LLM to use (see llm_list)"),
				mcp.Required(),
			),
			mcp.WithString("prompt",
				mcp.Description("The prompt to send to the LLM"),
				mcp.Required(),
			),
			mcp.WithNumber("timeout",
				mcp.Description("LLM call timeout in seconds (min: 60, max: 900, default: 300)"),
			),
		), s.handleLLMDispatch)

	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolLLMTest,
			mcp.WithDescription("Test if an LLM is available and responding. Useful for pre-flight checks before starting long-running tasks."),
			mcp.WithString("llm_id",
				mcp.Description("ID of the LLM to test"),
				mcp.Required(),
			),
		), s.handleLLMTest)

	// System tools
	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolHealth,
			mcp.WithDescription("Check Maestro health status. Returns whether the system is healthy and any issues that need to be resolved, such as missing configuration or disabled LLMs."),
		), s.handleHealth)

	// File operations (cross-domain)
	s.mcpServer.AddTool(
		s.defaultTool(global.ToolFileCopy,
			mcp.WithDescription("Copy a file within or between domains (reference, playbooks, projects). More efficient than using get+put as it doesn't load file content into the conversation. Use this instead of get+put when copying files."),
			mcp.WithString("from_path",
				mcp.Description("Source file path"),
				mcp.Required(),
			),
			mcp.WithString("to_path",
				mcp.Description("Destination file path"),
				mcp.Required(),
			),
			mcp.WithString("from_source",
				mcp.Description("Source domain: 'project' (default), 'playbook', or 'reference'"),
			),
			mcp.WithString("from_project",
				mcp.Description("Source project name (required when from_source is 'project')"),
			),
			mcp.WithString("from_playbook",
				mcp.Description("Source playbook name (required when from_source is 'playbook')"),
			),
			mcp.WithString("to_source",
				mcp.Description("Destination domain: 'project' (default) or 'playbook' (reference is read-only)"),
			),
			mcp.WithString("to_project",
				mcp.Description("Destination project name (required when to_source is 'project')"),
			),
			mcp.WithString("to_playbook",
				mcp.Description("Destination playbook name (required when to_source is 'playbook')"),
			),
			mcp.WithString("summary",
				mcp.Description("Optional summary description for the destination file metadata"),
			),
		), s.handleFileCopy)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolFileImport,
			mcp.WithDescription("Import external files into a project's files/imported/ directory. This bypasses the normal chroot restrictions to allow importing files from anywhere on the filesystem. Imported files can then be accessed via project_file_* tools."),
			mcp.WithString("source",
				mcp.Description("Source file or directory path (absolute path on the filesystem)"),
				mcp.Required(),
			),
			mcp.WithString("project",
				mcp.Description("Target project name to import files into"),
				mcp.Required(),
			),
			mcp.WithBoolean("recursive",
				mcp.Description("If true, recursively import directories. Required when source is a directory."),
			),
			mcp.WithBoolean("convert",
				mcp.Description("If true, automatically convert imported files (PDF, DOCX, XLSX) to Markdown after import."),
			),
		), s.handleFileImport)

	// Report tools (read-only domain with controlled write)
	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolReportList,
			mcp.WithDescription("List all reports in a project's reports directory."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
		), s.handleReportList)

	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolReportRead,
			mcp.WithDescription("Read a report from a project. Reports are generated automatically by the runner."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("report",
				mcp.Description("Report filename (e.g., '20251219-1234-Audit-Report.md')"),
				mcp.Required(),
			),
			mcp.WithNumber("byte_offset",
				mcp.Description("Byte position to start reading from (default: 0)"),
			),
			mcp.WithNumber("max_bytes",
				mcp.Description("Maximum bytes to return (default: 0 = entire file)"),
			),
		), s.handleReportRead)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolReportStart,
			mcp.WithDescription("Start a report session for a project. Sets a prefix (e.g., '20251219-1234-Audit-') that all subsequent report_append calls will use."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("title",
				mcp.Description("Report title (used to generate prefix)"),
				mcp.Required(),
			),
			mcp.WithString("intro",
				mcp.Description("Optional introductory paragraph to write to the main report"),
			),
		), s.handleReportStart)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolReportAppend,
			mcp.WithDescription("Append content to a report. Uses the active report prefix. If no session is active, auto-initializes with project name."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("content",
				mcp.Description("Content to append (markdown)"),
				mcp.Required(),
			),
			mcp.WithString("report",
				mcp.Description("Report name suffix (optional - omit for main report, e.g., 'Summary' creates <prefix>Summary.md)"),
			),
		), s.handleReportAppend)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolReportEnd,
			mcp.WithDescription("End the report session and clear the prefix. Future report_append calls will start a new session."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
		), s.handleReportEnd)

	// List Management tools
	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolListList,
			mcp.WithDescription("List all lists in the specified source (project, playbook, or reference)."),
			mcp.WithString("source",
				mcp.Description("Source domain: 'project' (default), 'playbook', or 'reference'"),
			),
			mcp.WithString("project",
				mcp.Description("Project name (required when source is 'project')"),
			),
			mcp.WithString("playbook",
				mcp.Description("Playbook name (required when source is 'playbook')"),
			),
			mcp.WithNumber("offset",
				mcp.Description("Number of results to skip"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results"),
			),
		), s.handleListList)

	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolListGet,
			mcp.WithDescription("Get the full contents of a list including all items."),
			mcp.WithString("list",
				mcp.Description("List name"),
				mcp.Required(),
			),
			mcp.WithString("source",
				mcp.Description("Source domain: 'project' (default), 'playbook', or 'reference'"),
			),
			mcp.WithString("project",
				mcp.Description("Project name (required when source is 'project')"),
			),
			mcp.WithString("playbook",
				mcp.Description("Playbook name (required when source is 'playbook')"),
			),
		), s.handleListGet)

	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolListGetSummary,
			mcp.WithDescription("Get list metadata with paginated items (content truncated to 100 chars)."),
			mcp.WithString("list",
				mcp.Description("List name"),
				mcp.Required(),
			),
			mcp.WithString("source",
				mcp.Description("Source domain: 'project' (default), 'playbook', or 'reference'"),
			),
			mcp.WithString("project",
				mcp.Description("Project name (required when source is 'project')"),
			),
			mcp.WithString("playbook",
				mcp.Description("Playbook name (required when source is 'playbook')"),
			),
			mcp.WithString("complete",
				mcp.Description("Filter by complete status (projects only): 'true', 'false', or '' (no filter)"),
			),
			mcp.WithNumber("offset",
				mcp.Description("Number of items to skip"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of items"),
			),
		), s.handleListGetSummary)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolListCreate,
			mcp.WithDescription("Create a new list. Lists cannot be created in the reference domain."),
			mcp.WithString("list",
				mcp.Description("List name"),
				mcp.Required(),
			),
			mcp.WithString("name",
				mcp.Description("Human-readable list name"),
				mcp.Required(),
			),
			mcp.WithString("source",
				mcp.Description("Source domain: 'project' (default) or 'playbook'"),
			),
			mcp.WithString("project",
				mcp.Description("Project name (required when source is 'project')"),
			),
			mcp.WithString("playbook",
				mcp.Description("Playbook name (required when source is 'playbook')"),
			),
			mcp.WithString("description",
				mcp.Description("List description (optional)"),
			),
		), s.handleListCreate)

	s.mcpServer.AddTool(
		s.destructiveTool(global.ToolListDelete,
			mcp.WithDescription("Delete a list. Lists cannot be deleted from the reference domain."),
			mcp.WithString("list",
				mcp.Description("List name"),
				mcp.Required(),
			),
			mcp.WithString("source",
				mcp.Description("Source domain: 'project' (default) or 'playbook'"),
			),
			mcp.WithString("project",
				mcp.Description("Project name (required when source is 'project')"),
			),
			mcp.WithString("playbook",
				mcp.Description("Playbook name (required when source is 'playbook')"),
			),
		), s.handleListDelete)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolListRename,
			mcp.WithDescription("Rename a list. Lists cannot be renamed in the reference domain."),
			mcp.WithString("list",
				mcp.Description("Current list name"),
				mcp.Required(),
			),
			mcp.WithString("new_list",
				mcp.Description("New list name"),
				mcp.Required(),
			),
			mcp.WithString("source",
				mcp.Description("Source domain: 'project' (default) or 'playbook'"),
			),
			mcp.WithString("project",
				mcp.Description("Project name (required when source is 'project')"),
			),
			mcp.WithString("playbook",
				mcp.Description("Playbook name (required when source is 'playbook')"),
			),
		), s.handleListRename)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolListCopy,
			mcp.WithDescription("Copy a list from one location to another. Supports copying between projects, playbooks, and reference (source only)."),
			mcp.WithString("from_list",
				mcp.Description("Source list name"),
				mcp.Required(),
			),
			mcp.WithString("to_list",
				mcp.Description("Destination list name"),
				mcp.Required(),
			),
			mcp.WithString("from_source",
				mcp.Description("Source domain: 'project' (default), 'playbook', or 'reference'"),
			),
			mcp.WithString("from_project",
				mcp.Description("Source project name (when from_source is 'project')"),
			),
			mcp.WithString("from_playbook",
				mcp.Description("Source playbook name (when from_source is 'playbook')"),
			),
			mcp.WithString("to_source",
				mcp.Description("Destination domain: 'project' (default) or 'playbook'"),
			),
			mcp.WithString("to_project",
				mcp.Description("Destination project name (when to_source is 'project')"),
			),
			mcp.WithString("to_playbook",
				mcp.Description("Destination playbook name (when to_source is 'playbook')"),
			),
			mcp.WithNumber("sample",
				mcp.Description("Randomly sample N items from the source list instead of copying all. Useful for test audits."),
			),
		), s.handleListCopy)

	// List Item Management tools
	s.mcpServer.AddTool(
		s.defaultTool(global.ToolListItemAdd,
			mcp.WithDescription("Add a new item to a list. Item IDs are auto-generated."),
			mcp.WithString("list",
				mcp.Description("List name"),
				mcp.Required(),
			),
			mcp.WithString("title",
				mcp.Description("Short item title (displayed in summaries)"),
				mcp.Required(),
			),
			mcp.WithString("content",
				mcp.Description("Full item content (used for task execution)"),
				mcp.Required(),
			),
			mcp.WithString("source",
				mcp.Description("Source domain: 'project' (default) or 'playbook'"),
			),
			mcp.WithString("project",
				mcp.Description("Project name (required when source is 'project')"),
			),
			mcp.WithString("playbook",
				mcp.Description("Playbook name (required when source is 'playbook')"),
			),
			mcp.WithString("source_doc",
				mcp.Description("Source document reference (optional)"),
			),
			mcp.WithString("section",
				mcp.Description("Section within source document (optional)"),
			),
		), s.handleListItemAdd)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolListItemUpdate,
			mcp.WithDescription("Update an existing item in a list. Only specified fields are updated."),
			mcp.WithString("list",
				mcp.Description("List name"),
				mcp.Required(),
			),
			mcp.WithString("id",
				mcp.Description("Item identifier"),
				mcp.Required(),
			),
			mcp.WithString("source",
				mcp.Description("Source domain: 'project' (default) or 'playbook'"),
			),
			mcp.WithString("project",
				mcp.Description("Project name (required when source is 'project')"),
			),
			mcp.WithString("playbook",
				mcp.Description("Playbook name (required when source is 'playbook')"),
			),
			mcp.WithString("title",
				mcp.Description("New item title (optional)"),
			),
			mcp.WithString("content",
				mcp.Description("New item content (optional)"),
			),
			mcp.WithString("source_doc",
				mcp.Description("New source document reference (optional)"),
			),
			mcp.WithString("section",
				mcp.Description("New section (optional)"),
			),
			mcp.WithBoolean("clear_tags",
				mcp.Description("Set to true to clear all tags"),
			),
			mcp.WithBoolean("complete",
				mcp.Description("Mark item as complete (true) or incomplete (false). Cannot be set to true for playbook lists."),
			),
		), s.handleListItemUpdate)

	s.mcpServer.AddTool(
		s.destructiveTool(global.ToolListItemRemove,
			mcp.WithDescription("Remove an item from a list."),
			mcp.WithString("list",
				mcp.Description("List name"),
				mcp.Required(),
			),
			mcp.WithString("id",
				mcp.Description("Item identifier"),
				mcp.Required(),
			),
			mcp.WithString("source",
				mcp.Description("Source domain: 'project' (default) or 'playbook'"),
			),
			mcp.WithString("project",
				mcp.Description("Project name (required when source is 'project')"),
			),
			mcp.WithString("playbook",
				mcp.Description("Playbook name (required when source is 'playbook')"),
			),
		), s.handleListItemRemove)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolListItemRename,
			mcp.WithDescription("Rename an item's ID. The new ID must be unique within the list."),
			mcp.WithString("list",
				mcp.Description("List name"),
				mcp.Required(),
			),
			mcp.WithString("id",
				mcp.Description("Current item identifier"),
				mcp.Required(),
			),
			mcp.WithString("new_id",
				mcp.Description("New item identifier"),
				mcp.Required(),
			),
			mcp.WithString("source",
				mcp.Description("Source domain: 'project' (default) or 'playbook'"),
			),
			mcp.WithString("project",
				mcp.Description("Project name (required when source is 'project')"),
			),
			mcp.WithString("playbook",
				mcp.Description("Playbook name (required when source is 'playbook')"),
			),
		), s.handleListItemRename)

	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolListItemGet,
			mcp.WithDescription("Get a single item from a list by ID."),
			mcp.WithString("list",
				mcp.Description("List name"),
				mcp.Required(),
			),
			mcp.WithString("id",
				mcp.Description("Item identifier"),
				mcp.Required(),
			),
			mcp.WithString("source",
				mcp.Description("Source domain: 'project' (default), 'playbook', or 'reference'"),
			),
			mcp.WithString("project",
				mcp.Description("Project name (required when source is 'project')"),
			),
			mcp.WithString("playbook",
				mcp.Description("Playbook name (required when source is 'playbook')"),
			),
		), s.handleListItemGet)

	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolListItemSearch,
			mcp.WithDescription("Search for items in a list. Query matches id or content (case-insensitive). All filters are ANDed."),
			mcp.WithString("list",
				mcp.Description("List name"),
				mcp.Required(),
			),
			mcp.WithString("source",
				mcp.Description("Source domain: 'project' (default), 'playbook', or 'reference'"),
			),
			mcp.WithString("project",
				mcp.Description("Project name (required when source is 'project')"),
			),
			mcp.WithString("playbook",
				mcp.Description("Playbook name (required when source is 'playbook')"),
			),
			mcp.WithString("query",
				mcp.Description("Search query (matches id or content, case-insensitive)"),
			),
			mcp.WithString("source_doc",
				mcp.Description("Filter by source document (exact match)"),
			),
			mcp.WithString("section",
				mcp.Description("Filter by section (exact match)"),
			),
			mcp.WithString("complete",
				mcp.Description("Filter by complete status (projects only): 'true', 'false', or '' (no filter)"),
			),
			mcp.WithNumber("offset",
				mcp.Description("Number of results to skip"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results"),
			),
		), s.handleListItemSearch)

	// List Task Creation tool
	s.mcpServer.AddTool(
		s.defaultTool(global.ToolListCreateTasks,
			mcp.WithDescription("Create tasks from list items. Creates one task per item with item context appended to the prompt."),
			mcp.WithString("list",
				mcp.Description("List name"),
				mcp.Required(),
			),
			mcp.WithString("project",
				mcp.Description("Target project for created tasks"),
				mcp.Required(),
			),
			mcp.WithString("type",
				mcp.Description("Task type for all created tasks"),
				mcp.Required(),
			),
			mcp.WithString("list_source",
				mcp.Description("Source domain for the list: 'project' (default), 'playbook', or 'reference'"),
			),
			mcp.WithString("list_project",
				mcp.Description("Project containing the list (when list_source is 'project')"),
			),
			mcp.WithString("list_playbook",
				mcp.Description("Playbook containing the list (when list_source is 'playbook')"),
			),
			mcp.WithString("path",
				mcp.Description("Task set path for created tasks (e.g., 'analysis', 'analysis/code')"),
			),
			mcp.WithString("title_template",
				mcp.Description("Task title template. Use {{title}} for item title, {{id}} for item ID. Default: '{{title}}'"),
			),
			mcp.WithNumber("priority",
				mcp.Description("Task priority for all created tasks"),
			),
			mcp.WithString("llm_model_id",
				mcp.Description("LLM model ID for runner execution"),
			),
			mcp.WithString("instructions_file",
				mcp.Description("Path to instructions file. For 'playbook' source, path MUST start with playbook name: 'playbook-name/path/file.md'. For 'project' source (uses target project) or 'reference' source, use relative path: 'path/file.md'."),
			),
			mcp.WithString("instructions_file_source",
				mcp.Description("Source type for instructions_file: 'project' (default - uses project's files directory), 'playbook' (uses playbook files), or 'reference' (uses embedded reference docs)."),
			),
			mcp.WithString("instructions_text",
				mcp.Description("Inline instructions text"),
			),
			mcp.WithString("prompt",
				mcp.Description("Base prompt (item context will be appended)"),
			),
			mcp.WithBoolean("qa_enabled",
				mcp.Description("Enable QA phase for this task"),
			),
			mcp.WithString("qa_instructions_file",
				mcp.Description("QA instructions file path"),
			),
			mcp.WithString("qa_instructions_file_source",
				mcp.Description("Source for QA instructions_file"),
			),
			mcp.WithString("qa_instructions_text",
				mcp.Description("QA inline instructions text"),
			),
			mcp.WithString("qa_prompt",
				mcp.Description("QA direct prompt text"),
			),
			mcp.WithString("qa_llm_model_id",
				mcp.Description("QA LLM model ID"),
			),
			mcp.WithNumber("sample",
				mcp.Description("Randomly sample N items from the list instead of using all items. Useful for test audits."),
			),
			mcp.WithBoolean("parallel",
				mcp.Description("Enable parallel task execution. Set to true if tasks are independent and can run concurrently for efficiency. Default: false (sequential)."),
			),
		), s.handleListCreateTasks)

	// Task Set CRUD tools
	s.mcpServer.AddTool(
		s.defaultTool(global.ToolTaskSetCreate,
			mcp.WithDescription("Create a new task set at a given path within a project."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("Task set path (e.g., 'analysis', 'analysis/code', max 3 levels)"),
				mcp.Required(),
			),
			mcp.WithString("title",
				mcp.Description("Task set title"),
				mcp.Required(),
			),
			mcp.WithString("description",
				mcp.Description("Task set description"),
			),
			mcp.WithBoolean("parallel",
				mcp.Description("Enable parallel task execution. Set to true if tasks are independent and can run concurrently for efficiency. Default: false (sequential)."),
			),
			mcp.WithString("worker_response_template",
				mcp.Description("Path to JSON schema file for worker responses"),
			),
			mcp.WithString("worker_report_template",
				mcp.Description("Path to markdown template for worker reports"),
			),
			mcp.WithString("qa_response_template",
				mcp.Description("Path to JSON schema file for QA responses"),
			),
			mcp.WithString("qa_report_template",
				mcp.Description("Path to markdown template for QA reports"),
			),
		), s.handleTaskSetCreate)

	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolTaskSetGet,
			mcp.WithDescription("Get a task set by path, including all its tasks."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("Task set path"),
				mcp.Required(),
			),
		), s.handleTaskSetGet)

	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolTaskSetList,
			mcp.WithDescription("List task sets in a project, optionally filtered by path prefix."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("Path prefix to filter by (optional)"),
			),
		), s.handleTaskSetList)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolTaskSetUpdate,
			mcp.WithDescription("Update a task set's metadata."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("Task set path"),
				mcp.Required(),
			),
			mcp.WithString("title",
				mcp.Description("New title (optional)"),
			),
			mcp.WithString("description",
				mcp.Description("New description (optional)"),
			),
			mcp.WithString("parallel",
				mcp.Description("Set parallel execution: 'true' or 'false'. Use true if tasks are independent (optional)."),
			),
			mcp.WithString("worker_response_template",
				mcp.Description("Path to JSON schema file for worker responses"),
			),
			mcp.WithString("worker_report_template",
				mcp.Description("Path to markdown template for worker reports"),
			),
			mcp.WithString("qa_response_template",
				mcp.Description("Path to JSON schema file for QA responses"),
			),
			mcp.WithString("qa_report_template",
				mcp.Description("Path to markdown template for QA reports"),
			),
		), s.handleTaskSetUpdate)

	s.mcpServer.AddTool(
		s.destructiveTool(global.ToolTaskSetDelete,
			mcp.WithDescription("Delete a task set and all its tasks."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("Task set path"),
				mcp.Required(),
			),
		), s.handleTaskSetDelete)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolTaskSetReset,
			mcp.WithDescription("Reset tasks in a task set to waiting status. Requires 'mode' parameter to specify which tasks to reset."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("Task set path"),
				mcp.Required(),
			),
			mcp.WithString("mode",
				mcp.Description("Reset mode: 'all' to reset all tasks, 'failed' to reset only failed tasks"),
				mcp.Required(),
			),
			mcp.WithBoolean("delete_results",
				mcp.Description("Delete results files from disk (default: true)"),
			),
			mcp.WithBoolean("end_report",
				mcp.Description("End the current report session (default: false). When true, response includes reminder to call report_start before running tasks."),
			),
		), s.handleTaskSetReset)

	// Task CRUD tools
	s.mcpServer.AddTool(
		s.defaultTool(global.ToolTaskCreate,
			mcp.WithDescription("Create a new task within a task set. At least one prompt field is required."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("Task set path"),
				mcp.Required(),
			),
			mcp.WithString("title",
				mcp.Description("Task title"),
				mcp.Required(),
			),
			mcp.WithString("type",
				mcp.Description("Task type for filtering/grouping"),
			),
			mcp.WithString("instructions_file",
				mcp.Description("Path to instructions file"),
			),
			mcp.WithString("instructions_file_source",
				mcp.Description("Source for instructions_file: 'project', 'playbook', or 'reference'"),
			),
			mcp.WithString("instructions_text",
				mcp.Description("Inline instructions text"),
			),
			mcp.WithString("prompt",
				mcp.Description("Direct prompt text"),
			),
			mcp.WithString("llm_model_id",
				mcp.Description("LLM model ID for execution"),
			),
			mcp.WithBoolean("qa_enabled",
				mcp.Description("Enable QA phase for this task"),
			),
			mcp.WithString("qa_instructions_file",
				mcp.Description("QA instructions file path"),
			),
			mcp.WithString("qa_instructions_file_source",
				mcp.Description("Source for QA instructions_file"),
			),
			mcp.WithString("qa_instructions_text",
				mcp.Description("QA inline instructions text"),
			),
			mcp.WithString("qa_prompt",
				mcp.Description("QA direct prompt text"),
			),
			mcp.WithString("qa_llm_model_id",
				mcp.Description("QA LLM model ID"),
			),
			mcp.WithNumber("qa_max_iterations",
				mcp.Description("Maximum QA retry iterations"),
			),
		), s.handleTaskCreate)

	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolTaskGet,
			mcp.WithDescription("Get a task by UUID or by path and ID."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("uuid",
				mcp.Description("Task UUID (preferred)"),
			),
			mcp.WithString("path",
				mcp.Description("Task set path (required with id)"),
			),
			mcp.WithNumber("id",
				mcp.Description("Task ID within task set (required with path)"),
			),
		), s.handleTaskGet)

	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolTaskList,
			mcp.WithDescription("List tasks, optionally filtered by path, status, or type."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("Task set path to list tasks from (optional, lists all if empty)"),
			),
			mcp.WithString("status",
				mcp.Description("Filter by work status: waiting, processing, done, failed"),
			),
			mcp.WithString("type",
				mcp.Description("Filter by task type"),
			),
			mcp.WithNumber("offset",
				mcp.Description("Number of tasks to skip"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of tasks to return"),
			),
		), s.handleTaskList)

	s.mcpServer.AddTool(
		s.defaultTool(global.ToolTaskUpdate,
			mcp.WithDescription("Update a task's metadata, instructions, or prompts."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("uuid",
				mcp.Description("Task UUID"),
				mcp.Required(),
			),
			mcp.WithString("title",
				mcp.Description("New title (optional)"),
			),
			mcp.WithString("type",
				mcp.Description("New type (optional)"),
			),
			mcp.WithString("work_status",
				mcp.Description("New work status (optional)"),
			),
			// Work execution fields
			mcp.WithString("instructions_file",
				mcp.Description("Path to instructions file (validated before update)"),
			),
			mcp.WithString("instructions_file_source",
				mcp.Description("Source for instructions_file: 'project', 'playbook', or 'reference'"),
			),
			mcp.WithString("instructions_text",
				mcp.Description("Inline instructions text"),
			),
			mcp.WithString("prompt",
				mcp.Description("Direct prompt text"),
			),
			mcp.WithString("llm_model_id",
				mcp.Description("LLM model ID for task execution"),
			),
			// QA execution fields
			mcp.WithString("qa_instructions_file",
				mcp.Description("Path to QA instructions file (validated before update)"),
			),
			mcp.WithString("qa_instructions_file_source",
				mcp.Description("Source for QA instructions_file: 'project', 'playbook', or 'reference'"),
			),
			mcp.WithString("qa_instructions_text",
				mcp.Description("QA inline instructions text"),
			),
			mcp.WithString("qa_prompt",
				mcp.Description("QA direct prompt text"),
			),
			mcp.WithString("qa_llm_model_id",
				mcp.Description("QA LLM model ID"),
			),
		), s.handleTaskUpdate)

	s.mcpServer.AddTool(
		s.destructiveTool(global.ToolTaskDelete,
			mcp.WithDescription("Delete a task by UUID."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("uuid",
				mcp.Description("Task UUID"),
				mcp.Required(),
			),
		), s.handleTaskDelete)

	// Task Execution tools
	s.mcpServer.AddTool(
		s.defaultTool(global.ToolTaskRun,
			mcp.WithDescription("Run eligible tasks for a project. Tasks in 'waiting' or 'retry' status are executed. Returns immediately with count of tasks queued."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("Task set path prefix to filter (optional)"),
			),
			mcp.WithString("type",
				mcp.Description("Filter by task type (optional)"),
			),
			mcp.WithString("parallel",
				mcp.Description("Override taskset parallel setting: 'true' or 'false' (optional, defaults to taskset setting)"),
			),
			mcp.WithNumber("timeout",
				mcp.Description("LLM call timeout in seconds (60-1200, default: 600)"),
			),
			mcp.WithBoolean("wait",
				mcp.Description("Wait for all tasks to complete before returning (default: false). Useful for scripting."),
			),
		), s.handleTaskRun)

	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolTaskStatus,
			mcp.WithDescription("Get current status of tasks in a project, including counts by status and whether a run is in progress."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("Task set path prefix to filter (optional)"),
			),
			mcp.WithString("type",
				mcp.Description("Filter by task type (optional)"),
			),
		), s.handleTaskStatus)

	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolTaskResults,
			mcp.WithDescription("Get task execution results. Returns completed task results with their outputs."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("Task set path prefix to filter (optional)"),
			),
			mcp.WithNumber("task_id",
				mcp.Description("Specific task ID to get result for (optional)"),
			),
			mcp.WithString("status",
				mcp.Description("Filter by status: done, failed (optional)"),
			),
			mcp.WithNumber("offset",
				mcp.Description("Number of results to skip (default: 0)"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results (default: 50)"),
			),
			mcp.WithBoolean("summary",
				mcp.Description("If true, returns only task_id, task_uuid, task_title, work_status (default: false)"),
			),
			mcp.WithString("worker_pattern",
				mcp.Description("Regex pattern to match against worker response (optional)"),
			),
			mcp.WithString("qa_pattern",
				mcp.Description("Regex pattern to match against QA response (optional). If both patterns provided, uses OR logic."),
			),
		), s.handleTaskResults)

	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolTaskResultGet,
			mcp.WithDescription("Get a single task result by UUID. Returns worker/QA responses without history or prompts. Includes worker_response_template for supervisor updates."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("uuid",
				mcp.Description("Task UUID"),
				mcp.Required(),
			),
		), s.handleTaskResultGet)

	s.mcpServer.AddTool(
		s.readOnlyTool(global.ToolTaskReport,
			mcp.WithDescription("Generate a report from task results. Supports filtering and multiple output formats."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("Task set path prefix to filter (optional)"),
			),
			mcp.WithString("status",
				mcp.Description("Filter by work status (optional)"),
			),
			mcp.WithString("type",
				mcp.Description("Filter by task type (optional)"),
			),
			mcp.WithBoolean("qa_passed",
				mcp.Description("Filter by QA passed status (optional)"),
			),
			mcp.WithString("qa_severity",
				mcp.Description("Filter by QA severity (optional)"),
			),
			mcp.WithString("format",
				mcp.Description("Output format: markdown (default) or json"),
			),
			mcp.WithString("output",
				mcp.Description("File path to save report (optional)"),
			),
		), s.handleTaskReport)

	// Supervisor tools
	s.mcpServer.AddTool(
		mcp.NewTool(global.ToolSupervisorUpdate,
			mcp.WithDescription("Allows a supervisor to replace the worker response with their own content. The response must pass template validation. History is append-only."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("uuid",
				mcp.Description("Task UUID"),
				mcp.Required(),
			),
			mcp.WithString("response",
				mcp.Description("Supervisor's replacement response (must match worker_response_template if defined)"),
				mcp.Required(),
			),
		), s.handleSupervisorUpdate)

	// Report generation tool
	s.mcpServer.AddTool(
		mcp.NewTool(global.ToolReportCreate,
			mcp.WithDescription("Generate reports from task results. Uses the same report generation logic as the runner. Supports optional path filtering."),
			mcp.WithString("project",
				mcp.Description("Project name"),
				mcp.Required(),
			),
			mcp.WithString("path",
				mcp.Description("Task set path prefix to filter (optional)"),
			),
		), s.handleReportCreate)

	return nil
}

// Run starts the MCP server with graceful shutdown
func (s *Server) Run() error {
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		err := server.ServeStdio(s.mcpServer)
		// ServeStdio returns when stdin is closed (EOF) or on error
		errChan <- err
	}()

	s.logger.Infof("MCP server started successfully")

	// Wait for shutdown signal, stdin close, or error
	select {
	case <-sigChan:
		s.logger.Info("Shutdown signal received")
		s.waitForRunner()
		s.logger.Info("Server stopped")
		// Flush logs before exiting
		if err := s.logger.Sync(); err != nil {
			s.logger.Warnf("Failed to flush logs on shutdown: %v", err)
		}
		return nil

	case err := <-errChan:
		if err != nil {
			s.logger.Errorf("Server error: %v", err)
			// Still wait for runner to complete before exiting on error
			s.waitForRunner()
			return fmt.Errorf("server error: %w", err)
		}
		// nil error means stdin was closed (EOF) - normal exit
		s.logger.Info("Connection closed")
		s.waitForRunner()
		s.logger.Info("Server exiting")
		return nil
	}
}

// waitForRunner waits for any active runner tasks to complete before shutdown.
// This ensures tasks complete and reports are written even if the calling process exits.
func (s *Server) waitForRunner() {
	if s.runner.IsRunning() {
		s.logger.Info("Waiting for runner to complete active tasks...")
		s.runner.Wait()
		s.logger.Info("Runner completed all tasks")
	}
}
