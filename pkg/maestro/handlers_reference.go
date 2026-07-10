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

// Reference orientation entry point. Reference files are otherwise read via
// the unified file_* tools with source="reference".

func (p *Provider) handleStartHere(call *toolspec.ToolCall) (*toolspec.Result, error) {
	p.logToolCall(global.ToolStartHere, nil)

	item, err := p.reference.Get("start.md", 0, 0)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(item)
}
