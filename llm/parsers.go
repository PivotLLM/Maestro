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

// ParsedOutput is the result of parsing LLM stdout.
//
// Fields fall into three groups:
//   - response: Text, ResponseParsed
//   - termination/error envelope: IsError, NormalTermination, StopReason
//   - resource accounting (mirrors ClawEh DispatchStatus where applicable):
//     NumTurns, InputTokens, OutputTokens, CacheReadTokens, CacheCreationTokens,
//     CostUSD, DurationMs, ProviderModel
type ParsedOutput struct {
	// Response
	Text           string
	ResponseParsed bool

	// Termination / error envelope
	IsError           bool
	NormalTermination bool
	StopReason        string

	// Resource accounting
	NumTurns            int
	InputTokens         int
	OutputTokens        int
	CacheReadTokens     int
	CacheCreationTokens int
	CostUSD             float64
	DurationMs          int64
	ProviderModel       string
}

// claudeUsageBlock mirrors the usage payload in Claude's JSONL result line.
type claudeUsageBlock struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

// claudeModelUsage mirrors a single entry in the envelope's modelUsage map.
// Field names are camelCase (matching the Claude CLI envelope), distinct from
// the snake_case top-level usage block. Missing fields decode to zero.
type claudeModelUsage struct {
	InputTokens              int `json:"inputTokens"`
	OutputTokens             int `json:"outputTokens"`
	CacheReadInputTokens     int `json:"cacheReadInputTokens"`
	CacheCreationInputTokens int `json:"cacheCreationInputTokens"`
}

// total returns the sum of all token counts in this entry. Missing fields are
// already zero from JSON decoding, so this is safe even on partial entries.
func (m claudeModelUsage) total() int {
	return m.InputTokens + m.OutputTokens + m.CacheReadInputTokens + m.CacheCreationInputTokens
}

// claudeResultLine represents a single line of Claude CLI JSONL output.
// Claude with --output-format json produces JSONL; the line with type "result"
// contains the actual response text plus usage, cost, duration, and stop_reason.
type claudeResultLine struct {
	Type         string           `json:"type"`
	Subtype      string           `json:"subtype"`
	Result       string           `json:"result"`
	IsError      bool             `json:"is_error"`
	NumTurns     int              `json:"num_turns"`
	DurationMs   int64            `json:"duration_ms"`
	StopReason   string           `json:"stop_reason"`
	TotalCostUSD float64          `json:"total_cost_usd"`
	Usage        claudeUsageBlock `json:"usage"`
	// Model is the envelope's top-level model field. Claude CLI sets this to
	// the *last* model used in the turn, which is often a helper tier even when
	// the bulk of work ran on a higher-tier model. Used only as a fallback when
	// modelUsage is missing, empty, or all-zero.
	Model string `json:"model"`
	// ModelUsage is keyed by the actual provider-returned model name (e.g.
	// "claude-opus-4-7[1m]", "claude-haiku-4-5-20251001"). Each value carries
	// per-model token counts. We pick the model with the highest total tokens
	// as the primary model for ProviderModel reporting.
	ModelUsage map[string]claudeModelUsage `json:"modelUsage"`
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

type geminiCliAPIStats struct {
	TotalRequests  int   `json:"totalRequests"`
	TotalErrors    int   `json:"totalErrors"`
	TotalLatencyMs int64 `json:"totalLatencyMs"`
}

type geminiCliModelStats struct {
	API    geminiCliAPIStats              `json:"api"`
	Tokens geminiCliTokens                `json:"tokens"`
	Roles  map[string]geminiCliModelStats `json:"roles,omitempty"`
}

type geminiCliTokens struct {
	Input      int `json:"input"`
	Candidates int `json:"candidates"`
	Cached     int `json:"cached"`
	Total      int `json:"total"`
}

// codexEvent represents a single JSONL event from `codex exec --json`.
type codexEvent struct {
	Type    string          `json:"type"`
	Message string          `json:"message,omitempty"`
	Item    *codexItem      `json:"item,omitempty"`
	Error   *codexEventErr  `json:"error,omitempty"`
	Usage   *codexUsageInfo `json:"usage,omitempty"`
}

type codexItem struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type codexEventErr struct {
	Message string `json:"message"`
}

type codexUsageInfo struct {
	InputTokens           int `json:"input_tokens"`
	CachedInputTokens     int `json:"cached_input_tokens"`
	OutputTokens          int `json:"output_tokens"`
	ReasoningOutputTokens int `json:"reasoning_output_tokens"`
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
// It scans lines for the "result" type and extracts the response text plus
// usage/cost/duration/stop_reason. Falls back to raw stdout if no result line
// is found.
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

		if r.Type != "result" {
			continue
		}

		normalTermination := !r.IsError
		stopReason := r.StopReason
		if !normalTermination && stopReason == "" {
			if r.Subtype != "" && r.Subtype != "success" {
				stopReason = r.Subtype
			} else {
				stopReason = "error"
			}
		}

		// Pick the primary model from modelUsage by highest total token count.
		// Claude CLI's top-level "model" field reports the *last* model used in
		// the turn — often a helper tier (e.g. haiku) even when the bulk of work
		// ran on a higher-tier model (e.g. opus). The real breakdown lives in
		// modelUsage. Ties are broken by lexicographically first key for
		// deterministic output. Falls back to the top-level "model" field if
		// modelUsage is missing, empty, or every entry has zero total tokens.
		providerModel := primaryClaudeModel(r.ModelUsage)
		if providerModel == "" {
			providerModel = r.Model
		}

		return ParsedOutput{
			Text:                r.Result,
			ResponseParsed:      true,
			IsError:             r.IsError,
			NormalTermination:   normalTermination,
			StopReason:          stopReason,
			NumTurns:            r.NumTurns,
			InputTokens:         r.Usage.InputTokens,
			OutputTokens:        r.Usage.OutputTokens,
			CacheReadTokens:     r.Usage.CacheReadInputTokens,
			CacheCreationTokens: r.Usage.CacheCreationInputTokens,
			CostUSD:             r.TotalCostUSD,
			DurationMs:          r.DurationMs,
			ProviderModel:       providerModel,
		}
	}

	if err := scanner.Err(); err != nil {
		// Line exceeded buffer — fall back to raw stdout
		return ParsedOutput{Text: stdout}
	}

	return ParsedOutput{Text: stdout}
}

// primaryClaudeModel selects the highest-usage model from a Claude CLI
// envelope's modelUsage map and returns its bare model ID.
//
// Selection rules:
//   - Per-model total = sum of all token-count fields present (defensive: any
//     missing field decodes to zero).
//   - The model with the largest total wins.
//   - Ties are broken by lexicographically first key, for deterministic output.
//   - Entries with a zero total are ignored. If every entry is zero (or the map
//     is empty/nil) the function returns "" so the caller can fall back to the
//     envelope's headline "model" field.
//
// The returned name has any trailing context-window suffix (e.g. "[1m]")
// stripped so callers receive a bare model ID such as "claude-opus-4-7".
func primaryClaudeModel(modelUsage map[string]claudeModelUsage) string {
	var (
		bestName  string
		bestTotal int
	)
	for name, usage := range modelUsage {
		total := usage.total()
		if total <= 0 {
			continue
		}
		switch {
		case bestName == "":
			bestName = name
			bestTotal = total
		case total > bestTotal:
			bestName = name
			bestTotal = total
		case total == bestTotal && name < bestName:
			bestName = name
		}
	}
	return stripModelContextSuffix(bestName)
}

// stripModelContextSuffix removes a trailing bracketed context-window suffix
// from a Claude model ID, e.g. "claude-opus-4-7[1m]" -> "claude-opus-4-7".
// Returns the input unchanged if no such suffix is present.
func stripModelContextSuffix(name string) string {
	if !strings.HasSuffix(name, "]") {
		return name
	}
	i := strings.LastIndex(name, "[")
	if i <= 0 {
		return name
	}
	return name[:i]
}

// parseGeminiOutput parses Gemini CLI JSON output (--output-format json).
// Surfaces token usage, latency, and provider model from the stats block.
// Falls back to raw stdout if parsing fails.
func parseGeminiOutput(stdout string) ParsedOutput {
	trimmed := strings.TrimSpace(stdout)
	if trimmed == "" {
		return ParsedOutput{Text: stdout}
	}

	var resp geminiCliJSONResponse
	if err := json.Unmarshal([]byte(trimmed), &resp); err != nil {
		return ParsedOutput{Text: stdout, StopReason: "unparseable output"}
	}

	// Choose the "main role" model entry to report; fall back to largest-token model.
	var (
		chosenModelKey   string
		chosenModelStats geminiCliModelStats
		chosenHasMain    bool
		chosenTotal      int
		totalErrors      int
	)
	for modelKey, modelStats := range resp.Stats.Models {
		totalErrors += modelStats.API.TotalErrors

		hasMain := hasMainRole(modelStats)
		modelTotal := modelStats.Tokens.Total

		switch {
		case chosenModelKey == "":
			chosenModelKey = modelKey
			chosenModelStats = modelStats
			chosenHasMain = hasMain
			chosenTotal = modelTotal
		case hasMain && !chosenHasMain:
			// Prefer a model that exposes the "main" role.
			chosenModelKey = modelKey
			chosenModelStats = modelStats
			chosenHasMain = true
			chosenTotal = modelTotal
		case hasMain == chosenHasMain && modelTotal > chosenTotal:
			// Same role class — break the tie by larger token total.
			chosenModelKey = modelKey
			chosenModelStats = modelStats
			chosenTotal = modelTotal
		}
	}

	normalTermination := totalErrors == 0
	stopReason := ""
	if !normalTermination {
		stopReason = "API error"
	}

	text := strings.TrimSpace(resp.Response)
	return ParsedOutput{
		Text:              text,
		ResponseParsed:    true,
		IsError:           !normalTermination,
		NormalTermination: normalTermination,
		StopReason:        stopReason,
		InputTokens:       chosenModelStats.Tokens.Input,
		OutputTokens:      chosenModelStats.Tokens.Candidates,
		CacheReadTokens:   chosenModelStats.Tokens.Cached,
		DurationMs:        chosenModelStats.API.TotalLatencyMs,
		ProviderModel:     chosenModelKey,
	}
}

// hasMainRole reports whether the given model stats exposes a "main" role.
func hasMainRole(m geminiCliModelStats) bool {
	_, ok := m.Roles["main"]
	return ok
}

// parseCodexOutput parses Codex CLI JSONL output (exec --json).
// Accumulates text from item.completed events with item.type == "agent_message".
// Surfaces input/output/cached token counts from the turn.completed event and
// flags turn.failed / error events as provider envelope errors.
// Event schema: https://github.com/openai/codex — item.completed / turn.failed /
// error / turn.completed. Falls back to raw stdout if no structured events are
// found.
func parseCodexOutput(stdout string) ParsedOutput {
	var parts []string
	var (
		isError     bool
		foundEvents bool
		stopReason  string
		usage       codexUsageInfo
		usageSeen   bool
	)

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

		foundEvents = true

		switch event.Type {
		case "item.completed":
			if event.Item != nil && event.Item.Type == "agent_message" && event.Item.Text != "" {
				parts = append(parts, event.Item.Text)
			}
		case "turn.completed":
			if event.Usage != nil {
				usage = *event.Usage
				usageSeen = true
			}
		case "turn.failed":
			isError = true
			if stopReason == "" {
				stopReason = "turn.failed"
			}
			if event.Error != nil && event.Error.Message != "" {
				// Prefer the more informative error message but keep
				// the recognizable "turn.failed" sentinel for callers
				// that match on it. Combine for visibility.
				stopReason = "turn.failed: " + event.Error.Message
			}
		case "error":
			isError = true
			if event.Message != "" && stopReason == "" {
				stopReason = event.Message
			} else if stopReason == "" {
				stopReason = "error"
			}
		}
	}

	if err := scanner.Err(); err != nil {
		// Line exceeded buffer — fall back to raw stdout
		return ParsedOutput{Text: stdout}
	}

	if !foundEvents {
		return ParsedOutput{Text: stdout}
	}

	out := ParsedOutput{
		Text:              strings.TrimSpace(strings.Join(parts, "\n")),
		ResponseParsed:    true,
		IsError:           isError,
		NormalTermination: !isError,
		StopReason:        stopReason,
	}
	if usageSeen {
		out.InputTokens = usage.InputTokens
		out.OutputTokens = usage.OutputTokens
		out.CacheReadTokens = usage.CachedInputTokens
	}
	return out
}

// parseGenericOutput returns raw stdout with no parsing.
// NormalTermination is set to true here; callCommandLLM overrides it to false for non-zero exit codes.
func parseGenericOutput(stdout string) ParsedOutput {
	return ParsedOutput{Text: stdout, NormalTermination: true}
}
