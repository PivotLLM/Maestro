# Phase 02 – Understand Requirements

## Purpose

Capture a clear, explicit understanding of what the user needs from this project so that:

- The plan you propose matches their goals.
- Success criteria are documented.
- Later, you can verify that the project actually met those requirements.

## When to Use

Use this phase:

- After initializing or resuming a project (Phase 01).
- When the user's goals or constraints are unclear or have changed.
- Before you design or modify the project plan (Phase 03).

## High-Level Goals

- Understand the user's objective in their own words.
- Identify available source materials (documents, systems, data).
- Identify constraints:
  - Thoroughness requirements (e.g., "no sampling, must check every item"),
  - Time or resource constraints,
  - Reporting expectations.
- Store this understanding in project files.

## Step-by-Step Checklist

1. **Ask clarifying questions**
   - Examples:
     - "What is the main goal of this project?"
     - "What documents or inputs should we use?"
     - "Are there checklists or requirements that must *all* be checked?"
     - "How detailed should the final report be, and who is the audience?"
   - Ask follow-up questions as needed until you have a clear picture.

2. **Verify evidence accessibility** (for audits/evaluations)
   - Review evidence locations documented in Phase 01
   - **Verify each evidence source is accessible**:
     - For project files: `project_file_list(project="<project>")` or `project_file_get`
     - For playbook files: `playbook_file_list` or `playbook_file_get`
     - For external paths: Confirm with user that files are in place
   - If evidence is missing or inaccessible:
     - Inform the user immediately
     - Ask them to provide or upload the missing files
     - Do NOT proceed to task creation until evidence is confirmed
   - Update `evidence-manifest.md` with verification status
   - Record in project log: "Evidence verified: [list of files]"

3. **Identify completeness requirements**
   - Especially ask:
     - "Is it important that we check every requirement/item without sampling?"
   - If yes, note that explicit full-coverage behavior is required.

4. **Identify constraints**
   - Ask about:
     - Deadlines or milestones,
     - Preferred order of work,
     - Any limits on which models or tools can be used.

5. **Write a requirements summary**
   - Create or update a file such as `requirements.md`:
     - `project_file_put(project="<project>", path="requirements.md", content="...")`
   - Include:
     - Restatement of the objective,
     - List of inputs and where they are stored,
     - Explicit completeness / no-sampling requirements,
     - Constraints and preferences,
     - Any assumptions you are making.
   - Log the milestone: `project_log_append(project="<project>", message="Requirements documented")`

6. **Confirm with the user**
   - Show a concise summary of your understanding.
   - Ask the user to confirm or correct it.
   - Update `requirements.md` if needed.

7. **Update project status**
   - Use `project_update(name="<project>", status="in_progress")` to indicate work has begun.

8. **Transition to planning**
   - Once the user agrees, tell them you will now design a plan (Phase 03).

## Typical Tools Used

- `project_file_put` – create or update `requirements.md`
- `project_file_get` – read existing notes
- `project_file_list` – verify evidence files exist in project
- `playbook_file_list` – verify evidence files exist in playbook
- `playbook_file_get` – read playbook evidence files
- `project_log_append` – record milestones
- `project_update` – update project status
- `llm_dispatch` – (optional) help distill long user descriptions into a concise requirements summary

## Expected Outputs

- A requirements document (`requirements.md`) containing:
  - Clear project objective,
  - Inputs and constraints,
  - Completeness expectations (e.g., full coverage),
  - Any assumptions.
- Evidence verification completed (for audits/evaluations):
  - All evidence files confirmed accessible
  - `evidence-manifest.md` updated with verification status
  - Project log entry: "Evidence verified"
- User-confirmed understanding of what success means for this project.
- Project log entry noting requirements were documented.
