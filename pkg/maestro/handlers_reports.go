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

// Report handlers - Read-only domain with controlled write access

// handleReportGet lists reports in a project, or reads one when a report name
// is given.
func (p *Provider) handleReportGet(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	report := parseString(call.Args, "report", "")

	p.logToolCall(global.ToolReportGet, map[string]string{"project": project, "report": report})

	if project == "" {
		return nil, fmt.Errorf("%s", "project parameter is required")
	}

	// No report name: list all reports.
	if report == "" {
		items, err := p.projects.ListReports(project)
		if err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
		}
		result := map[string]interface{}{
			"project": project,
			"reports": items,
			"count":   len(items),
		}
		return createJSONResult(result)
	}

	// Report name given: read it.
	byteOffset := int64(parseFloat64(call.Args, "byte_offset", 0))
	maxBytes := int64(parseFloat64(call.Args, "max_bytes", 0))
	item, err := p.projects.ReadReport(project, report, byteOffset, maxBytes)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}
	return createJSONResult(item)
}

// handleReportWrite drives the report session lifecycle: action=start begins a
// session (sets the prefix), action=append (default) adds content, action=end
// clears the session.
func (p *Provider) handleReportWrite(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	action := parseString(call.Args, "action", "append")

	p.logToolCall(global.ToolReportWrite, map[string]string{"project": project, "action": action})

	if project == "" {
		return nil, fmt.Errorf("%s", "project parameter is required")
	}

	switch action {
	case "start":
		title := parseString(call.Args, "title", "")
		intro := parseString(call.Args, "intro", "")
		if title == "" {
			return nil, fmt.Errorf("%s", "title parameter is required when action is 'start'")
		}
		prefix, err := p.projects.StartReport(project, title, intro)
		if err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
		}
		result := map[string]interface{}{
			"project":     project,
			"action":      "start",
			"prefix":      prefix,
			"main_report": prefix + "Report.md",
			"message":     "Report session started. Use report_write (action=append) to add content.",
		}
		return createJSONResult(result)

	case "append":
		content := parseString(call.Args, "content", "")
		report := parseString(call.Args, "report", "") // Optional - empty means main report
		if content == "" {
			return nil, fmt.Errorf("%s", "content parameter is required when action is 'append'")
		}
		if err := p.projects.AppendReport(project, content, report); err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
		}

		// Get current prefix to return filename info
		prefix, _ := p.projects.GetReportPrefix(project)
		var filename string
		if report == "" {
			filename = prefix + "Report.md"
		} else {
			filename = prefix + report + ".md"
		}

		result := map[string]interface{}{
			"project":       project,
			"action":        "append",
			"report":        filename,
			"bytes_written": len(content),
			"success":       true,
		}
		return createJSONResult(result)

	case "end":
		// Get prefix and list of reports BEFORE ending the session
		prefix, err := p.projects.GetReportPrefix(project)
		if err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
		}

		// List all reports to identify which ones belong to this session
		allReports, err := p.projects.ListReports(project)
		if err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
		}

		// Filter reports that match the current prefix
		var sessionReports []string
		for _, r := range allReports {
			if len(r.Name) >= len(prefix) && r.Name[:len(prefix)] == prefix {
				sessionReports = append(sessionReports, r.Name)
			}
		}

		// Now end the session
		if err := p.projects.EndReport(project); err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
		}

		result := map[string]interface{}{
			"project": project,
			"action":  "end",
			"prefix":  prefix,
			"reports": sessionReports,
			"count":   len(sessionReports),
			"message": "Report session ended. Prefix cleared.",
			"success": true,
		}
		return createJSONResult(result)

	default:
		return nil, fmt.Errorf("%s", "action must be 'start', 'append', or 'end'")
	}
}
