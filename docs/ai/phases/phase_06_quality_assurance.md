# Phase 06 – Quality Assurance

## Purpose

Systematically verify the accuracy and correctness of all task outputs before final reporting:

- Detect hallucinations, fabricated evidence, or incorrect information.
- Verify that cited references, quotes, and evidence actually exist in source documents.
- Ensure outputs conform to required formats and standards.
- Catch errors before they propagate into final deliverables.

## When to Use

Use this phase when Phase 05 (Execute Tasks) is complete and all tasks have results.

This phase **MUST** be completed **UNLESS** the user explicitly agrees that it is not necessary.

## High-Level Goals

- Verify each task's output independently.
- Use a different LLM or different prompting approach for verification.
- Check that all cited evidence exists and supports the stated conclusions.
- Identify any hallucinations, fabrications, or errors.
- Flag issues for correction before proceeding to final reporting.

## QA Approach Options

### Option A: Inline QA (Recommended)

Enable QA verification when creating tasks in Phase 05:

```
task_create(
  project="<project>",
  path="analysis",
  title="Analyze REQ-001",
  prompt="...",
  llm_model_id="claude-sonnet",
  qa_enabled=true,
  qa_prompt="Verify the analysis is accurate...",
  qa_llm_model_id="claude-opus"
)
```

**Note**: Maximum QA iterations are controlled at the task set level via `limits.max_qa`, not per-task. When creating the task set, specify limits:

```
taskset_create(
  project="<project>",
  path="analysis",
  title="Analysis Tasks",
  max_worker=2,
  max_qa=3
)
```

With inline QA:
- The runner automatically runs QA after work completes
- QA results are stored in the task record
- Failed QA can trigger rework iterations (up to `limits.max_qa` invocations)

### Option B: Separate QA Task Set

Create a dedicated QA task set to verify completed work:

```
taskset_create(
  project="<project>",
  path="qa",
  title="Quality Assurance",
  description="Verification of analysis results"
)
```

## The Hallucination Problem

Worker LLMs can produce outputs that appear correct but contain:

- **Fabricated evidence**: Citing sections, pages, or quotes that don't exist.
- **Incorrect references**: Pointing to the wrong section for a claim.
- **Made-up quotes**: Text presented as quotes that isn't in the source.
- **Overstated conclusions**: Claiming PASS when evidence is insufficient.
- **Format violations**: Missing required fields or incorrect structure.

QA must independently verify outputs against source documents.

## Step-by-Step Checklist (for Separate QA)

1. **Retrieve task results**
   - Use `task_results(project="<project>", path="analysis")` to get completed task outputs.
   - Note the total count of tasks requiring QA.

2. **Design QA prompts**
   - QA prompts should instruct the verifier to:
     - Read the original task output.
     - Access the same source documents the worker used.
     - Verify each factual claim against the source.
     - Check that cited sections/pages exist.
     - Verify any quotes are accurate.
     - Confirm the verdict is supported by evidence.
   - QA prompts should NOT:
     - Re-do the original analysis from scratch.
     - Simply agree with the worker's conclusions.

3. **Consider using a different LLM**
   - Use `llm_list()` to see available LLMs.
   - Consider using a different model for QA than was used for execution.
   - This reduces the chance of correlated errors.

4. **Create QA tasks**
   - Create tasks in the QA task set:
   ```
   task_create(
     project="<project>",
     path="qa",
     title="QA: Analyze REQ-001",
     type="qa_verification",
     llm_model_id="claude-opus",
     prompt="Verify the following task output:\n\n<original_task_result>"
   )
   ```

5. **QA task output format**
   - QA tasks should produce structured output:
   ```
   Task Verified: <original_task_id>

   QA Verdict: VERIFIED | ISSUES_FOUND | CRITICAL_ERROR

   Evidence Checks:
   - <citation_1>: CONFIRMED | NOT_FOUND | MISQUOTED
   - <citation_2>: CONFIRMED | NOT_FOUND | MISQUOTED

   Format Compliance: PASS | FAIL

   Issues Found:
   - <issue_1>
   - <issue_2>

   Recommendation: ACCEPT | REVISE | REJECT

   QA Notes: <detailed explanation>
   ```

6. **Execute QA tasks**
   - Run QA tasks: `task_run(project="<project>", path="qa")`
   - Monitor for failures or timeouts.

7. **Review QA results**
   - Retrieve QA results: `task_results(project="<project>", path="qa")`
   - Categorize results:
     - **VERIFIED**: Output is accurate, proceed to reporting.
     - **ISSUES_FOUND**: Minor issues that need correction.
     - **CRITICAL_ERROR**: Major hallucination or fabrication detected.

8. **Handle issues**
   - For ISSUES_FOUND:
     - Review the specific issues identified.
     - Update the original task if needed.
     - Re-run QA on the corrected output.
   - For CRITICAL_ERROR:
     - Flag for human review.
     - Do NOT include in final report until resolved.
     - Log the error pattern for future prevention.

9. **Track QA coverage**
   - Ensure every execution task has been verified.
   - Use `task_status` to verify counts.
   - No task should proceed to final reporting without QA verification.

10. **Document QA summary**
    - Create a QA summary file:
    ```
    project_file_put(
      project="<project>",
      path="qa_summary.md",
      content="..."
    )
    ```
    - Include:
      - Total tasks verified.
      - Count by QA verdict.
      - List of issues found and how they were resolved.
    - Log completion: `project_log_append(project="<project>", message="QA complete")`

## Checking Inline QA Results

If using inline QA (tasks with `qa_enabled=true`):

1. Check task results for QA status:
   ```
   task_results(project="<project>", path="analysis")
   ```

2. Review QA fields in each task:
   - `qa.passed`: Whether QA passed
   - `qa.severity`: Issue severity (low, medium, high, critical)
   - `qa.result`: Full QA result (JSON with notes, issues, etc.)
   - `qa.invocations`: Number of QA LLM calls used

3. Generate a report showing QA status:
   ```
   task_report(
     project="<project>",
     format="markdown"
   )
   ```

   The report includes QA review sections for ALL QA-enabled tasks, showing:
   - QA verdict and notes/feedback
   - Issues list (if present)
   - Formatted using `qa_report_template` when configured

   To filter to only failed QA:
   ```
   task_report(
     project="<project>",
     format="markdown",
     qa_passed=false
   )
   ```

## QA Verification Checklist

For each task output, QA should verify:

- [ ] **Evidence exists**: Every cited section/page/table exists in the source document.
- [ ] **Quotes are accurate**: Any quoted text matches the source exactly.
- [ ] **References are correct**: Citations point to content that supports the claim.
- [ ] **Verdict is justified**: The conclusion follows from the evidence presented.
- [ ] **Format is correct**: All required fields are present.
- [ ] **No fabrication**: No made-up facts, figures, or references.

## Typical Tools Used

- `task_results` – retrieve task outputs for verification
- `taskset_create` – create QA task set
- `task_create` – create QA verification tasks
- `task_run` – execute QA tasks
- `task_status` – track QA task status
- `task_report` – generate report with QA status
- `project_file_get` – access source documents for verification
- `project_file_put` – write QA summary
- `llm_list` – check available LLMs for QA
- `project_log_append` – record QA progress

## Expected Outputs

- QA verification for every task (inline or separate).
- QA results with verdict and issue details.
- Corrected tasks for any issues found.
- QA summary document (if using separate QA).
- Project log entries documenting QA completion.
- Confidence that outputs are accurate before final reporting.

## Best Practices

1. **Never skip QA unless the user explicitly agrees.** The cost of undetected errors far exceeds verification cost.

2. **Use different LLMs when possible.** Correlated errors are less likely.

3. **Verify specific claims, not just reasonableness.** A response can sound reasonable while containing fabricated details.

4. **Document all issues found.** Patterns help improve prompts.

5. **Re-verify after corrections.** Don't assume a corrected output is correct.

6. **Include QA status in final reports.** Stakeholders should know outputs were verified.
