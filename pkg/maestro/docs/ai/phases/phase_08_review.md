# Phase 08 – Facilitated Review

## Purpose

Enable human review and refinement of AI-generated task results through a structured facilitation process that:

- Presents results to a human supervisor for review,
- Allows supervisors to modify or replace AI-generated responses,
- Captures lessons learned for process improvement,
- Extracts item-specific guidance for future work.

## When to Use

Use this phase when:

- Task execution is complete (Phase 05),
- Quality assurance has been performed (Phase 06),
- Reports have been generated (Phase 07), and
- A human supervisor needs to review and refine results.

## High-Level Goals

- Facilitate systematic review of AI-generated work.
- Enable supervisor overrides with full audit trail.
- Capture lessons learned during the review process.
- Extract item-specific guidance to improve future work.
- Update playbooks with insights gained.

## Key Concepts

### Supervisor Override

The `supervisor_update` tool allows a human supervisor to replace the AI worker's response with their own content. This is useful when:

- The AI response is incomplete or incorrect,
- The supervisor has domain expertise to improve the response,
- The response needs adjustment based on organizational context.

Key behaviors:
- **Immutable history**: The original response and supervisor's replacement are both preserved in the task result history.
- **SupervisorOverride flag**: Set to `true` in the result, indicating human modification.
- **Template validation**: The supervisor's response must pass the same schema validation as the worker response.
- **QA data cleared**: Previous QA verification is removed since it evaluated the original (now replaced) response.
- **QA status set to "superseded"**: Indicates QA was invalidated by supervisor action.
- **QA verdict set to "N/A"**: Reports will show "QA: N/A" instead of stale pass/fail verdicts.

### Getting Task Results with Schema

Use `task_result_get` to retrieve a single task result along with the schema needed for supervisor updates:

```
task_result_get(
  project="<project>",
  uuid="<task-uuid>"
)
```

This returns:
- `worker_response` - The JSON response from the worker
- `worker_response_template` - Path to the schema file
- `worker_response_schema` - **Full schema content** (so you don't need to fetch it separately)
- `qa_response`, `qa_verdict`, `qa_status` - QA verification results
- `supervisor_override` - Whether a supervisor has already updated this finding
- `completed_at` - When the task was completed

The `worker_response_schema` field provides the actual JSON schema content, eliminating the need to make a separate call to fetch the schema before performing a supervisor update.

### List Item Completion Tracking

Use the `complete` field on list items to track review progress:

```
list_item_update(
  project="<project>",
  list="<list>",
  id="<item-id>",
  complete=true
)
```

Query incomplete items to find what still needs review:

```
list_item_search(
  project="<project>",
  list="<list>",
  complete="false"
)
```

### Lessons Learned

Create a dedicated file to capture process improvements discovered during review:

```
project_file_append(
  project="<project>",
  path="review/lessons-learned.md",
  content="## Finding: [Title]\n\n[Description of what was learned...]\n\n"
)
```

### Item-Specific Guidance

When reviewing reveals that certain items need special handling in future work, capture this guidance and add it to the playbook:

```
playbook_file_append(
  playbook="<playbook>",
  path="guidance/<item-category>.md",
  content="## [Item ID]\n\n[Specific guidance for this item...]\n\n"
)
```

## Step-by-Step Checklist

1. **Prepare for review session**
   - Ensure reports are generated: `report_list(project="<project>")`
   - List tasks to get UUIDs: `task_list(project="<project>", path="<taskset>")`
   - Identify which items need human review (typically all, for audits)

2. **Facilitate systematic review**
   - For each task, get full details with schema:
     ```
     task_result_get(project="<project>", uuid="<task-uuid>")
     ```
   - Present results to supervisor showing:
     - The task title and context,
     - The AI-generated response (parsed from `worker_response`),
     - QA feedback if available (`qa_response`, `qa_verdict`),
     - The schema for updates (`worker_response_schema`).
   - Ask the supervisor if the response is acceptable or needs modification

3. **Apply supervisor modifications**
   - If the supervisor provides corrections:
     ```
     supervisor_update(
       project="<project>",
       uuid="<task-uuid>",
       response="<supervisor's-corrected-response>"
     )
     ```
   - The response must match the `worker_response_template` schema if defined

4. **Track review progress**
   - Mark items as reviewed:
     ```
     list_item_update(
       project="<project>",
       list="<list>",
       id="<item-id>",
       complete=true
     )
     ```
   - Check remaining items: `list_get_summary(project="<project>", list="<list>", complete="false")`

5. **Capture lessons learned**
   - Throughout the review, note any process improvements
   - Document in project's lessons-learned file
   - Examples of lessons:
     - "AI tends to miss X in responses to Y-type items"
     - "The prompt for Z-type tasks should include..."
     - "QA checks should also verify..."

6. **Extract item-specific guidance**
   - When reviewing reveals special handling requirements:
     - Document the specific guidance
     - Add to playbook for future use
   - Example: A specific control always requires certain evidence types

7. **Update playbook with improvements**
   - Add new guidance files:
     ```
     playbook_file_put(
       playbook="<playbook>",
       path="guidance/<category>.md",
       content="# [Category] Guidance\n\n..."
     )
     ```
   - Update existing lists in playbook if items need annotations
   - Update prompts or templates based on lessons learned

8. **Regenerate reports if needed**
   - After supervisor updates, reports may be stale
   - Use `report_create` to regenerate:
     ```
     report_create(project="<project>")
     ```
   - Or start a new report session if you want a fresh report:
     ```
     report_start(project="<project>", title="Reviewed Report")
     ```

9. **Finalize review**
   - Verify all items are marked complete
   - Confirm all lessons learned are documented
   - Confirm playbook updates are applied
   - Log completion:
     ```
     project_log_append(
       project="<project>",
       message="Review phase complete. X items reviewed, Y supervisor overrides applied."
     )
     ```

## Typical Tools Used

- `task_result_get` – get single task result with schema for supervisor updates
- `supervisor_update` – replace worker response with supervisor's version
- `report_create` – regenerate reports after modifications (creates new report files)
- `task_results` – get multiple task results for batch review
- `task_list` – list tasks with UUIDs and status
- `list_item_update` – mark items as reviewed (complete=true)
- `list_item_search` – find items not yet reviewed (complete=false)
- `list_get_summary` – check review progress
- `project_file_put/append` – document lessons learned
- `playbook_file_put/append` – add guidance to playbook
- `project_log_append` – log review milestones

## Expected Outputs

- All items reviewed and marked complete in relevant lists
- Supervisor overrides applied where needed (with full audit trail)
- Lessons-learned file in project documenting process improvements
- Playbook updated with:
  - Item-specific guidance for future work
  - Improved prompts or templates based on learnings
- Reports regenerated to reflect any modifications
- Project log documenting review completion

## Review Workflow Example

```
# Facilitated Review Session

## 1. List tasks to get UUIDs
task_list(project="audit-2025", path="assessment")

## 2. Get individual result with schema
result = task_result_get(project="audit-2025", uuid="abc123-...")
# Returns worker_response, worker_response_schema, qa_verdict, etc.

## 3. Present to supervisor
Show:
  - Task title and status
  - Worker's response (parsed JSON)
  - QA verdict and comments (if present)
Ask: "Is this acceptable? (yes/modify)"

## 4. If supervisor wants to modify:
supervisor_update(
  project="audit-2025",
  uuid="abc123-...",
  response="{...supervisor's corrected JSON response...}"
)
# QA data is cleared, QA verdict set to "N/A"

## 5. Mark as reviewed
list_item_update(
  project="audit-2025",
  list="controls",
  id="CTRL-5.1",
  complete=true
)

## 6. Check progress
list_get_summary(project="audit-2025", list="controls", complete="false")
# Shows remaining items to review

## 7. Regenerate reports after all updates
report_create(project="audit-2025")
# Creates new report files reflecting supervisor changes
```

## Important Notes

- **Audit trail**: All supervisor modifications are preserved in task history. The original AI response is never deleted.
- **Template compliance**: Supervisor responses must match the same schema as worker responses. Use `worker_response_schema` from `task_result_get` to see required fields.
- **QA on supervisor updates**: When a supervisor updates a finding, previous QA data is cleared and the QA verdict is set to "N/A". This prevents stale QA results from appearing in reports.
- **Report regeneration**: After supervisor updates, always use `report_create` to generate fresh reports. This creates new files with a new timestamp prefix.
- **Playbook updates persist**: Guidance added during review benefits all future projects using the same playbook.
- **Complete flag protection**: The `complete` field cannot be set to true for playbook lists (only project lists).
