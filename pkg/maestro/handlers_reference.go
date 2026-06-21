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

// Reference tool handlers (read-only, embedded)

func (p *Provider) handleReferenceList(call *toolspec.ToolCall) (*toolspec.Result, error) {
	prefix := parseString(call.Args, "prefix", "")

	p.logToolCall(global.ToolReferenceList, map[string]string{"prefix": prefix})

	items, err := p.reference.List(prefix)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"items": items,
		"count": len(items),
	}

	return createJSONResult(result)
}

func (p *Provider) handleReferenceGet(call *toolspec.ToolCall) (*toolspec.Result, error) {
	path := parseString(call.Args, "path", "")
	byteOffset := int64(parseFloat64(call.Args, "byte_offset", 0))
	maxBytes := int64(parseFloat64(call.Args, "max_bytes", 0))

	p.logToolCall(global.ToolReferenceGet, map[string]string{"path": path})

	if path == "" {
		return nil, fmt.Errorf("%s", "path parameter is required")
	}

	item, err := p.reference.Get(path, byteOffset, maxBytes)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(item)
}

func (p *Provider) handleStartHere(call *toolspec.ToolCall) (*toolspec.Result, error) {
	p.logToolCall(global.ToolStartHere, nil)

	item, err := p.reference.Get("start.md", 0, 0)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(item)
}

func (p *Provider) handleReferenceSearch(call *toolspec.ToolCall) (*toolspec.Result, error) {
	query := parseString(call.Args, "query", "")
	limit := int(parseFloat64(call.Args, "limit", 0))
	offset := int(parseFloat64(call.Args, "offset", 0))

	p.logToolCall(global.ToolReferenceSearch, map[string]string{"query": query})

	if query == "" {
		return nil, fmt.Errorf("%s", "query parameter is required")
	}

	items, total, err := p.reference.Search(query, limit, offset)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"items": items,
		"total": total,
		"count": len(items),
	}

	return createJSONResult(result)
}
