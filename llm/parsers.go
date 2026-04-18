/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package llm

import (
	"bufio"
	"encoding/json"
	"strings"

	"github.com/PivotLLM/Maestro/config"
)

// ParsedOutput is the result of parsing LLM stdout
type ParsedOutput struct {
	Text      string // extracted response text
	IsError   bool   // LLM reported an error in its output envelope
	TurnCount int    // number of turns (0 if not reported)
}

// claudeResultLine represents a single line of Claude CLI JSONL output.
// Claude with --output-format json produces JSONL; the line with type "result"
// contains the actual response text.
type claudeResultLine struct {
	Type     string `json:"type"`
	Subtype  string `json:"subtype"`
	Result   string `json:"result"`
	IsError  bool   `json:"is_error"`
	NumTurns int    `json:"num_turns"`
}

// geminiCliJSONResponse represents the JSON output from the gemini CLI with --output-format json.
type geminiCliJSONResponse struct {
	SessionID string              `json:"session_id"`
	Response  string              `json:"response"`
	Stats     geminiCliStatsBlock `json:"stats"`
}

type geminiCliStatsBlock struct {
	Models map[string]geminiCliModelStats `json:"models"`
}

type geminiCliModelStats struct {
	Tokens geminiCliTokens `json:"tokens"`
}

type geminiCliTokens struct {
	Input      int `json:"input"`
	Candidates int `json:"candidates"`
	Total      int `json:"total"`
}

// codexEvent represents a single JSONL event from `codex exec --json`.
type codexEvent struct {
	Type    string         `json:"type"`
	Message string         `json:"message,omitempty"`
	Item    *codexItem     `json:"item,omitempty"`
	Error   *codexEventErr `json:"error,omitempty"`
}

type codexItem struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type codexEventErr struct {
	Message string `json:"message"`
}

// parseOutput dispatches to the appropriate parser based on format.
func parseOutput(format string, stdout string) ParsedOutput {
	switch format {
	case config.OutputFormatClaude:
		return parseClaudeOutput(stdout)
	case config.OutputFormatGemini:
		return parseGeminiOutput(stdout)
	case config.OutputFormatCodex:
		return parseCodexOutput(stdout)
	default:
		return parseGenericOutput(stdout)
	}
}

// parseClaudeOutput parses Claude CLI JSONL output (--output-format json).
// It scans lines for the "result" type and extracts the response text.
// Falls back to raw stdout if no result line is found.
func parseClaudeOutput(stdout string) ParsedOutput {
	scanner := bufio.NewScanner(strings.NewReader(stdout))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var r claudeResultLine
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue
		}

		if r.Type == "result" {
			return ParsedOutput{
				Text:      r.Result,
				IsError:   r.IsError,
				TurnCount: r.NumTurns,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		// Line exceeded buffer — fall back to raw stdout
		return ParsedOutput{Text: stdout}
	}

	return ParsedOutput{Text: stdout}
}

// parseGeminiOutput parses Gemini CLI JSON output (--output-format json).
// Falls back to raw stdout if parsing fails.
func parseGeminiOutput(stdout string) ParsedOutput {
	trimmed := strings.TrimSpace(stdout)
	if trimmed == "" {
		return ParsedOutput{Text: stdout}
	}

	var resp geminiCliJSONResponse
	if err := json.Unmarshal([]byte(trimmed), &resp); err != nil {
		return ParsedOutput{Text: stdout}
	}

	text := strings.TrimSpace(resp.Response)
	if text == "" {
		return ParsedOutput{Text: stdout}
	}

	return ParsedOutput{Text: text}
}

// parseCodexOutput parses Codex CLI JSONL output (exec --json).
// Accumulates text from item.completed events with item.type == "agent_message".
// Event schema: https://github.com/openai/codex — item.completed / turn.failed / error.
// Falls back to raw stdout if no content is found.
func parseCodexOutput(stdout string) ParsedOutput {
	var parts []string
	isError := false

	scanner := bufio.NewScanner(strings.NewReader(stdout))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var event codexEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		switch event.Type {
		case "item.completed":
			if event.Item != nil && event.Item.Type == "agent_message" && event.Item.Text != "" {
				parts = append(parts, event.Item.Text)
			}
		case "turn.failed":
			isError = true
			if event.Error != nil && event.Error.Message != "" && len(parts) == 0 {
				parts = append(parts, event.Error.Message)
			}
		case "error":
			isError = true
			if event.Message != "" && len(parts) == 0 {
				parts = append(parts, event.Message)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		// Line exceeded buffer — fall back to raw stdout
		return ParsedOutput{Text: stdout}
	}

	if len(parts) == 0 {
		return ParsedOutput{Text: stdout, IsError: isError}
	}

	return ParsedOutput{
		Text:    strings.TrimSpace(strings.Join(parts, "\n")),
		IsError: isError,
	}
}

// parseGenericOutput returns raw stdout with no parsing.
func parseGenericOutput(stdout string) ParsedOutput {
	return ParsedOutput{Text: stdout}
}
