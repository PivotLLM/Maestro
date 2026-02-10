/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PivotLLM/Maestro/global"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		config    *configData
		wantError bool
	}{
		{
			name: "valid config",
			config: &configData{
				Version:      1,
				BaseDir:      "/tmp/maestro",
				PlaybooksDir: "playbooks",
				ProjectsDir:  "projects",
				LLMs: []LLM{
					{
						ID:          "test",
						DisplayName: "Test LLM",
						Type:        "command",
						Command:     "/bin/echo",
						Args:        []string{"{{PROMPT}}"},
						Description: "Test LLM",
					},
				},
			},
			wantError: false,
		},
		{
			name: "invalid version",
			config: &configData{
				Version: 2,
			},
			wantError: true,
		},
		{
			name: "empty LLMs",
			config: &configData{
				Version: 1,
				BaseDir: "/tmp/maestro",
				LLMs:    []LLM{},
			},
			wantError: true,
		},
		{
			name: "valid command LLM with explicit type",
			config: &configData{
				Version: 1,
				BaseDir: "/tmp/maestro",
				LLMs: []LLM{
					{
						ID:          "test-cmd",
						DisplayName: "Test Command LLM",
						Type:        "command",
						Command:     "/bin/echo",
						Args:        []string{"{{PROMPT}}"},
						Description: "Test Command LLM",
					},
				},
			},
			wantError: false,
		},
		{
			name: "valid command LLM",
			config: &configData{
				Version: 1,
				BaseDir: "/tmp/maestro",
				LLMs: []LLM{
					{
						ID:          "test-cmd",
						DisplayName: "Test Command LLM",
						Type:        "command",
						Command:     "/usr/bin/echo",
						Args:        []string{"{{PROMPT}}"},
						Description: "Test command LLM",
					},
				},
			},
			wantError: false,
		},
		{
			name: "command LLM missing PROMPT placeholder",
			config: &configData{
				Version: 1,
				BaseDir: "/tmp/maestro",
				LLMs: []LLM{
					{
						ID:          "test-cmd",
						DisplayName: "Test Command LLM",
						Type:        "command",
						Command:     "/usr/bin/echo",
						Args:        []string{"hello"},
						Description: "Test command LLM",
					},
				},
			},
			wantError: true,
		},
		{
			name: "command LLM missing command",
			config: &configData{
				Version: 1,
				BaseDir: "/tmp/maestro",
				LLMs: []LLM{
					{
						ID:          "test-cmd",
						DisplayName: "Test Command LLM",
						Type:        "command",
						Args:        []string{"{{PROMPT}}"},
						Description: "Test command LLM",
					},
				},
			},
			wantError: true,
		},
		{
			name: "command LLM with stdin option",
			config: &configData{
				Version: 1,
				BaseDir: "/tmp/maestro",
				LLMs: []LLM{
					{
						ID:          "test-stdin",
						DisplayName: "Test Stdin LLM",
						Type:        "command",
						Command:     "/bin/cat",
						Args:        []string{},
						Stdin:       true,
						Description: "Test stdin LLM",
					},
				},
			},
			wantError: false,
		},
		{
			name: "invalid LLM type",
			config: &configData{
				Version: 1,
				BaseDir: "/tmp/maestro",
				LLMs: []LLM{
					{
						ID:          "test",
						DisplayName: "Test LLM",
						Type:        "invalid",
						Description: "Test LLM",
					},
				},
			},
			wantError: true,
		},
		{
			name: "valid default_llm",
			config: &configData{
				Version:    1,
				BaseDir:    "/tmp/maestro",
				DefaultLLM: "claude",
				LLMs: []LLM{
					{
						ID:          "claude",
						DisplayName: "Claude",
						Type:        "command",
						Command:     "/bin/echo",
						Args:        []string{"{{PROMPT}}"},
						Description: "Test LLM",
						Enabled:     true,
					},
					{
						ID:          "gpt4",
						DisplayName: "GPT-4",
						Type:        "command",
						Command:     "/bin/echo",
						Args:        []string{"{{PROMPT}}"},
						Description: "Test LLM",
						Enabled:     true,
					},
				},
			},
			wantError: false,
		},
		{
			name: "default_llm not found in LLMs list",
			config: &configData{
				Version:    1,
				BaseDir:    "/tmp/maestro",
				DefaultLLM: "nonexistent",
				LLMs: []LLM{
					{
						ID:          "claude",
						DisplayName: "Claude",
						Type:        "command",
						Command:     "/bin/echo",
						Args:        []string{"{{PROMPT}}"},
						Description: "Test LLM",
						Enabled:     true,
					},
				},
			},
			wantError: true,
		},
		{
			name: "default_llm is disabled - warns and clears",
			config: &configData{
				Version:    1,
				BaseDir:    "/tmp/maestro",
				DefaultLLM: "claude",
				LLMs: []LLM{
					{
						ID:          "claude",
						DisplayName: "Claude",
						Type:        "command",
						Command:     "/bin/echo",
						Args:        []string{"{{PROMPT}}"},
						Description: "Test LLM",
						Enabled:     false,
					},
				},
			},
			wantError: false, // now just warns and clears default_llm
		},
		{
			name: "empty default_llm is valid",
			config: &configData{
				Version:    1,
				BaseDir:    "/tmp/maestro",
				DefaultLLM: "",
				LLMs: []LLM{
					{
						ID:          "claude",
						DisplayName: "Claude",
						Type:        "command",
						Command:     "/bin/echo",
						Args:        []string{"{{PROMPT}}"},
						Description: "Test LLM",
						Enabled:     true,
					},
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{data: tt.config}
			err := cfg.validate()
			if (err != nil) != tt.wantError {
				t.Errorf("validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestExpandHomePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantHome bool // if true, expects home dir prefix
	}{
		{
			name:     "absolute path",
			path:     "/usr/local/bin",
			wantHome: false,
		},
		{
			name:     "home path",
			path:     "~/documents",
			wantHome: true,
		},
		{
			name:     "relative path",
			path:     "relative/path",
			wantHome: false,
		},
	}

	home, _ := os.UserHomeDir()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandHomePath(tt.path)
			if tt.wantHome {
				expected := filepath.Join(home, "documents")
				if result != expected {
					t.Errorf("expandHomePath(%s) = %s, want %s", tt.path, result, expected)
				}
			} else {
				if result != tt.path {
					t.Errorf("expandHomePath(%s) = %s, want %s", tt.path, result, tt.path)
				}
			}
		})
	}
}

func TestResolvePath(t *testing.T) {
	cfg := &Config{
		data: &configData{
			BaseDir: "/base/dir",
		},
	}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "absolute path",
			path:     "/absolute/path",
			expected: "/absolute/path",
		},
		{
			name:     "relative path",
			path:     "relative/path",
			expected: "/base/dir/relative/path",
		},
		{
			name:     "empty path",
			path:     "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cfg.resolvePath(tt.path)
			if result != tt.expected {
				t.Errorf("resolvePath(%s) = %s, want %s", tt.path, result, tt.expected)
			}
		})
	}
}

func TestGetters(t *testing.T) {
	cfg := &Config{
		data: &configData{
			Version:    1,
			BaseDir:    "/base/dir",
			DefaultLLM: "llm1",
			LLMs: []LLM{
				{ID: "llm1", DisplayName: "LLM 1"},
				{ID: "llm2", DisplayName: "LLM 2"},
			},
			Logging: Logging{
				File:  "/var/log/maestro.log",
				Level: "INFO",
			},
			ValidateLLMsOnStartup: true,
		},
		// Set up fixed categories as they would be built by normalizePaths
		categories: []Category{
			{Name: global.CategoryReference, ReadOnly: true, Embedded: true},
			{Name: global.CategoryPlaybooks, Directory: "/playbooks", ReadOnly: false},
			{Name: global.CategoryProjects, Directory: "/projects", ReadOnly: false},
		},
		playbooksDir: "/playbooks",
		projectsDir:  "/projects",
	}

	// Test Version
	if cfg.Version() != 1 {
		t.Errorf("Version() = %d, want 1", cfg.Version())
	}

	// Test BaseDir
	if cfg.BaseDir() != "/base/dir" {
		t.Errorf("BaseDir() = %s, want /base/dir", cfg.BaseDir())
	}

	// Test Categories (now fixed at 3)
	cats := cfg.Categories()
	if len(cats) != 3 {
		t.Errorf("Categories() length = %d, want 3", len(cats))
	}

	// Test GetCategory for reference
	refCat := cfg.GetCategory(global.CategoryReference)
	if refCat == nil {
		t.Error("GetCategory(reference) returned nil")
	} else {
		if !refCat.ReadOnly {
			t.Error("Reference category should be read-only")
		}
		if !refCat.Embedded {
			t.Error("Reference category should be embedded")
		}
	}

	// Test GetCategory for playbooks
	playbooksCat := cfg.GetCategory(global.CategoryPlaybooks)
	if playbooksCat == nil {
		t.Error("GetCategory(playbooks) returned nil")
	} else if playbooksCat.Directory != "/playbooks" {
		t.Errorf("GetCategory(playbooks).Directory = %s, want /playbooks", playbooksCat.Directory)
	}

	// Test GetCategory not found
	if cfg.GetCategory("nonexistent") != nil {
		t.Error("GetCategory(nonexistent) should return nil")
	}

	// Test PlaybooksDir
	if cfg.PlaybooksDir() != "/playbooks" {
		t.Errorf("PlaybooksDir() = %s, want /playbooks", cfg.PlaybooksDir())
	}

	// Test ProjectsDir
	if cfg.ProjectsDir() != "/projects" {
		t.Errorf("ProjectsDir() = %s, want /projects", cfg.ProjectsDir())
	}

	// Test LLMs
	llms := cfg.LLMs()
	if len(llms) != 2 {
		t.Errorf("LLMs() length = %d, want 2", len(llms))
	}

	// Test GetLLM
	llm := cfg.GetLLM("llm1")
	if llm == nil {
		t.Error("GetLLM(llm1) returned nil")
	} else if llm.ID != "llm1" {
		t.Errorf("GetLLM(llm1).ID = %s, want llm1", llm.ID)
	}

	// Test GetLLM not found
	if cfg.GetLLM("nonexistent") != nil {
		t.Error("GetLLM(nonexistent) should return nil")
	}

	// Test LogFile
	if cfg.LogFile() != "/var/log/maestro.log" {
		t.Errorf("LogFile() = %s, want /var/log/maestro.log", cfg.LogFile())
	}

	// Test LogLevel
	if cfg.LogLevel() != "INFO" {
		t.Errorf("LogLevel() = %s, want INFO", cfg.LogLevel())
	}

	// Test DefaultLLM
	if cfg.DefaultLLM() != "llm1" {
		t.Errorf("DefaultLLM() = %s, want llm1", cfg.DefaultLLM())
	}

	// Test ValidateLLMsOnStartup
	if !cfg.ValidateLLMsOnStartup() {
		t.Error("ValidateLLMsOnStartup() = false, want true")
	}
}

func TestDefaultLLMEmpty(t *testing.T) {
	cfg := &Config{
		data: &configData{
			Version: 1,
			BaseDir: "/base/dir",
			LLMs: []LLM{
				{ID: "llm1", DisplayName: "LLM 1"},
			},
		},
	}

	// Test that DefaultLLM returns empty string when not configured
	if cfg.DefaultLLM() != "" {
		t.Errorf("DefaultLLM() = %s, want empty string", cfg.DefaultLLM())
	}
}

func TestLLMTypeMethods(t *testing.T) {
	tests := []struct {
		name          string
		llm           LLM
		wantType      string
		wantIsCommand bool
	}{
		{
			name:          "default type (empty)",
			llm:           LLM{Type: ""},
			wantType:      "command",
			wantIsCommand: true,
		},
		{
			name:          "explicit command type",
			llm:           LLM{Type: "command"},
			wantType:      "command",
			wantIsCommand: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.llm.GetType(); got != tt.wantType {
				t.Errorf("GetType() = %v, want %v", got, tt.wantType)
			}
			if got := tt.llm.IsCommandType(); got != tt.wantIsCommand {
				t.Errorf("IsCommandType() = %v, want %v", got, tt.wantIsCommand)
			}
		})
	}
}

func TestNormalizePaths(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "maestro-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmpDir)

	cfg := &Config{
		data: &configData{
			Version:      1,
			BaseDir:      tmpDir,
			PlaybooksDir: "custom-playbooks",
			ProjectsDir:  "custom-projects",
			Logging:      Logging{File: "test.log"},
		},
	}

	err = cfg.normalizePaths()
	if err != nil {
		t.Fatalf("normalizePaths() error = %v", err)
	}

	// Check that directories were created
	expectedPlaybooks := filepath.Join(tmpDir, "custom-playbooks")
	expectedProjects := filepath.Join(tmpDir, "custom-projects")

	if cfg.PlaybooksDir() != expectedPlaybooks {
		t.Errorf("PlaybooksDir() = %s, want %s", cfg.PlaybooksDir(), expectedPlaybooks)
	}
	if cfg.ProjectsDir() != expectedProjects {
		t.Errorf("ProjectsDir() = %s, want %s", cfg.ProjectsDir(), expectedProjects)
	}

	// Verify directories exist
	if _, err := os.Stat(expectedPlaybooks); os.IsNotExist(err) {
		t.Error("Playbooks directory was not created")
	}
	if _, err := os.Stat(expectedProjects); os.IsNotExist(err) {
		t.Error("Projects directory was not created")
	}

	// Check that fixed categories were built correctly
	cats := cfg.Categories()
	if len(cats) != 3 {
		t.Fatalf("Expected 3 categories, got %d", len(cats))
	}

	// Verify reference category
	refCat := cfg.GetCategory(global.CategoryReference)
	if refCat == nil {
		t.Fatal("Reference category not found")
	}
	if !refCat.Embedded {
		t.Error("Reference category should be embedded")
	}
	if !refCat.ReadOnly {
		t.Error("Reference category should be read-only")
	}

	// Verify playbooks category
	playbooksCat := cfg.GetCategory(global.CategoryPlaybooks)
	if playbooksCat == nil {
		t.Fatal("Playbooks category not found")
	}
	if playbooksCat.Embedded {
		t.Error("Playbooks category should not be embedded")
	}
	if playbooksCat.ReadOnly {
		t.Error("Playbooks category should not be read-only")
	}
}

func TestNormalizePathsDefaults(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "maestro-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmpDir)

	// Test with empty playbooks_dir and projects_dir (should use defaults)
	cfg := &Config{
		data: &configData{
			Version:      1,
			BaseDir:      tmpDir,
			PlaybooksDir: "", // Empty - should use default
			ProjectsDir:  "", // Empty - should use default
		},
	}

	err = cfg.normalizePaths()
	if err != nil {
		t.Fatalf("normalizePaths() error = %v", err)
	}

	expectedPlaybooks := filepath.Join(tmpDir, global.DefaultPlaybooksDir)
	expectedProjects := filepath.Join(tmpDir, global.DefaultProjectsDir)

	if cfg.PlaybooksDir() != expectedPlaybooks {
		t.Errorf("PlaybooksDir() = %s, want %s", cfg.PlaybooksDir(), expectedPlaybooks)
	}
	if cfg.ProjectsDir() != expectedProjects {
		t.Errorf("ProjectsDir() = %s, want %s", cfg.ProjectsDir(), expectedProjects)
	}
}
