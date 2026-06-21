# Task Report: {{.Title}}

**Task ID**: {{.ID}}
**UUID**: {{.UUID}}
**Type**: {{.Type}}
**Status**: {{.WorkStatus}}
{{if .CompletedAt}}**Completed**: {{.CompletedAt.Format "2006-01-02 15:04:05"}}{{end}}

## Result

{{.WorkResult}}

{{if .QAEnabled}}
## Quality Assurance

- **QA Status**: {{if .QAPassed}}✅ Passed{{else}}❌ Failed{{end}}
{{if .QASeverity}}- **Severity**: {{upper .QASeverity}}{{end}}

{{if .QAFeedback}}
### QA Feedback

{{.QAFeedback}}
{{end}}
{{end}}

<!--
TEMPLATE NOTES:
This is the default worker report template. For structured audit/evaluation reports,
create a playbook-specific template with the following recommended structure:

### {{.item_id}} {{.requirement}}

**Task**: {{._task_id}} ({{._task_status}})
**Status**: {{.status}}
**QA**: {{if ._qa_passed}}✅ Passed{{else}}❌ Failed{{end}}

**Requirement**
{{.requirement}}

**Test**
{{.test}}

**Evidence**
{{range .evidence}}
{{.num}}. {{.document}}, {{.section}}
{{end}}

**Observations**
{{.observations}}

**Requests**
{{.requests}}

**Rationale**
{{.rationale}}

**Evidence Paths** (for QA verification)
{{range .evidence}}
{{.num}}. {{.path}}, {{.section}}
{{end}}
-->
