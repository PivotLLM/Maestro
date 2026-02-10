/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

// Package worker implements Claude headless execution for task processing.
// It spawns Claude CLI processes with MCP configuration to execute tasks.
package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/PivotLLM/Maestro/logging"
)

// Worker executes tasks via Claude CLI in headless mode
type Worker struct {
	claudePath     string
	mcpConfigPath  string
	allowedTools   string
	permissionMode string
	maxTurns       int
	timeout        time.Duration
	logger         *logging.Logger
}

// WorkerResult represents the result of a worker execution
//
//goland:noinspection GoNameStartsWithPackageName
type WorkerResult struct {
	Success    bool              `json:"success"`
	Output     string            `json:"output"`
	Error      string            `json:"error,omitempty"`
	ExitCode   int               `json:"exit_code"`
	Duration   time.Duration     `json:"duration"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	TurnCount  int               `json:"turn_count,omitempty"`
	InputCost  int               `json:"input_cost,omitempty"`
	OutputCost int               `json:"output_cost,omitempty"`
}

// ClaudeOutput represents the JSON output from Claude CLI
type ClaudeOutput struct {
	Type          string  `json:"type"`
	Subtype       string  `json:"subtype,omitempty"`
	CostUSD       float64 `json:"cost_usd,omitempty"`
	DurationMS    int64   `json:"duration_ms,omitempty"`
	DurationAPIMS int64   `json:"duration_api_ms,omitempty"`
	IsError       bool    `json:"is_error,omitempty"`
	NumTurns      int     `json:"num_turns,omitempty"`
	Result        string  `json:"result,omitempty"`
	SessionID     string  `json:"session_id,omitempty"`
	TotalCostUSD  float64 `json:"total_cost_usd,omitempty"`
}

// Option configures a Worker
type Option func(*Worker)

// DefaultClaudePath is the default path to the Claude CLI
const DefaultClaudePath = "claude"

// DefaultMaxTurns is the default maximum number of turns
const DefaultMaxTurns = 10

// DefaultTimeout is the default execution timeout
const DefaultTimeout = 5 * time.Minute

// New creates a new Worker with the given options
func New(logger *logging.Logger, opts ...Option) *Worker {
	w := &Worker{
		claudePath:     DefaultClaudePath,
		allowedTools:   "mcp__maestro__*",
		permissionMode: "acceptEdits",
		maxTurns:       DefaultMaxTurns,
		timeout:        DefaultTimeout,
		logger:         logger,
	}

	for _, opt := range opts {
		opt(w)
	}

	return w
}

// WithClaudePath sets the path to the Claude CLI executable
func WithClaudePath(path string) Option {
	return func(w *Worker) {
		w.claudePath = path
	}
}

// WithMCPConfig sets the path to the MCP configuration file
func WithMCPConfig(path string) Option {
	return func(w *Worker) {
		w.mcpConfigPath = path
	}
}

// WithAllowedTools sets the allowed tools pattern
func WithAllowedTools(pattern string) Option {
	return func(w *Worker) {
		w.allowedTools = pattern
	}
}

// WithPermissionMode sets the permission mode for Claude
func WithPermissionMode(mode string) Option {
	return func(w *Worker) {
		w.permissionMode = mode
	}
}

// WithMaxTurns sets the maximum number of turns
func WithMaxTurns(turns int) Option {
	return func(w *Worker) {
		w.maxTurns = turns
	}
}

// WithTimeout sets the execution timeout
func WithTimeout(timeout time.Duration) Option {
	return func(w *Worker) {
		w.timeout = timeout
	}
}

// Execute runs the Claude CLI with the given prompt and returns the result
func (w *Worker) Execute(ctx context.Context, prompt string) (*WorkerResult, error) {
	return w.ExecuteWithOptions(ctx, prompt, nil)
}

// ExecuteOptions provides additional options for execution
type ExecuteOptions struct {
	MaxTurns int
	Timeout  time.Duration
	Metadata map[string]string
}

// ExecuteWithOptions runs the Claude CLI with the given prompt and options
func (w *Worker) ExecuteWithOptions(ctx context.Context, prompt string, opts *ExecuteOptions) (*WorkerResult, error) {
	start := time.Now()

	// Determine settings
	maxTurns := w.maxTurns
	timeout := w.timeout
	if opts != nil {
		if opts.MaxTurns > 0 {
			maxTurns = opts.MaxTurns
		}
		if opts.Timeout > 0 {
			timeout = opts.Timeout
		}
	}

	// Build command arguments
	args := []string{
		"-p", prompt,
		"--output-format", "json",
		"--max-turns", strconv.Itoa(maxTurns),
	}

	if w.mcpConfigPath != "" {
		args = append(args, "--mcp-config", w.mcpConfigPath)
	}

	if w.allowedTools != "" {
		args = append(args, "--allowedTools", w.allowedTools)
	}

	if w.permissionMode != "" {
		args = append(args, "--permission-mode", w.permissionMode)
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create command
	cmd := exec.CommandContext(execCtx, w.claudePath, args...)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Log execution start
	if w.logger != nil {
		w.logger.Infof("Worker: Executing Claude CLI with %d max turns, timeout %v", maxTurns, timeout)
	}

	// Run command
	err := cmd.Run()
	duration := time.Since(start)

	result := &WorkerResult{
		Duration: duration,
		Metadata: make(map[string]string),
	}

	if opts != nil && opts.Metadata != nil {
		for k, v := range opts.Metadata {
			result.Metadata[k] = v
		}
	}

	// Handle execution errors
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			result.Error = "execution timed out"
			result.ExitCode = -1
			if w.logger != nil {
				w.logger.Warnf("Worker: Execution timed out after %v", timeout)
			}
			return result, nil
		}

		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Error = stderr.String()
			if w.logger != nil {
				w.logger.Warnf("Worker: Claude CLI exited with code %d: %s", result.ExitCode, result.Error)
			}
			return result, nil
		}

		// Other error (e.g., command not found)
		result.Error = err.Error()
		result.ExitCode = -1
		if w.logger != nil {
			w.logger.Errorf("Worker: Failed to execute Claude CLI: %v", err)
		}
		return result, fmt.Errorf("failed to execute Claude CLI: %w", err)
	}

	// Parse JSON output
	output := stdout.String()
	result.Output = output
	result.Success = true
	result.ExitCode = 0

	// Try to parse structured output
	claudeOutput, parseErr := parseClaudeOutput(output)
	if parseErr == nil && claudeOutput != nil {
		result.TurnCount = claudeOutput.NumTurns
		if claudeOutput.Result != "" {
			result.Output = claudeOutput.Result
		}
		if claudeOutput.IsError {
			result.Success = false
			result.Error = claudeOutput.Result
		}
	}

	if w.logger != nil {
		w.logger.Infof("Worker: Execution completed in %v, success=%v, output size=%d bytes",
			duration, result.Success, len(result.Output))
	}

	return result, nil
}

// parseClaudeOutput parses the JSON output from Claude CLI
// The output may contain multiple JSON objects, we look for the result type
func parseClaudeOutput(output string) (*ClaudeOutput, error) {
	// Claude outputs JSONL (one JSON object per line)
	lines := strings.Split(strings.TrimSpace(output), "\n")

	var lastResult *ClaudeOutput
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var co ClaudeOutput
		if err := json.Unmarshal([]byte(line), &co); err != nil {
			continue // Skip non-JSON lines
		}

		// Look for result type or keep track of last valid output
		if co.Type == "result" {
			return &co, nil
		}
		lastResult = &co
	}

	if lastResult != nil {
		return lastResult, nil
	}

	return nil, fmt.Errorf("no valid output found")
}

// IsAvailable checks if the Claude CLI is available
func (w *Worker) IsAvailable() bool {
	_, err := exec.LookPath(w.claudePath)
	return err == nil
}

// GetClaudePath returns the configured Claude CLI path
func (w *Worker) GetClaudePath() string {
	return w.claudePath
}

// GetMCPConfigPath returns the configured MCP config path
func (w *Worker) GetMCPConfigPath() string {
	return w.mcpConfigPath
}
