package maestro

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/PivotLLM/Maestro/config"
	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/lists"
	"github.com/PivotLLM/Maestro/llm"
	"github.com/PivotLLM/Maestro/logging"
	"github.com/PivotLLM/Maestro/playbooks"
	"github.com/PivotLLM/Maestro/projects"
	"github.com/PivotLLM/Maestro/reference"
	"github.com/PivotLLM/Maestro/runner"
	"github.com/PivotLLM/Maestro/tasks"

	"github.com/PivotLLM/toolspec"
)

// HostDeps encapsulates the host-provided services
type HostDeps struct {
	Logger *logging.Logger
	Runner *runner.Runner
	// Dispatcher, when set, makes the host own LLM selection and execution:
	// Maestro's tools only describe the work (a DispatchRequest) and this turns
	// it into a DispatchResult. With a host Dispatcher present, Maestro does not
	// choose the model and the LLM-management tools are not exposed.
	Dispatcher llm.Dispatcher
}

// Provider implements toolspec.ToolProvider for Maestro.
type Provider struct {
	config             *config.Config
	logger             *logging.Logger
	reference          *reference.Service
	playbooks          *playbooks.Service
	projects           *projects.Service
	tasks              *tasks.Service
	lists              *lists.Service
	llm                *llm.Service
	runner             *runner.Runner
	markNonDestructive bool
	hostDispatched     bool
	deps               toolspec.Deps
}

// RegisterTools initializes the Maestro services from deps.Cfg and returns the tools.
func (p *Provider) RegisterTools(deps toolspec.Deps) []toolspec.ToolDefinition {
	p.deps = deps
	cfg, ok := deps.Cfg.(*config.Config)
	if !ok {
		// If no config provided, create a default one
		cfg = config.New()
	}
	p.config = cfg

	// Initialize logger and runner from Host if provided
	var rInst *runner.Runner
	var hostDispatcher llm.Dispatcher
	if hd, ok := deps.Host.(HostDeps); ok {
		if hd.Logger != nil {
			p.logger = hd.Logger
		} else {
			p.logger, _ = logging.New("")
		}
		if hd.Runner != nil {
			rInst = hd.Runner
		}
		hostDispatcher = hd.Dispatcher
	} else if l, ok := deps.Host.(*logging.Logger); ok && l != nil {
		// Fallback for previous implementation
		p.logger = l
	} else {
		p.logger, _ = logging.New("")
	}

	// Recreate the initialization from server.go
	var externalDirs []reference.ExternalDir
	for _, refDir := range cfg.ReferenceDirs() {
		externalDirs = append(externalDirs, reference.ExternalDir{
			Path:  refDir.Path,
			Mount: refDir.Mount,
		})
	}

	p.reference = reference.NewService(
		reference.WithEmbeddedFS(cfg.EmbeddedFS()),
		reference.WithExternalDirs(externalDirs),
		reference.WithLogger(p.logger),
	)
	p.playbooks = playbooks.NewService(cfg.PlaybooksDir(), p.logger)
	p.projects = projects.NewService(cfg, p.logger)
	p.tasks = tasks.NewService(cfg, p.projects, p.logger)
	p.lists = lists.NewService(
		lists.WithProjectsDir(cfg.ProjectsDir()),
		lists.WithPlaybooksDir(cfg.PlaybooksDir()),
		lists.WithEmbeddedFS(cfg.EmbeddedFS()),
		lists.WithLogger(p.logger),
	)
	p.llm = llm.NewService(cfg, p.logger, nil)

	// The runner dispatches through the host's Dispatcher when one is injected
	// (the host owns model selection); otherwise it uses Maestro's own llm.Service.
	dispatcher := llm.Dispatcher(p.llm)
	if hostDispatcher != nil {
		dispatcher = hostDispatcher
		p.hostDispatched = true
	}

	if rInst != nil {
		p.runner = rInst
	} else {
		p.runner = runner.New(cfg, p.logger, nil, p.playbooks, p.reference, dispatcher, p.tasks, p.projects)
	}
	p.markNonDestructive = cfg.MarkNonDestructive()

	defs := p.getToolDefinitions()
	if p.hostDispatched {
		// The host owns LLM selection, so Maestro does not expose the
		// LLM-management tools — its tools only describe work to dispatch.
		defs = withoutTools(defs, global.ToolLLMList, global.ToolLLMDispatch, global.ToolLLMTest)
	}
	return defs
}

// withoutTools returns defs with any tool whose Name matches one of names removed.
func withoutTools(defs []toolspec.ToolDefinition, names ...string) []toolspec.ToolDefinition {
	drop := make(map[string]bool, len(names))
	for _, n := range names {
		drop[n] = true
	}
	out := defs[:0]
	for _, d := range defs {
		if !drop[d.Name] {
			out = append(out, d)
		}
	}
	return out
}

// createJSONResult formats data as a toolspec.Result
func createJSONResult(data interface{}) (*toolspec.Result, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return &toolspec.Result{ForLLM: "Failed to serialize JSON", IsError: true}, nil
	}
	return &toolspec.Result{ForLLM: string(b)}, nil
}

func parseString(args map[string]any, key string, def string) string {
	if val, ok := args[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return def
}

func parseFloat64(args map[string]any, key string, def float64) float64 {
	if val, ok := args[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return f
			}
		}
	}
	return def
}

func parseBool(args map[string]any, key string, def bool) bool {
	if val, ok := args[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
		if s, ok := val.(string); ok {
			return strings.ToLower(s) == "true"
		}
	}
	return def
}

func (p *Provider) logToolCall(toolName string, params map[string]string) {
	if p.logger == nil {
		return
	}
	if len(params) == 0 {
		p.logger.Infof("Tool %s called", toolName)
		return
	}
	var parts []string
	for k, v := range params {
		if v != "" {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
	}
	if len(parts) == 0 {
		p.logger.Infof("Tool %s called", toolName)
	} else {
		p.logger.Infof("Tool %s called: %s", toolName, strings.Join(parts, ", "))
	}
}
