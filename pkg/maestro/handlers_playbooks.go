/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package maestro

import (
	"fmt"

	"github.com/PivotLLM/toolspec"

	"github.com/PivotLLM/Maestro/global"
)

// Playbook tool handlers

func (p *Provider) handlePlaybookList(call *toolspec.ToolCall) (*toolspec.Result, error) {
	p.logToolCall(global.ToolPlaybookList, nil)
	playbooks, err := p.playbooks.List()
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"playbooks": playbooks,
		"count":     len(playbooks),
	}

	return createJSONResult(result)
}

func (p *Provider) handlePlaybookCreate(call *toolspec.ToolCall) (*toolspec.Result, error) {
	name := parseString(call.Args, "name", "")

	p.logToolCall(global.ToolPlaybookCreate, map[string]string{"name": name})

	if name == "" {
		return nil, fmt.Errorf("%s", "name parameter is required")
	}

	if err := p.playbooks.Create(name); err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"playbook": name,
		"created":  true,
	}

	return createJSONResult(result)
}

func (p *Provider) handlePlaybookRename(call *toolspec.ToolCall) (*toolspec.Result, error) {
	name := parseString(call.Args, "name", "")
	newName := parseString(call.Args, "new_name", "")

	p.logToolCall(global.ToolPlaybookRename, map[string]string{"name": name, "new_name": newName})

	if name == "" {
		return nil, fmt.Errorf("%s", "name parameter is required")
	}
	if newName == "" {
		return nil, fmt.Errorf("%s", "new_name parameter is required")
	}

	if err := p.playbooks.Rename(name, newName); err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"from":    name,
		"to":      newName,
		"renamed": true,
	}

	return createJSONResult(result)
}

func (p *Provider) handlePlaybookDelete(call *toolspec.ToolCall) (*toolspec.Result, error) {
	name := parseString(call.Args, "name", "")

	p.logToolCall(global.ToolPlaybookDelete, map[string]string{"name": name})

	if name == "" {
		return nil, fmt.Errorf("%s", "name parameter is required")
	}

	if err := p.playbooks.Delete(name); err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"playbook": name,
		"deleted":  true,
	}

	return createJSONResult(result)
}
