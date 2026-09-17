/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package maestro

import (
	"fmt"
	"strings"

	"github.com/PivotLLM/toolspec"

	"github.com/PivotLLM/Maestro/global"
)

// Reference orientation entry point. Reference files are otherwise read via
// the unified file_* tools with source="reference".

func (p *Provider) handleStartHere(call *toolspec.ToolCall) (*toolspec.Result, error) {
	p.logToolCall(global.ToolStartHere, nil)

	item, err := p.reference.Get("start.md", 0, 0)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}
	if p.hostDispatched {
		item.Content = hostDispatchStartText(item.Content)
	}

	return createJSONResult(item)
}

// Text substituted into start.md under host dispatch (see hostDispatchStartText).
const (
	standaloneLLMToolsLine = "Additional tools: `llm_list`, `llm_dispatch`, `llm_test`, `health`"
	hostLLMToolsLine       = "Additional tools: `health`"
	llmManagementHeading   = "### LLM Management"
	hostLLMManagement      = `### LLM Management (host-dispatched)

Maestro is embedded in a host that owns model selection. The LLM-management tools (list, dispatch, test) are not available and there is no Maestro LLM config: every worker, QA and revision prompt runs as a sub-agent of the host agent that called Maestro, with the host's own tools, model and fallback chain.

**Model selection**: leave ` + "`llm_model_id` and `qa_llm_model_id`" + ` empty to run on the host agent's default model. To request a specific model, set them to one of the host agent's configured model aliases (the same names accepted by the host's spawn tool). An alias the host does not recognise fails the task immediately, without retry. The examples above that show ` + "`llm_model_id=\"claude\"`" + ` are standalone-mode examples; do not copy those values here.

**Pre-flight check**: not performed — the host's fallback chain handles model availability.

**Timeouts**: each dispatched prompt is bounded by the host's turn timeout. A task that exceeds it fails and is retried according to the task set's limits.

`
)

// hostDispatchStartText adapts the embedded start.md for host dispatch: the
// LLM-management tools it lists do not exist, and llm_model_id means a host
// model alias rather than a Maestro LLM config entry. It swaps the tools line
// and replaces the whole "### LLM Management" section (up to the next heading
// outside a code fence) with hostLLMManagement. Text that does not contain
// those markers is returned unchanged.
func hostDispatchStartText(s string) string {
	s = strings.Replace(s, standaloneLLMToolsLine, hostLLMToolsLine, 1)
	// Timeout advice names llm_dispatch alongside task_run; only the latter exists here.
	s = strings.ReplaceAll(s, "`task_run` or `llm_dispatch`", "`task_run`")

	start := strings.Index(s, llmManagementHeading)
	if start < 0 {
		return s
	}
	lines := strings.Split(s[start:], "\n")
	inFence := false
	end := len(lines)
	for i := 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "```") {
			inFence = !inFence
			continue
		}
		if !inFence && strings.HasPrefix(lines[i], "#") {
			end = i
			break
		}
	}
	rest := strings.Join(lines[end:], "\n")
	return s[:start] + hostLLMManagement + rest
}
