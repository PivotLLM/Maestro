/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

// Package templates provides JSON schema validation and template processing
// for worker responses and QA feedback.
package templates

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/PivotLLM/Maestro/logging"
	"github.com/xeipuuv/gojsonschema"
)

// Validator validates JSON data against schemas and processes templates
type Validator struct {
	logger      *logging.Logger
	schemaCache map[string]*gojsonschema.Schema
}

// ValidationResult represents the result of a validation
type ValidationResult struct {
	Valid     bool     `json:"valid"`
	Errors    []string `json:"errors,omitempty"`     // User-friendly error messages
	RawErrors []string `json:"raw_errors,omitempty"` // Original error messages from validator
}

// New creates a new Validator
func New(logger *logging.Logger) *Validator {
	return &Validator{
		logger:      logger,
		schemaCache: make(map[string]*gojsonschema.Schema),
	}
}

// ValidateJSON validates JSON data against a schema string
func (v *Validator) ValidateJSON(data []byte, schemaJSON string) (*ValidationResult, error) {
	schemaLoader := gojsonschema.NewStringLoader(schemaJSON)
	documentLoader := gojsonschema.NewBytesLoader(data)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	validationResult := &ValidationResult{
		Valid: result.Valid(),
	}

	if !result.Valid() {
		for _, desc := range result.Errors() {
			rawError := desc.String()
			validationResult.RawErrors = append(validationResult.RawErrors, rawError)
			validationResult.Errors = append(validationResult.Errors, formatValidationError(rawError))
		}
	}

	return validationResult, nil
}

// formatValidationError converts technical validation errors to user-friendly messages
func formatValidationError(rawError string) string {
	// Common patterns from gojsonschema:
	// "(root): field is required" -> "Missing required field: field"
	// "(root): Additional property x is not allowed" -> "Unexpected field: x (not allowed by schema)"
	// "field: Invalid type. Expected: string, given: number" -> "Field 'field': expected string, got number"
	// "(root).field: field is required" -> "Missing required field: field"

	// Handle "is required" errors
	if strings.Contains(rawError, "is required") {
		// Extract the field name - it's usually after ": " or after "(root)."
		parts := strings.SplitN(rawError, ": ", 2)
		if len(parts) == 2 {
			fieldPart := parts[1]
			fieldName := strings.TrimSuffix(fieldPart, " is required")
			// Clean up context prefix like "(root)." or "(root)"
			if strings.HasPrefix(parts[0], "(root).") {
				context := strings.TrimPrefix(parts[0], "(root).")
				return fmt.Sprintf("Missing required field: %s (in %s)", fieldName, context)
			}
			return fmt.Sprintf("Missing required field: %s", fieldName)
		}
	}

	// Handle "Additional property" errors
	if strings.Contains(rawError, "Additional property") {
		// "(root): Additional property x is not allowed"
		parts := strings.SplitN(rawError, "Additional property ", 2)
		if len(parts) == 2 {
			fieldPart := strings.TrimSuffix(parts[1], " is not allowed")
			return fmt.Sprintf("Unexpected field: %s (not allowed by schema)", fieldPart)
		}
	}

	// Handle "Invalid type" errors
	if strings.Contains(rawError, "Invalid type") {
		// "field: Invalid type. Expected: string, given: number"
		parts := strings.SplitN(rawError, ": Invalid type. ", 2)
		if len(parts) == 2 {
			field := parts[0]
			if field == "(root)" {
				field = "root object"
			}
			typeInfo := strings.ReplaceAll(parts[1], "Expected: ", "expected ")
			typeInfo = strings.ReplaceAll(typeInfo, ", given: ", ", got ")
			return fmt.Sprintf("Field '%s': %s", field, typeInfo)
		}
	}

	// Handle enum errors
	if strings.Contains(rawError, "must be one of the following") {
		parts := strings.SplitN(rawError, ": ", 2)
		if len(parts) == 2 {
			field := parts[0]
			if field == "(root)" {
				field = "root value"
			}
			return fmt.Sprintf("Field '%s': %s", field, parts[1])
		}
	}

	// Default: clean up (root) prefix at minimum
	if strings.HasPrefix(rawError, "(root): ") {
		return strings.TrimPrefix(rawError, "(root): ")
	}
	if strings.HasPrefix(rawError, "(root).") {
		return strings.TrimPrefix(rawError, "(root).")
	}

	return rawError
}

// ValidateJSONFile validates JSON data against a schema file
func (v *Validator) ValidateJSONFile(data []byte, schemaPath string) (*ValidationResult, error) {
	// Check cache
	schema, ok := v.schemaCache[schemaPath]
	if !ok {
		schemaContent, err := os.ReadFile(schemaPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read schema file: %w", err)
		}

		schemaLoader := gojsonschema.NewBytesLoader(schemaContent)
		schema, err = gojsonschema.NewSchema(schemaLoader)
		if err != nil {
			return nil, fmt.Errorf("failed to parse schema: %w", err)
		}
		v.schemaCache[schemaPath] = schema
	}

	documentLoader := gojsonschema.NewBytesLoader(data)
	result, err := schema.Validate(documentLoader)
	if err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	validationResult := &ValidationResult{
		Valid: result.Valid(),
	}

	if !result.Valid() {
		for _, desc := range result.Errors() {
			rawError := desc.String()
			validationResult.RawErrors = append(validationResult.RawErrors, rawError)
			validationResult.Errors = append(validationResult.Errors, formatValidationError(rawError))
		}
	}

	return validationResult, nil
}

// ExtractFields extracts specific fields from JSON data
func (v *Validator) ExtractFields(data []byte, fields []string) (map[string]interface{}, error) {
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	result := make(map[string]interface{})
	for _, field := range fields {
		if value, ok := parsed[field]; ok {
			result[field] = value
		}
	}

	return result, nil
}

// ExtractField extracts a single field from JSON data
func (v *Validator) ExtractField(data []byte, field string) (interface{}, error) {
	fields, err := v.ExtractFields(data, []string{field})
	if err != nil {
		return nil, err
	}

	value, ok := fields[field]
	if !ok {
		return nil, fmt.Errorf("field not found: %s", field)
	}

	return value, nil
}

// ExtractBool extracts a boolean field from JSON data
func (v *Validator) ExtractBool(data []byte, field string) (bool, error) {
	value, err := v.ExtractField(data, field)
	if err != nil {
		return false, err
	}

	boolVal, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("field %s is not a boolean", field)
	}

	return boolVal, nil
}

// ExtractString extracts a string field from JSON data
func (v *Validator) ExtractString(data []byte, field string) (string, error) {
	value, err := v.ExtractField(data, field)
	if err != nil {
		return "", err
	}

	strVal, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("field %s is not a string", field)
	}

	return strVal, nil
}

// PopulateTemplate populates a Go template with data
func (v *Validator) PopulateTemplate(templateContent string, data interface{}) (string, error) {
	tmpl, err := template.New("template").Funcs(templateFuncs()).Parse(templateContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// PopulateTemplateFile populates a Go template file with data
func (v *Validator) PopulateTemplateFile(templatePath string, data interface{}) (string, error) {
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template file: %w", err)
	}

	return v.PopulateTemplate(string(content), data)
}

// templateFuncs returns custom template functions
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"json": func(v interface{}) string {
			data, err := json.MarshalIndent(v, "", "  ")
			if err != nil {
				return fmt.Sprintf("error: %v", err)
			}
			return string(data)
		},
		"jsonCompact": func(v interface{}) string {
			data, err := json.Marshal(v)
			if err != nil {
				return fmt.Sprintf("error: %v", err)
			}
			return string(data)
		},
		"truncate": func(s string, length int) string {
			if len(s) <= length {
				return s
			}
			return s[:length] + "..."
		},
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"title": strings.Title,
		"join":  strings.Join,
		"default": func(def, value interface{}) interface{} {
			if value == nil {
				return def
			}
			if s, ok := value.(string); ok && s == "" {
				return def
			}
			return value
		},
	}
}

// ExtractJSON extracts JSON from a response that may be wrapped in various ways:
// 1. LLM client wrapper: {"text": "...actual content..."}
// 2. Markdown code fences: ```json\n{...}\n```
// 3. Prose before/after the JSON object
//
// It returns the innermost valid JSON object, or the original string if none found.
func ExtractJSON(response string) string {
	// Trim whitespace
	response = strings.TrimSpace(response)

	// Step 1: Check if wrapped in {"text": "..."} from LLM client
	response = unwrapTextWrapper(response)

	// Step 2: Extract JSON from markdown code fences if present
	if extracted := extractFromCodeFence(response); extracted != "" {
		return extracted
	}

	// Step 3: Find JSON - try object or array, whichever comes first
	firstBrace := strings.Index(response, "{")
	firstBracket := strings.Index(response, "[")

	// Determine which to try first based on position
	if firstBrace != -1 && (firstBracket == -1 || firstBrace < firstBracket) {
		// Try object first
		if extracted := extractJSONObject(response); extracted != "" {
			return extracted
		}
		if extracted := extractJSONArray(response); extracted != "" {
			return extracted
		}
	} else if firstBracket != -1 {
		// Try array first
		if extracted := extractJSONArray(response); extracted != "" {
			return extracted
		}
		if extracted := extractJSONObject(response); extracted != "" {
			return extracted
		}
	}

	// Return original if no valid JSON found
	return response
}

// unwrapTextWrapper checks if the response is wrapped in {"text": "..."} and extracts the inner content
func unwrapTextWrapper(response string) string {
	// Quick check - must start with { and be valid JSON
	if !strings.HasPrefix(response, "{") {
		return response
	}

	var wrapper struct {
		Text string `json:"text"`
	}

	if err := json.Unmarshal([]byte(response), &wrapper); err != nil {
		return response
	}

	// If we found a text field and it's the only/main content, unwrap it
	if wrapper.Text != "" {
		// Verify the wrapper only has "text" field (not a schema response that happens to have text)
		var generic map[string]interface{}
		if err := json.Unmarshal([]byte(response), &generic); err == nil {
			if len(generic) == 1 {
				if _, hasText := generic["text"]; hasText {
					return strings.TrimSpace(wrapper.Text)
				}
			}
		}
	}

	return response
}

// extractFromCodeFence extracts JSON from markdown code fences like ```json\n{...}\n```
func extractFromCodeFence(response string) string {
	// Look for ```json or ``` followed by JSON
	patterns := []string{"```json\n", "```json\r\n", "```\n{", "```\r\n{"}

	for _, pattern := range patterns {
		startIdx := strings.Index(response, pattern)
		if startIdx == -1 {
			continue
		}

		// Find the content start (after the opening fence)
		contentStart := startIdx + len(pattern)
		if strings.HasSuffix(pattern, "{") {
			contentStart-- // Include the opening brace
		}

		// Find the closing fence
		remaining := response[contentStart:]
		endIdx := strings.Index(remaining, "```")
		if endIdx == -1 {
			continue
		}

		content := strings.TrimSpace(remaining[:endIdx])

		// Validate it's proper JSON
		var js json.RawMessage
		if json.Unmarshal([]byte(content), &js) == nil {
			return content
		}
	}

	return ""
}

// extractJSONObject finds the first valid JSON object in the response
func extractJSONObject(response string) string {
	firstBrace := strings.Index(response, "{")
	if firstBrace == -1 {
		return ""
	}

	lastBrace := strings.LastIndex(response, "}")
	if lastBrace == -1 || lastBrace <= firstBrace {
		return ""
	}

	// Fast path: try first { to last } - this is the common case
	// when LLM returns clean JSON with optional prose before/after
	candidate := response[firstBrace : lastBrace+1]
	var js json.RawMessage
	if json.Unmarshal([]byte(candidate), &js) == nil {
		return candidate
	}

	// Fallback: iterate through } characters to find the first valid JSON
	// This handles cases like extra } after the JSON or multiple JSON objects
	for i := firstBrace; i < len(response); i++ {
		if response[i] == '}' {
			candidate := response[firstBrace : i+1]
			if json.Unmarshal([]byte(candidate), &js) == nil {
				return candidate
			}
		}
	}

	return ""
}

// extractJSONArray finds the first valid JSON array in the response
func extractJSONArray(response string) string {
	firstBracket := strings.Index(response, "[")
	if firstBracket == -1 {
		return ""
	}

	lastBracket := strings.LastIndex(response, "]")
	if lastBracket == -1 || lastBracket <= firstBracket {
		return ""
	}

	// Fast path: try first [ to last ] - this is the common case
	candidate := response[firstBracket : lastBracket+1]
	var js json.RawMessage
	if json.Unmarshal([]byte(candidate), &js) == nil {
		return candidate
	}

	// Fallback: iterate through ] characters to find the first valid JSON
	for i := firstBracket; i < len(response); i++ {
		if response[i] == ']' {
			candidate := response[firstBracket : i+1]
			if json.Unmarshal([]byte(candidate), &js) == nil {
				return candidate
			}
		}
	}

	return ""
}

// QAResponse represents the parsed QA response with the standardized verdict
type QAResponse struct {
	Verdict string `json:"verdict"` // Standardized: "pass", "fail", "escalate"
}

// qaVerdictOnly is used internally to extract only the verdict field
type qaVerdictOnly struct {
	Verdict string `json:"verdict"`
}

// ParseQAResponse parses a QA response and extracts the standardized verdict field.
// The verdict must be one of: "pass", "fail", "escalate" (case-insensitive).
// All QA schemas must include this field for workflow control.
// Other fields in the QA response are playbook-specific and used only for reporting.
func (v *Validator) ParseQAResponse(data []byte) (*QAResponse, error) {
	var parsed qaVerdictOnly
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse QA response: %w", err)
	}

	if parsed.Verdict == "" {
		return nil, fmt.Errorf("QA response missing required 'verdict' field")
	}

	// Normalize to lowercase for comparison
	verdict := strings.ToLower(parsed.Verdict)

	// Validate verdict value
	switch verdict {
	case "pass", "fail", "escalate":
		// Valid
	default:
		return nil, fmt.Errorf("invalid verdict: %q (must be 'pass', 'fail', or 'escalate')", parsed.Verdict)
	}

	return &QAResponse{
		Verdict: verdict,
	}, nil
}

// DefaultQASchema returns the default JSON schema for QA responses.
// The 'verdict' field is required for all QA responses and controls workflow.
// Additional fields (feedback, issues, etc.) are optional and playbook-specific.
func DefaultQASchema() string {
	return `{
  "type": "object",
  "required": ["verdict"],
  "properties": {
    "verdict": {
      "type": "string",
      "enum": ["pass", "fail", "escalate"],
      "description": "QA verdict: pass = work acceptable, fail = send back to worker, escalate = cannot be resolved by QA"
    },
    "feedback": {"type": "string"},
    "issues": {
      "type": "array",
      "items": {"type": "string"}
    }
  }
}`
}

// DefaultWorkerSchema returns a basic JSON schema for worker responses
func DefaultWorkerSchema() string {
	return `{
  "type": "object",
  "required": ["result"],
  "properties": {
    "result": {"type": "string"},
    "metadata": {"type": "object"}
  }
}`
}

// ValidateQASchema validates that a QA response schema includes the required verdict field.
// Returns an error if the schema is missing the verdict field or has invalid enum values.
func ValidateQASchema(schemaContent string) error {
	if schemaContent == "" {
		return nil // No schema to validate
	}

	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(schemaContent), &schema); err != nil {
		return fmt.Errorf("invalid JSON schema: %w", err)
	}

	// Check if properties exists
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("QA schema missing 'properties' object")
	}

	// Check if verdict property exists
	verdictProp, ok := properties["verdict"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("QA schema missing required 'verdict' property")
	}

	// Check if verdict has correct enum values
	enumValues, ok := verdictProp["enum"].([]interface{})
	if !ok {
		return fmt.Errorf("QA schema 'verdict' property must have 'enum' with values ['pass', 'fail', 'escalate']")
	}

	// Validate enum contains the required values
	requiredValues := map[string]bool{"pass": false, "fail": false, "escalate": false}
	for _, v := range enumValues {
		if s, ok := v.(string); ok {
			lower := strings.ToLower(s)
			if _, exists := requiredValues[lower]; exists {
				requiredValues[lower] = true
			}
		}
	}

	// Check all required values are present
	for value, found := range requiredValues {
		if !found {
			return fmt.Errorf("QA schema 'verdict' enum missing required value '%s'", value)
		}
	}

	// Check that verdict is in the required array
	required, _ := schema["required"].([]interface{})
	verdictRequired := false
	for _, r := range required {
		if s, ok := r.(string); ok && s == "verdict" {
			verdictRequired = true
			break
		}
	}
	if !verdictRequired {
		return fmt.Errorf("QA schema must include 'verdict' in the 'required' array")
	}

	return nil
}
