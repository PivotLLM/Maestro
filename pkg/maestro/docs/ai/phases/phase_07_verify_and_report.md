# Phase 07 – Verify Completeness and Report

## Purpose

Confirm that the project is truly complete and produce one or more final reports that:

- Summarize what was done,
- Present key findings,
- Point to per-item evidence,
- Support review and audit.

## When to Use

Use this phase when:

- The main tasks appear to be complete, and
- QA verification has been performed (Phase 06), and
- The user is ready to see final results.

## High-Level Goals

- Verify completeness across all lists and tasks.
- Identify any remaining gaps or follow-up items.
- Produce a clear, organized report using `task_report`.
- Suggest possible improvements or templates for future projects.

## Step-by-Step Checklist

1. **Verify task completion**
   - Use `task_status(project="<project>")` to:
     - Confirm all tasks are `done` or appropriately closed.
   - For each task set:
     - `task_status(project="<project>", path="<path>")`
   - Confirm that all tasks are:
     - `done`, or
     - Marked `failed` with clear logs and follow-up tasks if necessary.

2. **Verify list coverage**
   - For each list:
     - Use `list_get_summary` to get item counts.
     - Compare the number of items with the number of completed tasks.
     - Ensure that every item has at least one associated task.

3. **Verify QA completion**
   - If using inline QA:
     - Check that all tasks have QA results
     - Review any failed QA items
   - If using separate QA task set:
     - Verify all QA tasks are complete
     - Review QA summary

4. **Identify any open issues**
   - If tasks are still `waiting` or `running`, decide whether:
     - They should be completed now, or
     - Deferred and documented as future work.
   - If deferred, document this clearly in the report.

5. **Check auto-generated reports**
   - Reports are **auto-generated** when the runner completes task sets
   - Use `report_list(project="<project>")` to see available reports
   - Use `report_read(project="<project>", report="<name>")` to read a report
   - Reports are in `<project>/reports/` directory

6. **Generate additional reports if needed**
   - Use `task_report` for custom or filtered reports:
     ```
     task_report(
       project="<project>",
       format="markdown"
     )
     ```
   - The report includes:
     - Project summary and statistics
     - Results organized by task set
     - Worker results formatted via `worker_report_template` (if configured)
     - QA review section for ALL QA-enabled tasks (not just failures):
       - QA verdict/status
       - QA notes/feedback
       - Issues list (if present)
       - Formatted via `qa_report_template` (if configured)
   - You can filter the report:
     ```
     task_report(
       project="<project>",
       format="markdown",
       path="analysis",        # Filter by task set
       status="done",          # Filter by status
       qa_passed=false         # Show only failed QA
     )
     ```

7. **Add custom content to reports**
   - Use `report_append` to add custom sections:
     ```
     report_append(
       project="<project>",
       content="## Executive Summary\n\n..."
     )
     ```
   - Include:
     - Project name and objective,
     - Summary of requirements and constraints,
     - Overview of the plan and phases executed,
     - Summary statistics,
     - Highlights and key findings,
     - Notable issues or risks,
     - Any open follow-ups or recommendations.

8. **Update project status**
   - Use `project_update(name="<project>", status="done")` to mark the project complete.
   - Log completion: `project_log_append(project="<project>", message="Project completed, final report generated")`

9. **Present the report to the user**
   - Summarize the report in the chat.
   - Point explicitly to the file path of the full report.
   - Ask if the user needs:
     - Additional views (e.g., JSON export, filtered reports),
     - Adjustments to the report.

10. **Continuous improvement**
    - Reflect on the project:
     - Did the phases and tasks work well?
     - Were there steps that could be generalized?
   - Extract any reusable patterns or templates.
   - Store them in playbooks for future use:
     - `playbook_file_put(playbook="<playbook>", path="template.md", content="...")`

## Typical Tools Used

- `task_status` – verify status of all tasks
- `task_report` – generate custom/filtered reports
- `report_list` – list auto-generated reports
- `report_read` – read a specific report
- `report_append` – add custom content to reports
- `list_get_summary` – verify list item counts
- `project_file_get` – inspect plan and items as needed
- `llm_dispatch` – assist with drafting supplementary content
- `project_update` – mark project as done
- `project_log_append` – record final milestone
- `playbook_file_put` – save reusable templates

## Expected Outputs

- Auto-generated reports in `<project>/reports/` directory containing:
  - Task results rendered via templates,
  - QA verification checklists and comments,
  - All findings organized by task set.
- Optional custom reports via `task_report` for specific views.
- Confirmation of completeness, or clearly documented limitations/follow-ups.
- Project status updated to `done`.
- Optional templates or playbook entries for similar future projects.

## Report Formats

### Markdown Report

```
task_report(project="<project>", format="markdown")
```

Produces a human-readable markdown document with:
- Project summary
- Task statistics by status
- Results organized by task set hierarchy
- QA status indicators

### JSON Report

```
task_report(project="<project>", format="json")
```

Produces structured JSON for programmatic processing or integration with other tools.

## Important

The end of every report **MUST** include the following disclosure:

```
---

This document was prepared with the assistance of artificial intelligence.
```
