# Phase 05 – Execute Tasks to Completion

## Purpose

Carry out the actual work of the project:

- Create task sets and tasks based on the lists from Phase 04.
- Process each item thoroughly.
- Track status in tasks.
- Ensure **no item is skipped** (no sampling).
- Store per-item outputs.

## When to Use

Use this phase when:

- The plan is approved (Phase 03).
- Item lists have been created (Phase 04), if the project requires them.
- You are ready to perform detailed analysis/evaluation for each item.

## High-Level Goals

- Create task sets to organize work.
- Create one task per item.
- For each task:
  - Execute via the runner or manually,
  - Store outputs,
  - Track completion.
- Confirm that every item in every list has a corresponding completed task.

## Step-by-Step Checklist

1. **Review lists and plan**
   - Read the plan:
     - `project_file_get(project="<project>", path="plan.md")`
   - Check which lists require per-item processing:
     - `list_list(project="<project>")`
   - Review list contents:
     - `list_get_summary(list="requirements", project="<project>")`

2. **Create task sets**
   - Create task sets for different types of work:
     ```
     taskset_create(
       project="<project>",
       path="analysis",
       title="Analysis Tasks",
       description="Detailed requirement analysis",
       parallel=true,
       max_worker=2,
       max_qa=2
     )
     ```
   - Task sets can specify:
     - `parallel`: Enable parallel execution (default: false for sequential). When true, uses `runner.max_concurrent` from config.
     - `max_worker`: Maximum worker LLM invocations per task (default: 2, max: 5)
     - `max_qa`: Maximum QA LLM invocations per task (default: 2, max: 5)
   - Consider using hierarchical paths:
     - `analysis` – main analysis
     - `analysis/security` – security-specific
     - `qa` – quality assurance (if separate from inline QA)

3. **Create per-item tasks**

   **Option A: Use list_create_tasks (recommended)**
   - For each list that needs processing:
     ```
     # Using project files (default)
     list_create_tasks(
       project="<project>",
       path="analysis",
       list="requirements",
       type="item_check",
       title_template="Evaluate {{id}}",
       llm_model_id="claude-sonnet",
       instructions_file="analysis-instructions.md",
       instructions_file_source="project",
       prompt="Analyze this requirement for completeness and clarity."
     )

     # Using playbook files (must include playbook name)
     list_create_tasks(
       project="<project>",
       path="analysis",
       list="requirements",
       type="item_check",
       title_template="Evaluate {{id}}",
       instructions_file="my-playbook/instructions/analyze.md",
       instructions_file_source="playbook",
       prompt="Evaluate this work unit."
     )
     ```
   - This creates one task per list item with automatic context injection.
   - **Important**: When using `playbook` source, `instructions_file` must be `playbook-name/path/to/file.md`

   **Option B: Manual task creation**
   - For each list:
     - Use `list_get` to retrieve all items.
     - For each item, use `task_create` with:
       - `project`: the project name
       - `path`: the task set path
       - `title`: e.g., "Evaluate REQ-001 – Authentication Required"
       - `type`: e.g., `"item_check"`
       - `llm_model_id`: which LLM to use
       - `prompt`: task-specific prompt
       - **At least one** of `instructions_file`, `instructions_text`, or `prompt` must be provided

   **With QA verification enabled:**
   ```
   task_create(
     project="<project>",
     path="analysis",
     title="Analyze REQ-001",
     type="analysis",
     prompt="Analyze this requirement...",
     llm_model_id="claude-sonnet",
     qa_enabled=true,
     qa_prompt="Verify the analysis is accurate...",
     qa_llm_model_id="claude-opus"
   )
   ```

4. **Execute tasks**

   **Use the automated runner (recommended)**
   - Call `task_run(project="<project>", path="analysis")` to execute eligible tasks
   - Optionally override the taskset's parallel setting: `task_run(..., parallel="true")` or `parallel="false"`
   - **Pre-flight check**: Before running any tasks, Maestro tests all required LLMs. If any LLM is unavailable, execution fails fast to avoid wasted time.
   - The runner handles:
     - Rate limiting and retries
     - Result storage in task records
     - QA verification (if enabled on tasks)
   - Use `task_status(project="<project>", path="analysis")` to monitor progress:
     - Shows task counts by status (waiting, running, done, failed)
   - Use `task_results(project="<project>", path="analysis")` to retrieve outputs

5. **Handle failures**
   - If a task fails:
     - Check the error in the task record
     - Reset the task with updated configuration if needed
     - Re-run the task set
   - Failed tasks are automatically retried:
     - Infrastructure failures (network, timeouts): up to `limits.max_retries` (no LLM cost)
     - Worker LLM invocations: up to `limits.max_worker` (billable)
   - When `task_run` executes, a budget safeguard prevents runaway costs
   - To reset all tasks in a task set for re-execution:
     ```
     taskset_reset(
       project="<project>",
       path="analysis",
       delete_results=true  # Removes results files from disk (default: true)
     )
     ```

6. **Check completeness**
   - For each list:
     - Use `list_get_summary` to get item counts.
   - Use `task_status(project="<project>")` to check task counts by status.
   - Confirm that:
     - There is a task for every item in each list.
     - All tasks are `done` or appropriately explained.
   - Log completion: `project_log_append(project="<project>", message="All items processed")`

7. **Keep the user informed**
   - Periodically summarize:
     - How many items are done vs total,
     - Any items that require user input,
     - Any patterns or concerns discovered.

## Typical Tools Used

- `list_list` – list available lists in the project
- `list_get`, `list_get_summary` – read list contents
- `taskset_create` – create task sets
- `taskset_reset` – reset all tasks in a task set for re-execution
- `list_create_tasks` – create one task per list item
- `task_create` – create individual tasks
- `task_run` – execute eligible tasks
- `task_status` – check task status counts
- `task_results` – retrieve task results
- `task_list` – list tasks with details
- `project_file_get` – read plan and source documents
- `project_log_append` – record important notes

## Expected Outputs

- Task sets created for organizing work
- Tasks created for each item with:
  - Work results stored
  - QA results (if enabled)
  - Final status values
- Project log entries noting progress and completion.

## Instructions File Sources

Tasks can load instructions from three sources:

- **project**: Files in the project's `files/` directory (default)
- **playbook**: Files from playbooks - **must use format** `playbook-name/path/to/file.md`
- **reference**: Embedded reference documentation

Example with playbook instructions:
```
task_create(
  project="my-project",
  path="analysis",
  title="Evaluate Control 5.1",
  type="control_check",
  instructions_file="security-audit/instructions/control-check.md",
  instructions_file_source="playbook",
  prompt="Evaluate this control for compliance."
)
```
