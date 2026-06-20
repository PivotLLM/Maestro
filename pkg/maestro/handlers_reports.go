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

func (p *Provider) handleReportList(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")

	p.logToolCall(global.ToolReportList, map[string]string{"project": project})

	if project == "" {
		return nil, fmt.Errorf("%s", "project parameter is required")
	}

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

func (p *Provider) handleReportRead(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	report := parseString(call.Args, "report", "")
	byteOffset := int64(parseFloat64(call.Args, "byte_offset", 0))
	maxBytes := int64(parseFloat64(call.Args, "max_bytes", 0))

	p.logToolCall(global.ToolReportRead, map[string]string{"project": project, "report": report})

	if project == "" {
		return nil, fmt.Errorf("%s", "project parameter is required")
	}
	if report == "" {
		return nil, fmt.Errorf("%s", "report parameter is required")
	}

	item, err := p.projects.ReadReport(project, report, byteOffset, maxBytes)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(item)
}

func (p *Provider) handleReportStart(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	title := parseString(call.Args, "title", "")
	intro := parseString(call.Args, "intro", "")

	p.logToolCall(global.ToolReportStart, map[string]string{"project": project, "title": title})

	if project == "" {
		return nil, fmt.Errorf("%s", "project parameter is required")
	}
	if title == "" {
		return nil, fmt.Errorf("%s", "title parameter is required")
	}

	prefix, err := p.projects.StartReport(project, title, intro)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"project":     project,
		"prefix":      prefix,
		"main_report": prefix + "Report.md",
		"message":     "Report session started. Use report_append to add content.",
	}

	return createJSONResult(result)
}

func (p *Provider) handleReportAppend(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	content := parseString(call.Args, "content", "")
	report := parseString(call.Args, "report", "") // Optional - empty means main report

	p.logToolCall(global.ToolReportAppend, map[string]string{"project": project, "report": report})

	if project == "" {
		return nil, fmt.Errorf("%s", "project parameter is required")
	}
	if content == "" {
		return nil, fmt.Errorf("%s", "content parameter is required")
	}

	err := p.projects.AppendReport(project, content, report)
	if err != nil {
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
		"report":        filename,
		"bytes_written": len(content),
		"success":       true,
	}

	return createJSONResult(result)
}

func (p *Provider) handleReportEnd(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")

	p.logToolCall(global.ToolReportEnd, map[string]string{"project": project})

	if project == "" {
		return nil, fmt.Errorf("%s", "project parameter is required")
	}

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
	err = p.projects.EndReport(project)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"project": project,
		"prefix":  prefix,
		"reports": sessionReports,
		"count":   len(sessionReports),
		"message": "Report session ended. Prefix cleared.",
		"success": true,
	}

	return createJSONResult(result)
}
