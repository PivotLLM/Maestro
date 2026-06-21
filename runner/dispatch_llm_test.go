// Maestro
// License: MIT

package runner

import (
	"testing"

	"github.com/PivotLLM/Maestro/config"
)

// TestDispatchLLMID_HostDispatched: under host-dispatch the runner never
// resolves or validates a Maestro LLM and never fails — the host picks the
// model. An empty/"default" request yields the neutral "host" label; an explicit
// id is preserved verbatim (for logging only). No config is consulted.
func TestDispatchLLMID_HostDispatched(t *testing.T) {
	r := &Runner{hostDispatched: true} // nil config: must not be touched

	cases := map[string]string{
		"":           "host",
		"default":    "host",
		"gpt-4o":     "gpt-4o",
		"some-alias": "some-alias",
	}
	for in, want := range cases {
		got, ok := r.dispatchLLMID(in)
		if !ok {
			t.Errorf("dispatchLLMID(%q): ok=false, want true under host-dispatch", in)
		}
		if got != want {
			t.Errorf("dispatchLLMID(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestDispatchLLMID_StandaloneNoLLMs: standalone Maestro with no enabled LLMs
// still reports the failure (ok=false) so the runner raises no_llm_enabled.
func TestDispatchLLMID_StandaloneNoLLMs(t *testing.T) {
	cfg := config.New(config.WithBaseDir(t.TempDir()))
	if err := cfg.Prepare(); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	r := &Runner{hostDispatched: false, config: cfg}

	if _, ok := r.dispatchLLMID(""); ok {
		t.Error("standalone with no LLMs: expected ok=false")
	}
}
