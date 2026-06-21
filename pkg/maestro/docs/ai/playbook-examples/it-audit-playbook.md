# Example Playbook: IT Audit

This example demonstrates how to structure a playbook for IT audits. The patterns here apply to any audit or assessment work involving:

- A defined set of controls, requirements, or criteria to evaluate
- Evidence collection and assessment
- Findings and recommendations
- Human review and quality assurance

## Playbook Structure

```
it-audit/
├── files/
│   ├── readme.md                    # Playbook overview and usage
│   ├── phases/
│   │   ├── phase_01_init.md         # Project initialization guidance
│   │   ├── phase_02_scope.md        # Scope definition
│   │   ├── phase_03_planning.md     # Audit planning
│   │   ├── phase_04_evidence.md     # Evidence collection
│   │   ├── phase_05_assessment.md   # Control assessment
│   │   ├── phase_06_qa.md           # Quality assurance
│   │   ├── phase_07_reporting.md    # Report generation
│   │   └── phase_08_review.md       # Facilitated human review
│   ├── prompts/
│   │   ├── assess-control.md        # Control assessment prompt
│   │   └── qa-review.md             # QA review prompt
│   ├── templates/
│   │   ├── worker-response.json     # Response schema for assessments
│   │   ├── worker-report.md         # Report template
│   │   ├── qa-response.json         # QA response schema
│   │   └── disclaimer.md            # AI disclosure for reports
│   └── guidance/
│       └── control-hints.md         # Control-specific assessment tips
└── lists/
    └── controls.json                # Reusable control list (if applicable)
```

## Phase 08: Facilitated Review

The review phase enables human supervision of AI-generated assessments.

### Purpose

- Present AI assessments to audit lead for review
- Apply corrections where AI assessments need refinement
- Capture lessons learned for process improvement
- Extract control-specific guidance for future audits

### Key Activities

**1. Systematic Review**

Present each control assessment to the supervisor:

```
# Get assessments for review
task_results(project="<project>", path="controls")

# Track review progress
list_get_summary(project="<project>", list="controls", complete="false")
```

**2. Supervisor Corrections**

When the supervisor identifies issues:

```
supervisor_update(
  project="<project>",
  uuid="<task-uuid>",
  response="{...corrected assessment JSON...}"
)
```

The supervisor's response must match the `worker_response_template` schema. The original AI response is preserved in task history.

**3. Track Review Progress**

Mark items as reviewed:

```
list_item_update(
  project="<project>",
  list="controls",
  id="<control-id>",
  complete=true
)
```

**4. Capture Lessons Learned**

Document process improvements discovered during review:

```
project_file_append(
  project="<project>",
  path="review/lessons-learned.md",
  content="## [Date] - Prompt Improvement\n\n**Finding:** AI consistently missed...\n**Recommendation:** Update prompt to include...\n\n"
)
```

**5. Extract Control-Specific Guidance**

When certain controls need special handling:

```
# Add to project's guidance file
project_file_append(
  project="<project>",
  path="review/control-guidance.md",
  content="## [Control ID]\n\n**Assessment Tips:**\n- [Specific guidance]\n\n"
)

# Then transfer valuable guidance to playbook for future audits
playbook_file_append(
  playbook="it-audit",
  path="guidance/control-hints.md",
  content="## [Control ID]\n\n[Guidance content]\n\n"
)
```

**6. Regenerate Reports**

After supervisor updates:

```
report_create(project="<project>")
```

### Expected Outputs

**In Project:**
- All controls marked `complete=true`
- `review/lessons-learned.md` - Process improvements
- `review/control-guidance.md` - Control-specific tips
- Regenerated reports with corrections

**In Playbook:**
- Updated `guidance/control-hints.md` with assessment tips

## Templates

### Worker Response Schema (worker-response.json)

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "control_id": {"type": "string"},
    "finding": {"type": "string"},
    "verdict": {"type": "string", "enum": ["conforming", "non-conforming", "partial", "not-applicable"]},
    "evidence": {"type": "array", "items": {"type": "string"}},
    "recommendations": {"type": "array", "items": {"type": "string"}}
  },
  "required": ["control_id", "finding", "verdict"]
}
```

### Disclaimer Template (disclaimer.md)

```markdown
---

**Notice:** This document was prepared with the assistance of artificial intelligence. All findings have been reviewed by qualified personnel.
```

## Lessons Learned Template

Use this structure to capture improvements:

```markdown
# Lessons Learned - [Audit Name]

## [Date] - [Category]

**Context:** [Describe the situation]
**Finding:** [What was learned]
**Recommendation:** [How to improve]
**Action:** [Specific changes to make]

---

### Categories

- **Prompt Improvements** - Better ways to phrase assessment prompts
- **Evidence Gaps** - Types of evidence AI consistently misses
- **Context Needs** - Additional context that improves assessments
- **Template Refinements** - Changes to response templates
- **QA Improvements** - Better verification checks
```

## Control Guidance Template

Use this structure for control-specific tips:

```markdown
# Control-Specific Guidance

## [Control ID] - [Control Title]

**Assessment Tips:**
- [Specific guidance for assessing this control]

**Required Evidence:**
- [Types of evidence that should be requested]

**Common Issues:**
- [Typical problems or gaps encountered]
```

## Continuous Improvement Cycle

The review phase enables a learning loop:

1. **Execute audit** - AI assesses controls
2. **Review phase** - Supervisor corrects and captures insights
3. **Update playbook** - Add guidance for future audits
4. **Next audit** - AI benefits from accumulated knowledge

Over time, the playbook accumulates domain expertise, reducing supervisor corrections in future audits.
