/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package server

import (
	"github.com/PivotLLM/Maestro/pkg/maestro"
	"github.com/PivotLLM/toolspec"
	"context"
	"strings"

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

func (s *Server) registerTools() error {
	provider := &maestro.Provider{}
	deps := toolspec.Deps{
		Cfg: s.config,
		Host: maestro.HostDeps{
			Logger: s.logger,
			Runner: s.runner,
		},
	}
	tools := provider.RegisterTools(deps)

	for _, t := range tools {
		// Convert toolspec tool to MCP tool
		// We can use the readOnly/destructive helpers if we want, or just create directly.
		
		var mcpOpts []mcp.ToolOption
		mcpOpts = append(mcpOpts, mcp.WithDescription(t.Description))
		
		// Use hints if available
		if t.Hints != nil {
			var mcpHints mcp.ToolAnnotation
			if t.Hints.ReadOnly != nil {
				mcpHints.ReadOnlyHint = t.Hints.ReadOnly
			}
			if t.Hints.Destructive != nil {
				mcpHints.DestructiveHint = t.Hints.Destructive
			}
			if t.Hints.OpenWorld != nil {
				mcpHints.OpenWorldHint = t.Hints.OpenWorld
			}
			mcpOpts = append(mcpOpts, mcp.WithToolAnnotation(mcpHints))
		}

		// Build parameters map since mcp.NewTool takes string opts but actually just builds an InputSchema.
		// A cleaner way is to use mcp.NewTool and override the InputSchema.
		tool := mcp.NewTool(t.Name, mcpOpts...)
		
		// Map parameters to MCP JSON Schema Properties
		tool.InputSchema.Type = "object"
		tool.InputSchema.Properties = make(map[string]interface{})
		for _, p := range t.Parameters {
			prop := map[string]interface{}{
				"type":        p.Type,
				"description": p.Description,
			}
			tool.InputSchema.Properties[p.Name] = prop
			if p.Required {
				tool.InputSchema.Required = append(tool.InputSchema.Required, p.Name)
			}
		}

		// Capture the handler
		handler := t.Handler
		
		s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Convert mcp.CallToolRequest to toolspec.ToolCall
			var args map[string]interface{}
			if req.Params.Arguments != nil {
				args, _ = req.Params.Arguments.(map[string]interface{})
			}
			if args == nil {
				args = make(map[string]interface{})
			}

			call := &toolspec.ToolCall{
				Ctx:  ctx,
				Args: args,
			}
			
			res, err := handler(call)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if res.IsError {
				return mcp.NewToolResultError(res.ForLLM), nil
			}
			
			// We try to return it as text or JSON depending on what it looks like.
			// Currently our tools return JSON strings via createJSONResult
			// If it's valid JSON, we should probably return it as JSON or just text.
			// Actually mcp-go expects a formatted result.
			// Let's just return text for simplicity, or try to parse JSON.
			if strings.HasPrefix(res.ForLLM, "{") || strings.HasPrefix(res.ForLLM, "[") {
				return mcp.NewToolResultText(res.ForLLM), nil // Or mcp.NewToolResultJSON ? Wait, NewToolResultText handles string.
			}
			return mcp.NewToolResultText(res.ForLLM), nil
		})
	}
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
// runner.Wait() uses activeRuns (a WaitGroup) which tracks both regular runs and
// dispatches; calling it unconditionally is safe — it returns immediately when nothing
// is in flight. The old IsRunning() guard only checked runningProjects, which dispatch
// tasks never register in, causing Maestro to exit before dispatch callbacks fired.
func (s *Server) waitForRunner() {
	s.logger.Info("Waiting for runner to complete active tasks...")
	s.runner.Wait()
	s.logger.Info("Runner completed all tasks")
}
