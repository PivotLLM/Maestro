/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package main

import (
	"embed"
	"flag"
	"fmt"
	"os"

	"github.com/PivotLLM/Maestro/config"
	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/logging"
	"github.com/PivotLLM/Maestro/server"
)

// EmbeddedReference contains all files from the docs/ai directory
//
//go:embed docs/ai/* docs/ai/phases/* docs/ai/templates/*
var EmbeddedReference embed.FS

func main() {
	// Top-level panic recovery
	defer func() {
		if rec := recover(); rec != nil {
			_, _ = fmt.Fprintf(os.Stderr, "FATAL PANIC: %v\n", rec)
			os.Exit(2)
		}
	}()

	// Parse command line flags
	var (
		configPath = flag.String("config", "", "Path to configuration file")
		version    = flag.Bool("version", false, "Show version information")
		help       = flag.Bool("help", false, "Show help information")
	)
	flag.Parse()

	// Handle version flag
	if *version {
		fmt.Printf("%s v%s\n", global.ProgramName, global.Version)
		return
	}

	// Handle help flag
	if *help {
		showHelp()
		return
	}

	// Normal MCP server mode - pass embedded FS and optional config path
	opts := []config.Option{config.WithEmbeddedFS(EmbeddedReference)}
	if *configPath != "" {
		opts = append(opts, config.WithConfigPath(*configPath))
	}
	cfg := config.New(opts...)

	// Load and validate configuration
	if err := cfg.Load(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger with config path
	logger, err := logging.New(cfg.LogFile())
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to initialize logging: %v\n", err)
		os.Exit(1)
	}
	defer func(logger *logging.Logger) {
		// Ensure logs are flushed before exit
		_ = logger.Sync()
		_ = logger.Close()
	}(logger)

	// Set log level from config
	logger.SetLevel(cfg.LogLevel())

	// Announce startup
	logger.Infof("%s v%s starting", global.ProgramName, global.Version)

	// Log first-run message
	if cfg.IsFirstRun() {
		logger.Infof("First run detected - created default configuration at %s", cfg.ConfigPath())
		logger.Info("Please edit the configuration to enable LLMs and set API keys")
	}

	// Log warning if no LLMs are enabled
	if !cfg.HasEnabledLLM() {
		logger.Warn("No LLMs are enabled - llm_dispatch will not work until you enable at least one LLM in the configuration")
	}

	// Optional LLM validation on startup
	if cfg.ValidateLLMsOnStartup() {
		logger.Info("LLM validation on startup is enabled, but not yet implemented")
		// TODO: Implement LLM validation if needed
	}

	// Create and start server
	srv, err := server.New(cfg, logger)
	if err != nil {
		logger.Fatalf("Failed to create server: %v", err)
	}

	// Run the server
	if err := srv.Run(); err != nil {
		logger.Fatalf("Server error: %v", err)
	}
}

func showHelp() {
	fmt.Printf(`%s v%s - MCP Server for LLM Orchestration

USAGE:
    %s [OPTIONS]

OPTIONS:
    --config PATH    Path to configuration file
                     (default: $MAESTRO_CONFIG or %s/%s)
    --version        Show version information
    --help          Show this help message

DESCRIPTION:
    Maestro is a Model Context Protocol (MCP) server that provides:

    - Read-only embedded reference documentation
    - Playbooks for reusable knowledge and templates
    - Project-scoped task management system
    - Multi-LLM dispatch capabilities
    - Persistent state for LLM session resumption

CONFIGURATION:
    The server requires a JSON configuration file that defines:

    - playbooks_dir: Directory for playbooks (default: playbooks)
    - projects_dir: Directory for projects (default: projects)
    - internal: Directory for internal project metadata
    - llms: Named endpoints with API credentials and descriptions

    On first run, a default configuration is created in %s.
    Edit the config file to add your LLM API keys.

THREE DOMAINS:
    - Reference: Read-only documentation embedded in the binary
    - Playbooks: User-managed reusable knowledge and templates
    - Projects: Work containers with tasks, files, and logs

FIRST RUN:
    1. Run %s once to create default config
    2. Edit %s/%s to configure LLM API keys
    3. Run %s again to start the server

EXAMPLES:
    # Start with default config
    %s

    # Start with custom config
    %s --config /path/to/config.json

    # Show version
    %s --version

ENVIRONMENT:
    MAESTRO_CONFIG    Path to configuration file (if --config not used)

For more information, use the reference_list and reference_get tools
to access the embedded documentation.
`, global.ProgramName, global.Version,
		global.ProgramName,
		global.DefaultBaseDir, global.DefaultConfigFileName,
		global.DefaultBaseDir,
		global.ProgramName,
		global.DefaultBaseDir, global.DefaultConfigFileName,
		global.ProgramName,
		global.ProgramName,
		global.ProgramName,
		global.ProgramName)
}
