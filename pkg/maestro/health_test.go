// Maestro
// License: MIT

package maestro

import (
	"encoding/json"
	"testing"

	"github.com/PivotLLM/Maestro/config"
)

// newHealthTestProvider builds a minimal Provider over a prepared base dir,
// exercising handleHealth without standing up the full service graph.
func newHealthTestProvider(t *testing.T, hostDispatched bool) *Provider {
	t.Helper()
	cfg := config.New(config.WithBaseDir(t.TempDir()))
	if err := cfg.Prepare(); err != nil {
		t.Fatalf("prepare config: %v", err)
	}
	return &Provider{config: cfg, hostDispatched: hostDispatched}
}

func healthResult(t *testing.T, p *Provider) map[string]any {
	t.Helper()
	res, err := p.handleHealth(nil)
	if err != nil {
		t.Fatalf("handleHealth: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(res.ForLLM), &out); err != nil {
		t.Fatalf("unmarshal health result %q: %v", res.ForLLM, err)
	}
	return out
}

// TestHandleHealth_HostDispatched: under host dispatch Maestro owns no LLM
// config, so health must not report LLM counts or flag "no LLMs enabled", and
// must stay healthy purely on the base directory.
func TestHandleHealth_HostDispatched(t *testing.T) {
	out := healthResult(t, newHealthTestProvider(t, true))

	if out["healthy"] != true {
		t.Errorf("expected healthy=true under host dispatch, got %v (issues=%v)", out["healthy"], out["issues"])
	}
	if _, ok := out["enabled_llms"]; ok {
		t.Error("enabled_llms must not be reported under host dispatch")
	}
	if _, ok := out["config_path"]; ok {
		t.Error("config_path must not be reported under host dispatch")
	}
	if out["dispatch"] != "host" {
		t.Errorf("expected dispatch=host, got %v", out["dispatch"])
	}
	if issues, ok := out["issues"]; ok {
		t.Errorf("expected no issues under host dispatch, got %v", issues)
	}
}

// TestHandleHealth_Standalone: a standalone Maestro (no host dispatcher) still
// reports LLM config and flags that none are enabled.
func TestHandleHealth_Standalone(t *testing.T) {
	out := healthResult(t, newHealthTestProvider(t, false))

	if _, ok := out["enabled_llms"]; !ok {
		t.Error("standalone health should report enabled_llms")
	}
	if out["dispatch"] != nil {
		t.Errorf("standalone health should not set dispatch, got %v", out["dispatch"])
	}
	// No LLMs configured in a bare base dir → flagged as an issue.
	if out["healthy"] != false {
		t.Errorf("expected unhealthy with no LLMs, got healthy=%v", out["healthy"])
	}
}
