/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package templates

import (
	"strings"
	"testing"
)

func TestValidateJSON(t *testing.T) {
	v := New(nil)

	schema := `{
		"type": "object",
		"required": ["name"],
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer"}
		}
	}`

	tests := []struct {
		name    string
		data    string
		valid   bool
		wantErr bool
	}{
		{
			name:  "valid with required field",
			data:  `{"name": "John"}`,
			valid: true,
		},
		{
			name:  "valid with all fields",
			data:  `{"name": "John", "age": 30}`,
			valid: true,
		},
		{
			name:  "invalid missing required field",
			data:  `{"age": 30}`,
			valid: false,
		},
		{
			name:  "invalid wrong type",
			data:  `{"name": 123}`,
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.ValidateJSON([]byte(tt.data), schema)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Valid != tt.valid {
				t.Errorf("valid = %v, want %v; errors: %v", result.Valid, tt.valid, result.Errors)
			}
		})
	}
}

func TestExtractFields(t *testing.T) {
	v := New(nil)

	data := []byte(`{
		"name": "John",
		"age": 30,
		"city": "NYC"
	}`)

	fields, err := v.ExtractFields(data, []string{"name", "age", "nonexistent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if name, ok := fields["name"]; !ok || name != "John" {
		t.Errorf("name = %v, want John", name)
	}

	if age, ok := fields["age"]; !ok || age != float64(30) {
		t.Errorf("age = %v, want 30", age)
	}

	if _, ok := fields["nonexistent"]; ok {
		t.Error("nonexistent should not be present")
	}
}

func TestExtractBool(t *testing.T) {
	v := New(nil)

	data := []byte(`{"passed": true, "value": "string"}`)

	passed, err := v.ExtractBool(data, "passed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !passed {
		t.Error("passed should be true")
	}

	_, err = v.ExtractBool(data, "value")
	if err == nil {
		t.Error("expected error for non-boolean field")
	}

	_, err = v.ExtractBool(data, "missing")
	if err == nil {
		t.Error("expected error for missing field")
	}
}

func TestExtractString(t *testing.T) {
	v := New(nil)

	data := []byte(`{"name": "John", "age": 30}`)

	name, err := v.ExtractString(data, "name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "John" {
		t.Errorf("name = %q, want John", name)
	}

	_, err = v.ExtractString(data, "age")
	if err == nil {
		t.Error("expected error for non-string field")
	}
}

func TestPopulateTemplate(t *testing.T) {
	v := New(nil)

	tests := []struct {
		name     string
		template string
		data     interface{}
		want     string
		wantErr  bool
	}{
		{
			name:     "simple",
			template: "Hello, {{.Name}}!",
			data:     map[string]string{"Name": "World"},
			want:     "Hello, World!",
		},
		{
			name:     "with truncate",
			template: "{{truncate .Text 5}}",
			data:     map[string]string{"Text": "Hello World"},
			want:     "Hello...",
		},
		{
			name:     "with json",
			template: "{{json .}}",
			data:     map[string]string{"key": "value"},
			want:     "{\n  \"key\": \"value\"\n}",
		},
		{
			name:     "with default - value present",
			template: "{{default \"N/A\" .Name}}",
			data:     map[string]string{"Name": "John"},
			want:     "John",
		},
		{
			name:     "invalid template",
			template: "{{.Invalid",
			data:     nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := v.PopulateTemplate(tt.template, tt.data)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != tt.want {
				t.Errorf("result = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestParseQAResponse(t *testing.T) {
	v := New(nil)

	tests := []struct {
		name        string
		data        string
		wantVerdict string
		wantErr     bool
	}{
		{
			name:        "verdict pass lowercase",
			data:        `{"verdict": "pass", "comments": "All checks passed"}`,
			wantVerdict: "pass",
		},
		{
			name:        "verdict Pass uppercase",
			data:        `{"verdict": "Pass", "comments": "All checks passed"}`,
			wantVerdict: "pass",
		},
		{
			name:        "verdict PASS all caps",
			data:        `{"verdict": "PASS", "comments": "All checks passed"}`,
			wantVerdict: "pass",
		},
		{
			name:        "verdict fail",
			data:        `{"verdict": "fail", "comments": "Evidence missing"}`,
			wantVerdict: "fail",
		},
		{
			name:        "verdict Fail mixed case",
			data:        `{"verdict": "Fail", "comments": "Evidence missing"}`,
			wantVerdict: "fail",
		},
		{
			name:        "verdict escalate",
			data:        `{"verdict": "escalate", "comments": "Need senior review"}`,
			wantVerdict: "escalate",
		},
		{
			name:        "verdict Escalate mixed case",
			data:        `{"verdict": "Escalate", "comments": "Need senior review"}`,
			wantVerdict: "escalate",
		},
		{
			name:        "verdict with extra fields ignored",
			data:        `{"verdict": "pass", "document_verification": [], "issues": [], "comments": "OK"}`,
			wantVerdict: "pass",
		},
		{
			name:    "missing verdict field",
			data:    `{"comments": "No verdict here"}`,
			wantErr: true,
		},
		{
			name:    "empty verdict field",
			data:    `{"verdict": "", "comments": "Empty verdict"}`,
			wantErr: true,
		},
		{
			name:    "invalid verdict value",
			data:    `{"verdict": "maybe", "comments": "Unsure"}`,
			wantErr: true,
		},
		{
			name:    "invalid json",
			data:    `not json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := v.ParseQAResponse([]byte(tt.data))
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if response.Verdict != tt.wantVerdict {
				t.Errorf("verdict = %q, want %q", response.Verdict, tt.wantVerdict)
			}
		})
	}
}

func TestValidateQASchema(t *testing.T) {
	tests := []struct {
		name    string
		schema  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty schema is valid",
			schema:  "",
			wantErr: false,
		},
		{
			name: "valid schema with lowercase enum",
			schema: `{
				"type": "object",
				"required": ["verdict"],
				"properties": {
					"verdict": {
						"type": "string",
						"enum": ["pass", "fail", "escalate"]
					}
				}
			}`,
			wantErr: false,
		},
		{
			name: "valid schema with mixed case enum",
			schema: `{
				"type": "object",
				"required": ["verdict"],
				"properties": {
					"verdict": {
						"type": "string",
						"enum": ["Pass", "Fail", "Escalate"]
					}
				}
			}`,
			wantErr: false,
		},
		{
			name: "valid schema with extra fields",
			schema: `{
				"type": "object",
				"required": ["verdict", "comments"],
				"properties": {
					"verdict": {
						"type": "string",
						"enum": ["pass", "fail", "escalate"]
					},
					"comments": {"type": "string"},
					"issues": {"type": "array"}
				}
			}`,
			wantErr: false,
		},
		{
			name: "missing verdict property",
			schema: `{
				"type": "object",
				"required": ["comments"],
				"properties": {
					"comments": {"type": "string"}
				}
			}`,
			wantErr: true,
			errMsg:  "missing required 'verdict' property",
		},
		{
			name: "verdict not in required array",
			schema: `{
				"type": "object",
				"required": ["comments"],
				"properties": {
					"verdict": {
						"type": "string",
						"enum": ["pass", "fail", "escalate"]
					},
					"comments": {"type": "string"}
				}
			}`,
			wantErr: true,
			errMsg:  "must include 'verdict' in the 'required' array",
		},
		{
			name: "verdict missing enum",
			schema: `{
				"type": "object",
				"required": ["verdict"],
				"properties": {
					"verdict": {
						"type": "string"
					}
				}
			}`,
			wantErr: true,
			errMsg:  "must have 'enum'",
		},
		{
			name: "verdict enum missing pass",
			schema: `{
				"type": "object",
				"required": ["verdict"],
				"properties": {
					"verdict": {
						"type": "string",
						"enum": ["fail", "escalate"]
					}
				}
			}`,
			wantErr: true,
			errMsg:  "missing required value 'pass'",
		},
		{
			name: "verdict enum missing fail",
			schema: `{
				"type": "object",
				"required": ["verdict"],
				"properties": {
					"verdict": {
						"type": "string",
						"enum": ["pass", "escalate"]
					}
				}
			}`,
			wantErr: true,
			errMsg:  "missing required value 'fail'",
		},
		{
			name: "verdict enum missing escalate",
			schema: `{
				"type": "object",
				"required": ["verdict"],
				"properties": {
					"verdict": {
						"type": "string",
						"enum": ["pass", "fail"]
					}
				}
			}`,
			wantErr: true,
			errMsg:  "missing required value 'escalate'",
		},
		{
			name:    "invalid json",
			schema:  "not json",
			wantErr: true,
			errMsg:  "invalid JSON schema",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateQASchema(tt.schema)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain JSON object",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON with markdown code fence",
			input:    "```json\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON with markdown code fence and extra text",
			input:    "Here is the result:\n```json\n{\"key\": \"value\"}\n```\nDone.",
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON with leading/trailing whitespace",
			input:    "   \n{\"key\": \"value\"}\n   ",
			expected: `{"key": "value"}`,
		},
		{
			name:     "nested JSON object",
			input:    "```\n{\"outer\": {\"inner\": \"value\"}}\n```",
			expected: `{"outer": {"inner": "value"}}`,
		},
		{
			name:     "JSON array",
			input:    "```json\n[1, 2, 3]\n```",
			expected: `[1, 2, 3]`,
		},
		{
			name:     "array of objects",
			input:    "Result:\n[{\"id\": 1}, {\"id\": 2}]",
			expected: `[{"id": 1}, {"id": 2}]`,
		},
		{
			name:     "no JSON - returns original",
			input:    "This is just plain text",
			expected: "This is just plain text",
		},
		{
			name:     "malformed JSON - returns original",
			input:    "{broken json",
			expected: "{broken json",
		},
		{
			name:     "complex response with explanation",
			input:    "I've analyzed the requirement.\n\n```json\n{\"verdict\": \"Pass\", \"evidence\": \"Found in section 4.1\"}\n```\n\nLet me know if you need more details.",
			expected: `{"verdict": "Pass", "evidence": "Found in section 4.1"}`,
		},
		// Tests for {"text": "..."} wrapper from LLM client
		{
			name:     "text wrapper with plain JSON inside",
			input:    `{"text": "{\"key\": \"value\"}"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "text wrapper with markdown code fence inside",
			input:    "{\"text\": \"Here is the result:\\n```json\\n{\\\"verdict\\\": \\\"Pass\\\"}\\n```\"}",
			expected: `{"verdict": "Pass"}`,
		},
		{
			name:     "text wrapper with complex response",
			input:    "{\"text\": \"Based on my analysis...\\n\\n```json\\n{\\\"section\\\": \\\"4.1\\\", \\\"verdict\\\": \\\"Complete\\\", \\\"comments\\\": \\\"Evidence found\\\"}\\n```\\n\\nLet me know if you need more.\"}",
			expected: `{"section": "4.1", "verdict": "Complete", "comments": "Evidence found"}`,
		},
		{
			name:     "non-wrapper JSON with text field - should not unwrap",
			input:    `{"text": "hello", "other": "field"}`,
			expected: `{"text": "hello", "other": "field"}`,
		},
		{
			name:     "schema response that happens to have text field",
			input:    `{"verdict": "Pass", "text": "some text", "evidence": "found"}`,
			expected: `{"verdict": "Pass", "text": "some text", "evidence": "found"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractJSON(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractJSON() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestDefaultSchemas(t *testing.T) {
	v := New(nil)

	// Test QA schema - valid with verdict=pass
	qaSchema := DefaultQASchema()
	qaData := []byte(`{"verdict": "pass", "feedback": "Good"}`)
	result, err := v.ValidateJSON(qaData, qaSchema)
	if err != nil {
		t.Fatalf("QA schema validation error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid QA response, errors: %v", result.Errors)
	}

	// Test QA schema - valid with verdict=fail
	qaData = []byte(`{"verdict": "fail", "feedback": "Issues found"}`)
	result, err = v.ValidateJSON(qaData, qaSchema)
	if err != nil {
		t.Fatalf("QA schema validation error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid QA response with fail verdict, errors: %v", result.Errors)
	}

	// Test QA schema - valid with verdict=escalate
	qaData = []byte(`{"verdict": "escalate", "feedback": "Need review"}`)
	result, err = v.ValidateJSON(qaData, qaSchema)
	if err != nil {
		t.Fatalf("QA schema validation error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid QA response with escalate verdict, errors: %v", result.Errors)
	}

	// Test invalid QA data - missing verdict
	invalidQA := []byte(`{"feedback": "Missing verdict"}`)
	result, err = v.ValidateJSON(invalidQA, qaSchema)
	if err != nil {
		t.Fatalf("QA schema validation error: %v", err)
	}
	if result.Valid {
		t.Error("expected invalid for missing verdict field")
	}

	// Test invalid QA data - invalid verdict value
	invalidQA = []byte(`{"verdict": "maybe", "feedback": "Invalid verdict"}`)
	result, err = v.ValidateJSON(invalidQA, qaSchema)
	if err != nil {
		t.Fatalf("QA schema validation error: %v", err)
	}
	if result.Valid {
		t.Error("expected invalid for invalid verdict value")
	}

	// Test worker schema
	workerSchema := DefaultWorkerSchema()
	workerData := []byte(`{"result": "Success"}`)
	result, err = v.ValidateJSON(workerData, workerSchema)
	if err != nil {
		t.Fatalf("Worker schema validation error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid worker response, errors: %v", result.Errors)
	}
}
