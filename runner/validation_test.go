/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package runner

import (
	"strings"
	"testing"
)

func TestValidateTaskInstructions(t *testing.T) {
	tests := []struct {
		name             string
		instructionsFile string
		source           string
		expectError      bool
		errorContains    string
	}{
		{
			name:             "empty file is valid",
			instructionsFile: "",
			source:           "",
			expectError:      false,
		},
		{
			name:             "project source with any path is valid",
			instructionsFile: "some/path/file.md",
			source:           "project",
			expectError:      false,
		},
		{
			name:             "playbook with valid format",
			instructionsFile: "cc/instructions/file.md",
			source:           "playbook",
			expectError:      false,
		},
		{
			name:             "playbook with missing slash",
			instructionsFile: "just-a-filename.md",
			source:           "playbook",
			expectError:      true,
			errorContains:    "invalid playbook instructions_file format",
		},
		{
			name:             "playbook with only playbook name",
			instructionsFile: "cc",
			source:           "playbook",
			expectError:      true,
			errorContains:    "invalid playbook instructions_file format",
		},
		{
			name:             "playbook with empty playbook name",
			instructionsFile: "/instructions/file.md",
			source:           "playbook",
			expectError:      true,
			errorContains:    "invalid playbook instructions_file format",
		},
		{
			name:             "playbook with empty path",
			instructionsFile: "cc/",
			source:           "playbook",
			expectError:      true,
			errorContains:    "invalid playbook instructions_file format",
		},
		{
			name:             "reference source with any path is valid",
			instructionsFile: "some/path/file.md",
			source:           "reference",
			expectError:      false,
		},
		{
			name:             "invalid source",
			instructionsFile: "file.md",
			source:           "invalid",
			expectError:      true,
			errorContains:    "invalid instructions_file_source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTaskInstructions(tt.instructionsFile, tt.source)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errorContains)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
