/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package llm

import "testing"

// Real Claude --output-format json single-line payload captured locally
// 2026-05-12 against haiku. Trimmed for test stability but field shape preserved.
const claudeSampleSuccess = `{"type":"result","subtype":"success","is_error":false,"duration_ms":2213,"num_turns":1,"result":"Hi!","stop_reason":"end_turn","session_id":"s","total_cost_usd":0.10784,"usage":{"input_tokens":10,"cache_creation_input_tokens":85968,"cache_read_input_tokens":0,"output_tokens":74},"modelUsage":{"claude-haiku-4-5-20251001":{"inputTokens":10,"outputTokens":74}}}`

const claudeSampleError = `{"type":"result","subtype":"error_during_execution","is_error":true,"duration_ms":500,"num_turns":1,"result":"","stop_reason":"","session_id":"s","total_cost_usd":0,"usage":{"input_tokens":5,"output_tokens":0}}`

func TestParseClaudeOutput_SuccessPopulatesResourceFields(t *testing.T) {
	got := parseClaudeOutput(claudeSampleSuccess)
	if !got.ResponseParsed {
		t.Fatalf("ResponseParsed = false, want true")
	}
	if got.IsError {
		t.Errorf("IsError = true, want false")
	}
	if !got.NormalTermination {
		t.Errorf("NormalTermination = false, want true")
	}
	if got.Text != "Hi!" {
		t.Errorf("Text = %q, want %q", got.Text, "Hi!")
	}
	if got.NumTurns != 1 {
		t.Errorf("NumTurns = %d, want 1", got.NumTurns)
	}
	if got.InputTokens != 10 {
		t.Errorf("InputTokens = %d, want 10", got.InputTokens)
	}
	if got.OutputTokens != 74 {
		t.Errorf("OutputTokens = %d, want 74", got.OutputTokens)
	}
	if got.CacheCreationTokens != 85968 {
		t.Errorf("CacheCreationTokens = %d, want 85968", got.CacheCreationTokens)
	}
	if got.CacheReadTokens != 0 {
		t.Errorf("CacheReadTokens = %d, want 0", got.CacheReadTokens)
	}
	if got.CostUSD < 0.107 || got.CostUSD > 0.109 {
		t.Errorf("CostUSD = %v, want ~0.10784", got.CostUSD)
	}
	if got.DurationMs != 2213 {
		t.Errorf("DurationMs = %d, want 2213", got.DurationMs)
	}
	if got.StopReason != "end_turn" {
		t.Errorf("StopReason = %q, want %q", got.StopReason, "end_turn")
	}
	if got.ProviderModel != "claude-haiku-4-5-20251001" {
		t.Errorf("ProviderModel = %q, want claude-haiku-4-5-20251001", got.ProviderModel)
	}
}

// TestParseClaudeOutput_ProviderModelFromModelUsage covers the primary-model
// selection logic added to address envelopes that mix multiple model tiers in a
// single turn (e.g. opus for the bulk of work + a haiku helper call). The
// table-driven cases verify:
//   - opus dominant + haiku helper → bare opus model id
//   - "[1m]"-suffixed key has the context-window suffix stripped
//   - modelUsage missing → falls back to top-level "model"
//   - modelUsage empty (or all entries zero-total) → falls back to top-level "model"
//   - tie on totals → deterministic pick (lexicographically first key)
func TestParseClaudeOutput_ProviderModelFromModelUsage(t *testing.T) {
	cases := []struct {
		name     string
		envelope string
		want     string
	}{
		{
			name: "opus_dominant_with_haiku_helper",
			envelope: `{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"num_turns":1,"result":"x","stop_reason":"end_turn","session_id":"s","total_cost_usd":0,"usage":{"input_tokens":0,"output_tokens":0},"model":"claude-haiku-4-5-20251001","modelUsage":{"claude-opus-4-7[1m]":{"inputTokens":1000,"outputTokens":500,"cacheReadInputTokens":2000,"cacheCreationInputTokens":3000},"claude-haiku-4-5-20251001":{"inputTokens":10,"outputTokens":20}}}`,
			want:     "claude-opus-4-7",
		},
		{
			name: "single_model_with_1m_suffix_stripped",
			envelope: `{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"num_turns":1,"result":"x","stop_reason":"end_turn","session_id":"s","total_cost_usd":0,"usage":{"input_tokens":0,"output_tokens":0},"model":"claude-opus-4-7[1m]","modelUsage":{"claude-opus-4-7[1m]":{"inputTokens":42,"outputTokens":7}}}`,
			want:     "claude-opus-4-7",
		},
		{
			name: "model_usage_missing_falls_back_to_headline",
			envelope: `{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"num_turns":1,"result":"x","stop_reason":"end_turn","session_id":"s","total_cost_usd":0,"usage":{"input_tokens":0,"output_tokens":0},"model":"claude-haiku-4-5-20251001"}`,
			want:     "claude-haiku-4-5-20251001",
		},
		{
			name: "model_usage_empty_falls_back_to_headline",
			envelope: `{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"num_turns":1,"result":"x","stop_reason":"end_turn","session_id":"s","total_cost_usd":0,"usage":{"input_tokens":0,"output_tokens":0},"model":"claude-haiku-4-5-20251001","modelUsage":{}}`,
			want:     "claude-haiku-4-5-20251001",
		},
		{
			name: "model_usage_all_zero_totals_falls_back_to_headline",
			envelope: `{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"num_turns":1,"result":"x","stop_reason":"end_turn","session_id":"s","total_cost_usd":0,"usage":{"input_tokens":0,"output_tokens":0},"model":"claude-opus-4-7","modelUsage":{"claude-haiku-4-5-20251001":{},"claude-opus-4-7[1m]":{"inputTokens":0,"outputTokens":0,"cacheReadInputTokens":0,"cacheCreationInputTokens":0}}}`,
			want:     "claude-opus-4-7",
		},
		{
			// Tie-breaking rule: when two models have identical totals, pick
			// the lexicographically first key (stable, documented).
			// Both entries sum to 100 here; "claude-haiku..." < "claude-opus..." so haiku wins.
			name:     "tie_on_total_lexicographically_first_wins",
			envelope: `{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"num_turns":1,"result":"x","stop_reason":"end_turn","session_id":"s","total_cost_usd":0,"usage":{"input_tokens":0,"output_tokens":0},"model":"claude-opus-4-7","modelUsage":{"claude-opus-4-7[1m]":{"inputTokens":60,"outputTokens":40},"claude-haiku-4-5-20251001":{"inputTokens":70,"outputTokens":30}}}`,
			want:     "claude-haiku-4-5-20251001",
		},
		{
			// Unparseable modelUsage value: malformed entry decodes to zero,
			// other model still wins on totals.
			name:     "unparseable_modelusage_entry_does_not_panic",
			envelope: `{"type":"result","subtype":"success","is_error":false,"duration_ms":1,"num_turns":1,"result":"x","stop_reason":"end_turn","session_id":"s","total_cost_usd":0,"usage":{"input_tokens":0,"output_tokens":0},"model":"claude-haiku-4-5-20251001","modelUsage":{"claude-opus-4-7[1m]":{"inputTokens":100}}}`,
			want:     "claude-opus-4-7",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseClaudeOutput(c.envelope)
			if got.ProviderModel != c.want {
				t.Errorf("ProviderModel = %q, want %q", got.ProviderModel, c.want)
			}
		})
	}
}

func TestParseClaudeOutput_ErrorEnvelope(t *testing.T) {
	got := parseClaudeOutput(claudeSampleError)
	if !got.IsError {
		t.Errorf("IsError = false, want true")
	}
	if got.NormalTermination {
		t.Errorf("NormalTermination = true, want false")
	}
	if got.StopReason == "" {
		t.Errorf("StopReason is empty; should fall back to subtype/error sentinel")
	}
	if got.StopReason != "error_during_execution" {
		t.Errorf("StopReason = %q, want %q", got.StopReason, "error_during_execution")
	}
}

// Codex sample captured 2026-05-12 from codex exec --json (gpt-5).
const codexSampleSuccess = `{"type":"thread.started","thread_id":"t"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"Hi."}}
{"type":"turn.completed","usage":{"input_tokens":13701,"cached_input_tokens":7552,"output_tokens":6,"reasoning_output_tokens":0}}`

const codexSampleFailure = `{"type":"thread.started","thread_id":"t"}
{"type":"turn.started"}
{"type":"turn.failed","error":{"message":"rate_limit_exceeded"}}`

func TestParseCodexOutput_SuccessPopulatesTokens(t *testing.T) {
	got := parseCodexOutput(codexSampleSuccess)
	if !got.ResponseParsed {
		t.Fatalf("ResponseParsed = false, want true")
	}
	if got.IsError {
		t.Errorf("IsError = true, want false")
	}
	if !got.NormalTermination {
		t.Errorf("NormalTermination = false, want true")
	}
	if got.Text != "Hi." {
		t.Errorf("Text = %q, want %q", got.Text, "Hi.")
	}
	if got.InputTokens != 13701 {
		t.Errorf("InputTokens = %d, want 13701", got.InputTokens)
	}
	if got.OutputTokens != 6 {
		t.Errorf("OutputTokens = %d, want 6", got.OutputTokens)
	}
	if got.CacheReadTokens != 7552 {
		t.Errorf("CacheReadTokens = %d, want 7552", got.CacheReadTokens)
	}
}

func TestParseCodexOutput_TurnFailedFlagsError(t *testing.T) {
	got := parseCodexOutput(codexSampleFailure)
	if !got.IsError {
		t.Errorf("IsError = false, want true")
	}
	if got.NormalTermination {
		t.Errorf("NormalTermination = true, want false")
	}
	if got.StopReason == "" {
		t.Errorf("StopReason is empty; want it to start with turn.failed")
	}
	// Must retain the turn.failed sentinel even when an error message is appended.
	if !contains(got.StopReason, "turn.failed") {
		t.Errorf("StopReason = %q, want contains %q", got.StopReason, "turn.failed")
	}
	if !contains(got.StopReason, "rate_limit_exceeded") {
		t.Errorf("StopReason = %q, want contains error message", got.StopReason)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// Gemini sample captured 2026-05-12 from gemini -o json. Trimmed.
const geminiSample = `{
  "session_id": "s",
  "response": "Hello!",
  "stats": {
    "models": {
      "gemini-2.5-flash-lite": {
        "api": {"totalRequests":1,"totalErrors":0,"totalLatencyMs":1193},
        "tokens": {"input":4112,"candidates":44,"total":4233,"cached":0},
        "roles": {
          "utility_router": {
            "api": {"totalRequests":1,"totalErrors":0,"totalLatencyMs":1193},
            "tokens": {"input":4112,"candidates":44,"total":4233,"cached":0}
          }
        }
      },
      "gemini-3-flash-preview": {
        "api": {"totalRequests":1,"totalErrors":0,"totalLatencyMs":4397},
        "tokens": {"input":62748,"candidates":19,"total":62830,"cached":0},
        "roles": {
          "main": {
            "api": {"totalRequests":1,"totalErrors":0,"totalLatencyMs":4397},
            "tokens": {"input":62748,"candidates":19,"total":62830,"cached":0}
          }
        }
      }
    }
  }
}`

const geminiSampleError = `{
  "session_id": "s",
  "response": "",
  "stats": {
    "models": {
      "gemini-3-flash-preview": {
        "api": {"totalRequests":1,"totalErrors":1,"totalLatencyMs":200},
        "tokens": {"input":0,"candidates":0,"total":0,"cached":0},
        "roles": {"main": {"api":{"totalRequests":1,"totalErrors":1,"totalLatencyMs":200}, "tokens":{}}}
      }
    }
  }
}`

func TestParseGeminiOutput_PrefersMainRoleModel(t *testing.T) {
	got := parseGeminiOutput(geminiSample)
	if got.ProviderModel != "gemini-3-flash-preview" {
		t.Errorf("ProviderModel = %q, want %q (the model exposing the main role)", got.ProviderModel, "gemini-3-flash-preview")
	}
	if got.InputTokens != 62748 {
		t.Errorf("InputTokens = %d, want 62748", got.InputTokens)
	}
	if got.OutputTokens != 19 {
		t.Errorf("OutputTokens = %d, want 19", got.OutputTokens)
	}
	if got.DurationMs != 4397 {
		t.Errorf("DurationMs = %d, want 4397", got.DurationMs)
	}
	if got.IsError || !got.NormalTermination {
		t.Errorf("expected normal termination; got IsError=%t NormalTermination=%t", got.IsError, got.NormalTermination)
	}
}

func TestParseGeminiOutput_TotalErrorsFlagsError(t *testing.T) {
	got := parseGeminiOutput(geminiSampleError)
	if !got.IsError {
		t.Errorf("IsError = false, want true")
	}
	if got.NormalTermination {
		t.Errorf("NormalTermination = true, want false")
	}
	if got.StopReason != "API error" {
		t.Errorf("StopReason = %q, want %q", got.StopReason, "API error")
	}
}

func TestProviderReportedError(t *testing.T) {
	cases := []struct {
		name string
		r    *DispatchResult
		want bool
	}{
		{"nil", nil, false},
		{"success", &DispatchResult{NormalTermination: true}, false},
		{"is_error", &DispatchResult{IsError: true}, true},
		{"abnormal_with_reason", &DispatchResult{NormalTermination: false, StopReason: "turn.failed"}, true},
		{"abnormal_without_reason", &DispatchResult{NormalTermination: false}, false},
		// Subtle: exit-code success with end_turn is normal — should not flag.
		{"normal_with_stop_reason", &DispatchResult{NormalTermination: true, StopReason: "end_turn"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.r.ProviderReportedError(); got != c.want {
				t.Errorf("ProviderReportedError = %t, want %t", got, c.want)
			}
		})
	}
}
