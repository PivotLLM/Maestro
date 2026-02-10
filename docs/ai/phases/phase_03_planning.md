# Phase 03 – Plan the Project

## Purpose

Design a concrete, multi-phase plan that:

- Reflects the user's requirements,
- Uses Maestro's tools effectively,
- Defines task sets and tasks,
- Can be executed step-by-step and fully traced.

## When to Use

Use this phase:

- After the requirements are understood and documented (Phase 02).
- When starting a new project.
- When significantly changing the scope of an existing project.

## High-Level Goals

- Break the project into logical phases.
- Decide where task sets and per-item processing are needed.
- Define which task sets will be created.
- Decide how you will ensure full coverage (no sampling).
- Document the plan in project files.
- Confirm the plan with the user.

## Step-by-Step Checklist

1. **Review requirements**
   - Read `requirements.md` using `project_file_get`.
   - Ensure you understand:
     - The objective,
     - Inputs,
     - Completeness requirements,
     - Constraints.

2. **Sketch the phases**
   - At a minimum, plan around:
     - Initialization (already done),
     - Understanding and organizing inputs,
     - Building lists (if there are items/requirements to enumerate),
     - Per-item processing,
     - Quality assurance,
     - Reporting and verification.

3. **Plan task sets**
   - For any set of items that must *all* be processed:
     - Plan a task set to organize the work.
     - Decide the path hierarchy (e.g., `analysis`, `analysis/security`, `qa`).
   - Note which later phase will:
     - Build lists (Phase 04),
     - Create tasks in task sets (Phase 05).

4. **Define task set structure**
   - Decide on the task sets you will create:
     - Examples:
       - `analysis` – main analysis tasks
       - `analysis/security` – security-specific analysis
       - `qa` – quality assurance tasks
     - Consider whether QA should be built into tasks or a separate task set.

5. **Document the plan**
   - Write a human-readable plan to `plan.md`:
     - `project_file_put(project="<project>", path="plan.md", content="...")`
   - The plan should:
     - Describe each phase and its outputs,
     - Mention expected lists and task sets,
     - Describe how you will ensure full coverage.
   - Log the milestone: `project_log_append(project="<project>", message="Plan created")`

6. **Confirm the plan with the user**
   - Summarize the proposed phases and task sets in the chat.
   - Ask for confirmation or adjustments.
   - Update `plan.md` if the user requests changes.

7. **Transition to building lists (if needed)**
   - If the work requires enumerating items (requirements, controls, sections), move to Phase 04.
   - Otherwise, move to Phase 05 (execution) if your plan can proceed directly.

## Typical Tools Used

- `project_file_get` – read `requirements.md`
- `project_file_put` – write or update `plan.md`
- `project_log_append` – record milestones
- `list_list` – check for existing playbook lists

## Expected Outputs

- `plan.md` containing:
  - Phases,
  - Planned lists and task sets,
  - Strategy for full coverage,
  - Notes on which LLMs will be used where (if applicable).
- Project log entry noting plan was created.
