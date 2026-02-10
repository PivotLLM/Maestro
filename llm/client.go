/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/PivotLLM/Maestro/config"
	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/library"
	"github.com/PivotLLM/Maestro/logging"
)

// Service provides LLM dispatch functionality
type Service struct {
	config    *config.Config
	logger    *logging.Logger
	library   *library.Service
	llmConfig map[string]*config.LLM
}

// DispatchRequest represents a request to dispatch work to an LLM
type DispatchRequest struct {
	LLMID       string           `json:"llm_id"`
	Prompt      string           `json:"prompt"`
	ContextKeys []string         `json:"context_keys,omitempty"`
	Options     *DispatchOptions `json:"options,omitempty"`
}

// DispatchOptions represents options for LLM dispatch
type DispatchOptions struct {
	ResponseFormat string  `json:"response_format,omitempty"`
	MaxTokens      int     `json:"max_tokens,omitempty"`
	Temperature    float64 `json:"temperature,omitempty"`
	ModelOverride  string  `json:"model_override,omitempty"`
	Timeout        int     `json:"timeout,omitempty"` // Timeout in seconds (min: 60, max: 900, default: 300)
}

// DispatchResult represents the result of an LLM dispatch
// This is returned when the LLM command was invoked (any exit code).
// For infrastructure failures (command not found, permission denied), Dispatch returns (nil, error).
type DispatchResult struct {
	ExitCode     int         `json:"exit_code"`               // Command exit code (0 = success, non-zero = LLM error)
	Stdout       string      `json:"stdout"`                  // Raw stdout (ALWAYS captured)
	Stderr       string      `json:"stderr"`                  // Raw stderr (ALWAYS captured)
	Output       interface{} `json:"output,omitempty"`        // Parsed JSON (if ExitCode==0 and parsing succeeded)
	ResponseSize int         `json:"response_size,omitempty"` // Size of stdout in bytes
}

// NewService creates a new LLM service
func NewService(cfg *config.Config, logger *logging.Logger, libraryService *library.Service) *Service {
	llmConfig := make(map[string]*config.LLM)

	// Build LLM config map
	llms := cfg.LLMs()
	for i := range llms {
		llm := &llms[i]
		llmConfig[llm.ID] = llm
	}

	return &Service{
		config:    cfg,
		logger:    logger,
		library:   libraryService,
		llmConfig: llmConfig,
	}
}

// LLMInfo represents information about a configured LLM
//
//goland:noinspection GoNameStartsWithPackageName
type LLMInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

// LLMExecInfo represents execution details for an LLM (for logging)
//
//goland:noinspection GoNameStartsWithPackageName
type LLMExecInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Mode        string `json:"mode"`         // "command" (only mode currently)
	PromptInput string `json:"prompt_input"` // "stdin" or "args"
}

// LLMListResult represents the result of listing LLMs
//
//goland:noinspection GoNameStartsWithPackageName
type LLMListResult struct {
	LLMs []LLMInfo `json:"llms"`
}

// ListLLMs returns information about all configured LLMs
func (s *Service) ListLLMs() *LLMListResult {
	var llms []LLMInfo

	for _, llm := range s.config.LLMs() {
		llms = append(llms, LLMInfo{
			ID:          llm.ID,
			DisplayName: llm.DisplayName,
			Description: llm.Description,
			Enabled:     llm.Enabled,
		})
	}

	return &LLMListResult{
		LLMs: llms,
	}
}

// GetExecInfo returns execution details for an LLM (for logging)
func (s *Service) GetExecInfo(llmID string) *LLMExecInfo {
	llm, ok := s.llmConfig[llmID]
	if !ok {
		return nil
	}

	mode := llm.Type
	if mode == "" {
		mode = "command"
	}

	promptInput := "args"
	if llm.Stdin {
		promptInput = "stdin"
	}

	return &LLMExecInfo{
		ID:          llm.ID,
		DisplayName: llm.DisplayName,
		Mode:        mode,
		PromptInput: promptInput,
	}
}

// GetLLM returns the full LLM configuration for the given ID
func (s *Service) GetLLM(llmID string) *config.LLM {
	llm, ok := s.llmConfig[llmID]
	if !ok {
		return nil
	}
	return llm
}

// validateRequest validates a dispatch request
func (s *Service) validateRequest(req *DispatchRequest) (*config.LLM, error) {
	if req.LLMID == "" {
		return nil, fmt.Errorf("llm_id is required")
	}

	if req.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	llm, exists := s.llmConfig[req.LLMID]
	if !exists {
		return nil, fmt.Errorf("unknown LLM ID: %s", req.LLMID)
	}

	if !llm.Enabled {
		return nil, fmt.Errorf("LLM %s is not enabled - set enabled: true in config to use it", req.LLMID)
	}

	return llm, nil
}

// loadContextContent loads content from context keys
// Note: Context injection via library is deprecated. Use project files instead.
func (s *Service) loadContextContent(contextKeys []string) (string, error) {
	if len(contextKeys) == 0 {
		return "", nil
	}

	// Library is no longer used - context injection is deprecated
	if s.library == nil {
		return "", fmt.Errorf("context injection is not available - use project files instead")
	}

	var contextParts []string
	totalSize := 0

	for _, key := range contextKeys {
		// Load item content
		item, err := s.library.GetItem(key, true)
		if err != nil {
			return "", fmt.Errorf("failed to load context key %s: %w", key, err)
		}

		// Check size limit
		contentSize := len(item.Content)
		if totalSize+contentSize > global.DefaultContextSizeLimit {
			return "", fmt.Errorf("context size exceeds limit (%d bytes)", global.DefaultContextSizeLimit)
		}

		totalSize += contentSize

		// Add to context
		contextPart := fmt.Sprintf("=== CONTEXT: %s ===\n%s\n", key, item.Content)
		contextParts = append(contextParts, contextPart)
	}

	return strings.Join(contextParts, ""), nil
}

// Dispatch dispatches work to an LLM
func (s *Service) Dispatch(req *DispatchRequest) (*DispatchResult, error) {
	// Validate request
	llm, err := s.validateRequest(req)
	if err != nil {
		return nil, err
	}

	// Extract and validate timeout
	timeout := global.DefaultTimeout
	if req.Options != nil && req.Options.Timeout > 0 {
		timeout, err = global.ValidateTimeout(req.Options.Timeout)
		if err != nil {
			return nil, err
		}
	}

	s.logger.Debugf("Dispatching to LLM %s (timeout: %ds): %s", req.LLMID, timeout, req.Prompt)

	// Load context content
	contextContent, err := s.loadContextContent(req.ContextKeys)
	if err != nil {
		return nil, err
	}

	// Execute command LLM
	result, err := s.callCommandLLM(llm, req, contextContent, timeout)
	if err != nil {
		return nil, err
	}

	s.logger.Debugf("LLM %s response processed successfully", req.LLMID)

	return result, nil
}

// TestLLM sends a simple test prompt to verify LLM availability
// Returns (true, nil) if LLM responds successfully
// Returns (false, nil) if LLM is rate-limited or unavailable (exit code != 0)
// Returns (false, error) if infrastructure error prevents test
func (s *Service) TestLLM(llmID string) (bool, error) {
	llm, exists := s.llmConfig[llmID]
	if !exists {
		return false, fmt.Errorf("unknown LLM ID: %s", llmID)
	}

	if !llm.Enabled {
		return false, fmt.Errorf("LLM %s is not enabled", llmID)
	}

	// Use configured test prompt or default
	testPrompt := "Respond with only the word OK"
	if llm.RecoveryConfig != nil && llm.RecoveryConfig.TestPrompt != "" {
		testPrompt = llm.RecoveryConfig.TestPrompt
	}

	result, err := s.Dispatch(&DispatchRequest{
		LLMID:  llmID,
		Prompt: testPrompt,
		Options: &DispatchOptions{
			Timeout: 60, // Short timeout for test
		},
	})

	if err != nil {
		return false, err // Infrastructure failure
	}

	// Check for rate limit patterns
	if s.IsRateLimited(result, llm) {
		return false, nil // Rate limited
	}

	return result.ExitCode == 0, nil
}

// IsRateLimited checks if a dispatch result indicates rate limiting
func (s *Service) IsRateLimited(result *DispatchResult, llm *config.LLM) bool {
	if result == nil || result.ExitCode == 0 {
		return false
	}

	if llm == nil || llm.RecoveryConfig == nil || len(llm.RecoveryConfig.RateLimitPatterns) == 0 {
		return false
	}

	combined := strings.ToLower(result.Stdout + result.Stderr)
	for _, pattern := range llm.RecoveryConfig.RateLimitPatterns {
		if strings.Contains(combined, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// callCommandLLM executes a command-line LLM
func (s *Service) callCommandLLM(llm *config.LLM, req *DispatchRequest, contextContent string, timeout int) (*DispatchResult, error) {
	// Build the full prompt with context
	var fullPrompt strings.Builder
	if contextContent != "" {
		fullPrompt.WriteString(contextContent)
	}
	fullPrompt.WriteString("=== TASK ===\n")
	fullPrompt.WriteString(req.Prompt)

	promptText := fullPrompt.String()

	// Build args - substitute {{PROMPT}} unless using stdin
	var args []string
	if llm.Stdin {
		// Use args as-is when using stdin
		args = llm.Args
	} else {
		// Substitute {{PROMPT}} placeholder in args
		args = make([]string, len(llm.Args))
		for i, arg := range llm.Args {
			args[i] = strings.ReplaceAll(arg, "{{PROMPT}}", promptText)
		}
	}

	s.logger.Debugf("Executing command: %s %v (stdin: %v)", llm.Command, args, llm.Stdin)

	// Create command with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, llm.Command, args...)

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Pipe prompt to stdin if configured
	if llm.Stdin {
		cmd.Stdin = strings.NewReader(promptText)
	}

	// Run command
	err := cmd.Run()

	// Get exit code
	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	// Get output (always capture stdout and stderr)
	output := strings.TrimSpace(stdout.String())
	stderrOutput := strings.TrimSpace(stderr.String())
	responseSize := len(output)

	// Check for infrastructure failures (command couldn't execute at all)
	if err != nil {
		// Timeout is an infrastructure failure
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			s.logger.Errorf("LLM command timed out after %d seconds", timeout)
			return nil, fmt.Errorf("command timed out after %d seconds", timeout)
		}

		// Check if this is an exec error (command not found, permission denied, etc.)
		// vs the command ran but returned non-zero exit code
		var execErr *exec.ExitError
		if !errors.As(err, &execErr) {
			// Not an ExitError - this is an infrastructure failure (command couldn't start)
			s.logger.Errorf("LLM command infrastructure failure: %v", err)
			return nil, fmt.Errorf("infrastructure failure: %w", err)
		}

		// Command executed but returned non-zero - this is an LLM error, not infrastructure
		// Fall through to return DispatchResult with the exit code
	}

	s.logger.Debugf("LLM command exited with code %d, returned %d bytes, stderr %d bytes", exitCode, responseSize, len(stderrOutput))

	// Build result - always include Stdout and Stderr
	result := &DispatchResult{
		ExitCode:     exitCode,
		Stdout:       output,
		Stderr:       stderrOutput,
		ResponseSize: responseSize,
	}

	// Only parse Output if command succeeded (exit code 0)
	if exitCode != 0 {
		s.logger.Warnf("LLM command exited with non-zero code %d", exitCode)
		return result, nil
	}

	// Handle response format for successful commands
	responseFormat := global.ResponseFormatText
	if req.Options != nil && req.Options.ResponseFormat != "" {
		responseFormat = req.Options.ResponseFormat
	}

	switch responseFormat {
	case global.ResponseFormatJSON:
		// Parse as JSON
		var jsonResult interface{}
		if err := json.Unmarshal([]byte(output), &jsonResult); err != nil {
			// JSON parse failure - still return the result with Stdout populated
			s.logger.Warnf("Failed to parse JSON from command output: %v", err)
			return result, nil
		}
		result.Output = jsonResult

	case global.ResponseFormatText:
		fallthrough
	default:
		result.Output = map[string]interface{}{
			"text": output,
		}
	}

	return result, nil
}
