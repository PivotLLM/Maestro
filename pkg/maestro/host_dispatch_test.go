// Maestro
// License: MIT

package maestro

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PivotLLM/toolspec"

	"github.com/PivotLLM/Maestro/config"
	"github.com/PivotLLM/Maestro/llm"
	"github.com/PivotLLM/Maestro/logging"
	"github.com/PivotLLM/Maestro/reference"
)

// stubDispatcher is the minimal host dispatcher needed to register the
// provider in host-dispatched mode.
type stubDispatcher struct{}

func (stubDispatcher) Dispatch(context.Context, *llm.DispatchRequest) (*llm.DispatchResult, error) {
	return &llm.DispatchResult{ExitCode: 0, Text: "ok", Success: true}, nil
}
func (stubDispatcher) GetLLM(id string) *config.LLM        { return &config.LLM{ID: id} }
func (stubDispatcher) GetExecInfo(string) *llm.LLMExecInfo { return &llm.LLMExecInfo{} }
func (stubDispatcher) TestLLM(string) (bool, error)        { return true, nil }

func registerProvider(t *testing.T, host any) []toolspec.ToolDefinition {
	t.Helper()
	cfg := config.New(config.WithBaseDir(t.TempDir()), config.WithEmbeddedFS(EmbeddedReference))
	if err := cfg.Prepare(); err != nil {
		t.Fatalf("prepare config: %v", err)
	}
	p := &Provider{}
	return p.RegisterTools(toolspec.Deps{Cfg: cfg, Host: host})
}

func modelHintDescriptions(defs []toolspec.ToolDefinition, param string) map[string]string {
	out := map[string]string{}
	for _, d := range defs {
		for _, prm := range d.Parameters {
			if prm.Name == param {
				out[d.Name] = prm.Description
			}
		}
	}
	return out
}

// TestRegisterTools_HostDispatch_ModelHintDescriptions: under host dispatch the
// model-hint parameters must describe the host's vocabulary (model aliases,
// empty = host default, unknown = fail without retry) instead of pointing at
// Maestro's LLM config; standalone keeps the original wording.
func TestRegisterTools_HostDispatch_ModelHintDescriptions(t *testing.T) {
	hostDefs := registerProvider(t, HostDeps{Dispatcher: stubDispatcher{}})
	for _, param := range []string{"llm_model_id", "qa_llm_model_id"} {
		descs := modelHintDescriptions(hostDefs, param)
		if len(descs) == 0 {
			t.Fatalf("no tool exposes %s under host dispatch", param)
		}
		for tool, desc := range descs {
			for _, want := range []string{"host", "alias", "Leave empty", "without retry"} {
				if !strings.Contains(desc, want) {
					t.Errorf("%s.%s description %q lacks %q", tool, param, desc, want)
				}
			}
		}
	}

	standaloneDefs := registerProvider(t, nil)
	for tool, desc := range modelHintDescriptions(standaloneDefs, "llm_model_id") {
		if strings.Contains(desc, "host") {
			t.Errorf("standalone %s.llm_model_id description %q must not mention the host", tool, desc)
		}
	}
}

// TestHostDispatchStartText_Synthetic: the tools line is swapped and the LLM
// Management section is replaced up to the next heading, with '#' lines inside
// code fences not mistaken for headings.
func TestHostDispatchStartText_Synthetic(t *testing.T) {
	in := strings.Join([]string{
		"# Start",
		standaloneLLMToolsLine,
		"",
		"### LLM Management",
		"Use `llm_list`.",
		"```",
		"# not a heading",
		"llm_test(llm_id=\"claude\")",
		"```",
		"**LLM Aliases**: use `llm_dispatch`.",
		"",
		"### Using the Runner",
		"runner text",
	}, "\n")

	out := hostDispatchStartText(in)

	if strings.Contains(out, standaloneLLMToolsLine) || !strings.Contains(out, hostLLMToolsLine) {
		t.Errorf("tools line not swapped:\n%s", out)
	}
	for _, gone := range []string{"`llm_list`", "`llm_dispatch`", "llm_test(", "# not a heading"} {
		if strings.Contains(out, gone) {
			t.Errorf("host text still contains %q:\n%s", gone, out)
		}
	}
	for _, kept := range []string{"# Start", "### LLM Management (host-dispatched)", "### Using the Runner", "runner text"} {
		if !strings.Contains(out, kept) {
			t.Errorf("host text lost %q:\n%s", kept, out)
		}
	}
	if strings.Count(out, "### LLM Management") != 1 {
		t.Errorf("expected exactly one LLM Management heading:\n%s", out)
	}
}

// TestHostDispatchStartText_NoMarkers: text without the markers is unchanged.
func TestHostDispatchStartText_NoMarkers(t *testing.T) {
	in := "# Plain\nnothing about llms here\n"
	if out := hostDispatchStartText(in); out != in {
		t.Errorf("text without markers was modified:\n%s", out)
	}
}

// TestHandleStartHere_HostDispatched: the real embedded start.md, served
// through the tool, must no longer advertise the LLM-management tools and must
// explain host model selection; standalone serves it unchanged.
func TestHandleStartHere_HostDispatched(t *testing.T) {
	lg, err := logging.New(filepath.Join(t.TempDir(), "maestro.log"))
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	defer lg.Close()
	ref := reference.NewService(reference.WithEmbeddedFS(EmbeddedReference), reference.WithLogger(lg))

	content := func(hostDispatched bool) string {
		t.Helper()
		p := &Provider{reference: ref, hostDispatched: hostDispatched}
		res, err := p.handleStartHere(&toolspec.ToolCall{})
		if err != nil {
			t.Fatalf("handleStartHere: %v", err)
		}
		var item struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(res.ForLLM), &item); err != nil {
			t.Fatalf("unmarshal start_here result: %v", err)
		}
		return item.Content
	}

	host := content(true)
	for _, gone := range []string{"`llm_list`", "`llm_dispatch`", "`llm_test`", "llm_test(llm_id"} {
		if strings.Contains(host, gone) {
			t.Errorf("host-dispatched start.md still advertises %q", gone)
		}
	}
	for _, want := range []string{"### LLM Management (host-dispatched)", "model aliases", "### Using the Runner"} {
		if !strings.Contains(host, want) {
			t.Errorf("host-dispatched start.md lacks %q", want)
		}
	}

	standalone := content(false)
	if !strings.Contains(standalone, "`llm_list`") {
		t.Error("standalone start.md must still document llm_list")
	}
}
