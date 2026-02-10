/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package config

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/PivotLLM/Maestro/global"
)

// setupDefaultConfig creates a default config file from the embedded config-example.json
func (c *Config) setupDefaultConfig(configPath string) error {
	// Read embedded config example
	content, err := c.embeddedFS.ReadFile("docs/ai/config-example.json")
	if err != nil {
		return fmt.Errorf("failed to read embedded config-example.json: %w", err)
	}

	// Ensure parent directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}

	// Write config file
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", configPath, err)
	}

	return nil
}

// Config provides access to application configuration
type Config struct {
	configPath    string                 // resolved path to config file
	data          *configData            // parsed configuration
	firstRun      bool                   // true if config was just created
	categories    []Category             // built from fixed category definitions
	chrootDir     string                 // resolved chroot directory (optional)
	playbooksDir  string                 // resolved playbooks directory
	projectsDir   string                 // resolved projects directory
	referenceDirs []ReferenceDirResolved // resolved external reference directories
	embeddedFS    embed.FS               // embedded reference files
}

// configData holds the parsed configuration (internal)
type configData struct {
	Version               int            `json:"version"`
	BaseDir               string         `json:"base_dir"`
	Chroot                string         `json:"chroot,omitempty"`
	PlaybooksDir          string         `json:"playbooks_dir,omitempty"`
	ProjectsDir           string         `json:"projects_dir,omitempty"`
	ReferenceDirs         []ReferenceDir `json:"reference_dirs,omitempty"`
	DefaultLLM            string         `json:"default_llm,omitempty"`
	LLMs                  []LLM          `json:"llms"`
	Runner                Runner         `json:"runner,omitempty"`
	Logging               Logging        `json:"logging"`
	ValidateLLMsOnStartup bool           `json:"validate_llms_on_startup,omitempty"`
	MarkNonDestructive    bool           `json:"mark_non_destructive,omitempty"`
}

// ReferenceDir represents an external directory to mount in the reference library
type ReferenceDir struct {
	Path  string `json:"path"`  // Filesystem path to the directory
	Mount string `json:"mount"` // Mount point name in reference library (e.g., "user", "standards")
}

// ReferenceDirResolved is a ReferenceDir with the path resolved to absolute
type ReferenceDirResolved struct {
	Path  string // Resolved absolute filesystem path
	Mount string // Mount point name
}

// Category represents a logical storage category (built internally, not from JSON)
type Category struct {
	Name      string
	Directory string
	ReadOnly  bool
	Embedded  bool // true for reference category (served from embedded FS)
}

// LLMTypeCommand LLMType constants
const (
	LLMTypeCommand = "command" // Command-line executable (only supported type for now)
)

// LLM represents an LLM configuration
type LLM struct {
	ID           string `json:"id"`
	DisplayName  string `json:"display_name"`
	Description  string `json:"description"`
	Enabled      bool   `json:"enabled,omitempty"`
	SystemPrompt string `json:"system_prompt,omitempty"`

	// Type specifies the provider type (only "command" supported for now)
	Type string `json:"type,omitempty"`

	// Command is the path to the executable
	Command string `json:"command,omitempty"`
	// Args is the list of arguments; use {{PROMPT}} as placeholder for the prompt (unless Stdin is true)
	Args []string `json:"args,omitempty"`
	// Stdin: if true, prompt is piped to command's stdin instead of using {{PROMPT}} placeholder
	Stdin bool `json:"stdin,omitempty"`

	// RecoveryConfig configures error recovery for this LLM (rate limits, transient errors)
	RecoveryConfig *LLMRecoveryConfig `json:"recovery,omitempty"`
}

// LLMRecoveryConfig configures error recovery for an LLM (rate limits, transient errors)
type LLMRecoveryConfig struct {
	// RateLimitPatterns to detect rate limiting in stdout/stderr (case-insensitive substring match)
	RateLimitPatterns []string `json:"rate_limit_patterns,omitempty"`
	// TestPrompt is a simple prompt used to probe if LLM is available
	TestPrompt string `json:"test_prompt,omitempty"`
	// TestScheduleSeconds is the schedule for testing availability (e.g., [30, 300, 900, 3600])
	TestScheduleSeconds []int `json:"test_schedule_seconds,omitempty"`
	// AbortAfterSeconds is the maximum cumulative time in recovery before aborting the run
	AbortAfterSeconds int `json:"abort_after_seconds,omitempty"`
}

// Logging represents logging configuration
type Logging struct {
	File  string `json:"file"`
	Level string `json:"level"`
}

// Runner represents runner configuration for automated task execution
type Runner struct {
	MaxConcurrent             int           `json:"max_concurrent,omitempty"`
	MaxRounds                 int           `json:"max_rounds,omitempty"`          // Max retry rounds per run (default: 5)
	RoundDelaySeconds         int           `json:"round_delay_seconds,omitempty"` // Delay between processing rounds (default: 0)
	Limits                    global.Limits `json:"limits,omitempty"`              // Default execution limits for tasks
	RetryDelaySeconds         int           `json:"retry_delay_seconds,omitempty"`
	RateLimit                 RateLimit     `json:"rate_limit,omitempty"`
	DefaultDisclaimerTemplate string        `json:"default_disclaimer_template,omitempty"` // Default disclaimer file for reports
}

// RateLimit represents rate limiting configuration
type RateLimit struct {
	MaxRequests   int `json:"max_requests,omitempty"`
	PeriodSeconds int `json:"period_seconds,omitempty"`
}

// Option is a functional option for configuring Config
type Option func(*Config)

// New creates a new Config instance with optional configuration
func New(opts ...Option) *Config {
	c := &Config{}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithConfigPath sets an explicit config file path
func WithConfigPath(path string) Option {
	return func(c *Config) {
		c.configPath = path
	}
}

// WithEmbeddedFS sets the embedded filesystem for reference documentation
func WithEmbeddedFS(efs embed.FS) Option {
	return func(c *Config) {
		c.embeddedFS = efs
	}
}

// Load loads and validates configuration from file
// If the base directory or config file doesn't exist, it creates them from embedded defaults
func (c *Config) Load() error {
	// Resolve config file path
	configPath, err := c.resolveConfigPath()
	if err != nil {
		return fmt.Errorf("failed to resolve config path: %w", err)
	}
	c.configPath = configPath

	// Determine if this is a first-run scenario
	baseDir := c.resolveDefaultBaseDir()
	baseDirExists := dirExists(baseDir)
	configExists := fileExists(configPath)

	// First-run: create base directory
	if !baseDirExists {
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			return fmt.Errorf("failed to create base directory %s: %w", baseDir, err)
		}
	}

	// Create default config if it doesn't exist
	if !configExists {
		c.firstRun = true
		if err := c.setupDefaultConfig(configPath); err != nil {
			return fmt.Errorf("failed to create default config at %s: %w", configPath, err)
		}
		// Continue loading the newly created config instead of returning error
	}

	// Read and parse config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	// First pass: detect unknown fields using strict parsing
	var cfg configData
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		// Check if it's an unknown field error
		errStr := err.Error()
		if strings.Contains(errStr, "unknown field") {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: config file %s: %v\n", configPath, err)
			// Re-parse without strict mode to still load the config
			if err := json.Unmarshal(data, &cfg); err != nil {
				return fmt.Errorf("failed to parse config file %s: %w", configPath, err)
			}
		} else {
			return fmt.Errorf("failed to parse config file %s: %w", configPath, err)
		}
	}

	c.data = &cfg

	// Resolve and validate base_dir
	if err := c.resolveBaseDir(); err != nil {
		return err
	}

	// Validate configuration (skip LLM API key validation on first run)
	if err := c.validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Normalize all paths (resolve relative to base_dir) and create directories
	if err := c.normalizePaths(); err != nil {
		return fmt.Errorf("failed to normalize paths: %w", err)
	}

	return nil
}

// resolveConfigPath determines the config file path using precedence rules
func (c *Config) resolveConfigPath() (string, error) {
	// 1. Explicit path (from WithConfigPath option)
	if c.configPath != "" {
		return c.resolveToAbsolute(c.configPath)
	}

	// 2. Environment variable
	if envPath := os.Getenv(global.ConfigEnvVar); envPath != "" {
		return c.resolveToAbsolute(envPath)
	}

	// 3. Default: base_dir/config.json
	baseDir := c.resolveDefaultBaseDir()
	return filepath.Join(baseDir, global.DefaultConfigFileName), nil
}

// resolveDefaultBaseDir returns the resolved default base directory
func (c *Config) resolveDefaultBaseDir() string {
	return expandHomePath(global.DefaultBaseDir)
}

// resolveBaseDir resolves and validates the base_dir from config
func (c *Config) resolveBaseDir() error {
	if c.data.BaseDir == "" {
		c.data.BaseDir = expandHomePath(global.DefaultBaseDir)
		return nil
	}

	// Expand ~/ if present
	resolved := expandHomePath(c.data.BaseDir)

	// Check if it's absolute
	if !filepath.IsAbs(resolved) {
		// Log warning and use default (we don't have logger here, so just use default)
		// In production, you might want to return an error or use a callback
		_, _ = fmt.Fprintf(os.Stderr, "Warning: base_dir '%s' is not absolute, using default '%s'\n",
			c.data.BaseDir, global.DefaultBaseDir)
		resolved = expandHomePath(global.DefaultBaseDir)
	}

	c.data.BaseDir = resolved
	return nil
}

// resolveToAbsolute converts a path to absolute, expanding ~/ if needed
func (c *Config) resolveToAbsolute(path string) (string, error) {
	expanded := expandHomePath(path)
	if filepath.IsAbs(expanded) {
		return expanded, nil
	}
	return filepath.Abs(expanded)
}

// resolvePath resolves a path relative to base_dir
// - If absolute, returns as-is
// - If starts with ~/, expands home directory
// - Otherwise, joins with base_dir
func (c *Config) resolvePath(path string) string {
	if path == "" {
		return ""
	}

	// Expand ~/ first
	expanded := expandHomePath(path)

	// If absolute, return as-is
	if filepath.IsAbs(expanded) {
		return expanded
	}

	// Relative: join with base_dir
	return filepath.Join(c.data.BaseDir, expanded)
}

// expandHomePath expands ~/ to the user's home directory
func expandHomePath(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		// Can't determine home dir, return path as-is
		return path
	}

	return filepath.Join(home, path[2:])
}

// validate validates the configuration
func (c *Config) validate() error {
	// Check version
	if c.data.Version != 1 {
		if c.data.Version < 1 {
			return fmt.Errorf("config version %d is too old (expected 1)", c.data.Version)
		}
		return fmt.Errorf("config version %d is newer than supported (expected 1)", c.data.Version)
	}

	// Check LLMs - at least one must be defined (but doesn't need to be enabled)
	if len(c.data.LLMs) == 0 {
		return fmt.Errorf("llms cannot be empty - please define at least one LLM")
	}

	llmIDs := make(map[string]bool)
	for _, llm := range c.data.LLMs {
		if llm.ID == "" {
			return fmt.Errorf("LLM id cannot be empty")
		}
		if llm.DisplayName == "" {
			return fmt.Errorf("LLM display_name cannot be empty for LLM %s", llm.ID)
		}
		if llm.Description == "" {
			return fmt.Errorf("LLM description cannot be empty for LLM %s", llm.ID)
		}

		if llmIDs[llm.ID] {
			return fmt.Errorf("duplicate LLM id: %s", llm.ID)
		}
		llmIDs[llm.ID] = true

		// Validate LLM type (only "command" supported for now)
		llmType := llm.Type
		if llmType == "" {
			llmType = LLMTypeCommand // default to command
		}

		if llmType != LLMTypeCommand {
			return fmt.Errorf("invalid LLM type '%s' for LLM %s (only 'command' is supported)", llmType, llm.ID)
		}

		// Validate command LLM
		if llm.Command == "" {
			return fmt.Errorf("LLM command cannot be empty for LLM %s", llm.ID)
		}

		// Verify {{PROMPT}} placeholder exists in args (unless Stdin is true)
		if !llm.Stdin {
			hasPromptPlaceholder := false
			for _, arg := range llm.Args {
				if strings.Contains(arg, "{{PROMPT}}") {
					hasPromptPlaceholder = true
					break
				}
			}
			if !hasPromptPlaceholder {
				return fmt.Errorf("LLM args must contain {{PROMPT}} placeholder for LLM %s (or set stdin: true)", llm.ID)
			}
		}

		// Validate command executable exists (only for enabled LLMs)
		if llm.Enabled {
			// Expand tilde in command path before checking
			expandedCmd := expandHomePath(llm.Command)
			if _, err := exec.LookPath(expandedCmd); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Warning: LLM %s: executable not found: %s - disabling\n", llm.ID, llm.Command)
				// Find the LLM in the slice and disable it
				for i := range c.data.LLMs {
					if c.data.LLMs[i].ID == llm.ID {
						c.data.LLMs[i].Enabled = false
						break
					}
				}
			} else {
				// Store the expanded path for use at runtime
				for i := range c.data.LLMs {
					if c.data.LLMs[i].ID == llm.ID {
						c.data.LLMs[i].Command = expandedCmd
						break
					}
				}
			}
		}
	}

	// Validate default_llm if specified
	if c.data.DefaultLLM != "" {
		// Check that default_llm exists
		if !llmIDs[c.data.DefaultLLM] {
			return fmt.Errorf("default_llm '%s' not found in llms list", c.data.DefaultLLM)
		}

		// Check that default_llm is enabled - if not, clear it and warn
		for _, llm := range c.data.LLMs {
			if llm.ID == c.data.DefaultLLM {
				if !llm.Enabled {
					_, _ = fmt.Fprintf(os.Stderr, "Warning: default_llm '%s' is not enabled - clearing default\n", c.data.DefaultLLM)
					c.data.DefaultLLM = ""
				}
				break
			}
		}
	}

	return nil
}

// normalizePaths resolves all paths to absolute paths and builds fixed categories
func (c *Config) normalizePaths() error {
	// Resolve chroot directory if specified (must be done first)
	if c.data.Chroot != "" {
		chrootDir := expandHomePath(c.data.Chroot)
		if !filepath.IsAbs(chrootDir) {
			return fmt.Errorf("chroot path must be absolute: %s", c.data.Chroot)
		}
		// Resolve symlinks for consistent path comparison
		resolved, err := filepath.EvalSymlinks(chrootDir)
		if err != nil {
			// If path doesn't exist, use the cleaned path
			if os.IsNotExist(err) {
				resolved = filepath.Clean(chrootDir)
			} else {
				return fmt.Errorf("failed to resolve chroot path %s: %w", chrootDir, err)
			}
		}
		c.chrootDir = resolved

		// Create chroot directory if it doesn't exist
		if err := os.MkdirAll(c.chrootDir, 0755); err != nil {
			return fmt.Errorf("failed to create chroot directory at %s: %w", c.chrootDir, err)
		}
	}

	// Resolve playbooks directory (use default if not specified)
	playbooksDir := c.data.PlaybooksDir
	if playbooksDir == "" {
		playbooksDir = global.DefaultPlaybooksDir
	}
	c.playbooksDir = c.resolvePath(playbooksDir)

	// Create playbooks directory if it doesn't exist
	if err := os.MkdirAll(c.playbooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create playbooks directory at %s: %w", c.playbooksDir, err)
	}

	// Resolve projects directory (use default if not specified)
	projectsDir := c.data.ProjectsDir
	if projectsDir == "" {
		projectsDir = global.DefaultProjectsDir
	}
	c.projectsDir = c.resolvePath(projectsDir)

	// Create projects directory if it doesn't exist
	if err := os.MkdirAll(c.projectsDir, 0755); err != nil {
		return fmt.Errorf("failed to create projects directory at %s: %w", c.projectsDir, err)
	}

	// Resolve external reference directories (optional)
	for _, refDir := range c.data.ReferenceDirs {
		if refDir.Path == "" {
			return fmt.Errorf("reference_dirs entry has empty path")
		}
		if refDir.Mount == "" {
			return fmt.Errorf("reference_dirs entry has empty mount for path %s", refDir.Path)
		}

		// Validate mount name (no slashes, no dots)
		if strings.Contains(refDir.Mount, "/") || strings.Contains(refDir.Mount, "\\") {
			return fmt.Errorf("reference_dirs mount cannot contain path separators: %s", refDir.Mount)
		}
		if refDir.Mount == "." || refDir.Mount == ".." {
			return fmt.Errorf("reference_dirs mount cannot be '.' or '..': %s", refDir.Mount)
		}

		resolvedPath := c.resolvePath(refDir.Path)

		// Create reference directory if it doesn't exist
		if err := os.MkdirAll(resolvedPath, 0755); err != nil {
			return fmt.Errorf("failed to create reference directory at %s: %w", resolvedPath, err)
		}

		c.referenceDirs = append(c.referenceDirs, ReferenceDirResolved{
			Path:  resolvedPath,
			Mount: refDir.Mount,
		})
	}

	// Normalize log file path (must be done before chroot validation)
	if c.data.Logging.File != "" {
		c.data.Logging.File = c.resolvePath(c.data.Logging.File)
	}

	// Validate chroot containment if chroot is enabled
	if c.chrootDir != "" {
		if err := c.validateChrootContainment(); err != nil {
			return err
		}
	}

	// Build fixed categories
	c.categories = []Category{
		{
			Name:     global.CategoryReference,
			ReadOnly: true,
			Embedded: true,
			// Directory is empty for embedded category
		},
		{
			Name:      global.CategoryPlaybooks,
			Directory: c.playbooksDir,
			ReadOnly:  false,
			Embedded:  false,
		},
		{
			Name:      global.CategoryProjects,
			Directory: c.projectsDir,
			ReadOnly:  false,
			Embedded:  false,
		},
	}

	return nil
}

// validateChrootContainment verifies that MCP-accessible directories are within the chroot.
// Note: base_dir is NOT required to be within chroot - it may contain internal/config files
// that should not be accessible via MCP tools.
func (c *Config) validateChrootContainment() error {
	// Helper to check if a path is within chroot
	isWithinChroot := func(path, name string) error {
		if path == "" {
			return nil // Empty paths are fine (optional directories)
		}

		// Resolve symlinks for the path being checked
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			if os.IsNotExist(err) {
				// Path doesn't exist yet, use cleaned path
				resolved = filepath.Clean(path)
			} else {
				return fmt.Errorf("failed to resolve %s path %s: %w", name, path, err)
			}
		}

		// Check if resolved path is within or equal to chroot
		if !global.IsPathWithin(c.chrootDir, resolved) && resolved != c.chrootDir {
			return fmt.Errorf("%s directory %s is outside chroot %s", name, resolved, c.chrootDir)
		}
		return nil
	}

	// Note: base_dir is intentionally NOT validated against chroot.
	// It may contain config files and internal data that should not be
	// accessible via MCP tools. Only MCP-accessible directories are validated.

	// Validate playbooks_dir is within chroot
	if err := isWithinChroot(c.playbooksDir, "playbooks_dir"); err != nil {
		return err
	}

	// Validate projects_dir is within chroot
	if err := isWithinChroot(c.projectsDir, "projects_dir"); err != nil {
		return err
	}

	// Note: reference_dirs are NOT validated against chroot.
	// They are read-only directories that cannot be modified via MCP tools,
	// so they don't pose a security risk even if outside the chroot.

	// Note: Log file is intentionally NOT validated against chroot.
	// It's an internal file that should be writable but not accessible via MCP tools.

	return nil
}

// Getter methods

// Version returns the config version
func (c *Config) Version() int {
	return c.data.Version
}

// BaseDir returns the resolved base directory (always absolute)
func (c *Config) BaseDir() string {
	return c.data.BaseDir
}

// Chroot returns the resolved chroot directory (empty if not configured)
func (c *Config) Chroot() string {
	return c.chrootDir
}

// ChrootEnabled returns true if chroot is configured
func (c *Config) ChrootEnabled() bool {
	return c.chrootDir != ""
}

// Categories returns all configured categories
func (c *Config) Categories() []Category {
	return c.categories
}

// GetCategory returns a category by name, or nil if not found
func (c *Config) GetCategory(name string) *Category {
	for i := range c.categories {
		if c.categories[i].Name == name {
			return &c.categories[i]
		}
	}
	return nil
}

// PlaybooksDir returns the resolved playbooks directory (always absolute)
func (c *Config) PlaybooksDir() string {
	return c.playbooksDir
}

// ReferenceDirs returns the resolved external reference directories
func (c *Config) ReferenceDirs() []ReferenceDirResolved {
	return c.referenceDirs
}

// EmbeddedFS returns the embedded filesystem containing reference documentation
func (c *Config) EmbeddedFS() embed.FS {
	return c.embeddedFS
}

// ProjectsDir returns the resolved projects directory (always absolute)
func (c *Config) ProjectsDir() string {
	return c.projectsDir
}

// LLMs returns all configured LLMs
func (c *Config) LLMs() []LLM {
	return c.data.LLMs
}

// GetLLM returns an LLM by ID, or nil if not found
func (c *Config) GetLLM(id string) *LLM {
	for i := range c.data.LLMs {
		if c.data.LLMs[i].ID == id {
			return &c.data.LLMs[i]
		}
	}
	return nil
}

// LogFile returns the resolved log file path (always absolute)
func (c *Config) LogFile() string {
	return c.data.Logging.File
}

// LogLevel returns the configured log level
func (c *Config) LogLevel() string {
	return c.data.Logging.Level
}

// ValidateLLMsOnStartup returns whether LLM validation is enabled
func (c *Config) ValidateLLMsOnStartup() bool {
	return c.data.ValidateLLMsOnStartup
}

// MarkNonDestructive returns true if tools should be marked as non-destructive
func (c *Config) MarkNonDestructive() bool {
	return c.data.MarkNonDestructive
}

// IsFirstRun returns true if this is the first run (config was just created)
func (c *Config) IsFirstRun() bool {
	return c.firstRun
}

// HasEnabledLLM returns true if at least one LLM is enabled
func (c *Config) HasEnabledLLM() bool {
	for _, llm := range c.data.LLMs {
		if llm.Enabled {
			return true
		}
	}
	return false
}

// EnabledLLMs returns only the enabled LLMs
func (c *Config) EnabledLLMs() []LLM {
	var enabled []LLM
	for _, llm := range c.data.LLMs {
		if llm.Enabled {
			enabled = append(enabled, llm)
		}
	}
	return enabled
}

// DefaultLLM returns the default LLM ID, or empty string if not configured
func (c *Config) DefaultLLM() string {
	return c.data.DefaultLLM
}

// ConfigPath returns the path to the loaded config file
func (c *Config) ConfigPath() string {
	return c.configPath
}

// Runner returns the runner configuration with defaults applied
func (c *Config) Runner() Runner {
	r := c.data.Runner
	// Apply defaults for zero values
	if r.MaxConcurrent <= 0 {
		r.MaxConcurrent = global.DefaultMaxConcurrent
	}
	if r.MaxRounds <= 0 {
		r.MaxRounds = global.DefaultMaxRounds
	}
	// Apply defaults for Limits
	r.Limits = r.Limits.WithDefaults()
	if r.RetryDelaySeconds <= 0 {
		r.RetryDelaySeconds = global.DefaultRetryDelaySeconds
	}
	if r.RateLimit.MaxRequests <= 0 {
		r.RateLimit.MaxRequests = global.DefaultRateLimitRequests
	}
	if r.RateLimit.PeriodSeconds <= 0 {
		r.RateLimit.PeriodSeconds = global.DefaultRateLimitPeriod
	}
	return r
}

// LLM methods

// GetSystemPrompt returns the system prompt for the LLM, with a default if not specified
func (llm *LLM) GetSystemPrompt() string {
	if llm.SystemPrompt == "" {
		return "You are a helpful assistant."
	}
	return llm.SystemPrompt
}

// GetType returns the effective LLM type (defaults to "command" if not specified)
func (llm *LLM) GetType() string {
	if llm.Type == "" {
		return LLMTypeCommand
	}
	return llm.Type
}

// IsCommandType returns true if this is a command-line LLM
func (llm *LLM) IsCommandType() bool {
	return llm.GetType() == LLMTypeCommand
}

// Helper functions

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && info.IsDir()
}
