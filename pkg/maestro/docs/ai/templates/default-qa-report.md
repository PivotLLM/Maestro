# QA Review

**Status**: {{if .verdict}}{{.verdict}}{{else}}{{if .passed}}✅ PASSED{{else}}❌ FAILED{{end}}{{end}}

## Document Verification

{{if .document_verification}}
| # | Document | Section | Exists | Supports |
|---|----------|---------|--------|----------|
{{range .document_verification}}- | {{.evidence_num}} | {{.document}} | {{.section}} | {{if .exists}}✅{{else}}❌{{end}} | {{if .supports_finding}}✅{{else}}❌{{end}} |
{{end}}
{{else}}
*No document verification data available*
{{end}}

## QA Comments

{{if .comments}}{{.comments}}{{else if .notes}}{{.notes}}{{else if .feedback}}{{.feedback}}{{else}}*No comments provided*{{end}}

{{if .issues}}
## Issues Found

{{range .issues}}
- **[{{.type}}]** {{.description}}{{if .severity}} ({{.severity}}){{end}}
{{end}}
{{end}}

{{if .auto_fail_reasons}}
## Automatic Failure Reasons

{{range .auto_fail_reasons}}
- {{.}}
{{end}}
{{end}}

<!--
TEMPLATE NOTES:
- Field names must match your playbook's QA JSON schema exactly
- Required fields in default schema:
  - verdict: "pass", "fail", or "escalate" (case-insensitive)
  - document_verification: Array of verification results
  - comments: Required QA summary (must always be provided)
  - issues: Array of issues found (empty if none)
- Document verification table should ALWAYS be shown
- This template is rendered for ALL QA-enabled tasks, not just failures
- Task metadata is available as _task_id, _task_title, _task_type, _qa_verdict
-->
