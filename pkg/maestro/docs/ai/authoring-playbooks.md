# Maestro Playbook Authoring Guide (For LLMs)

This document explains **how you, the LLM, should design and maintain playbooks** that work within the Maestro framework.

A **playbook** is a reusable, domain- or topic-specific set of instructions that layers on top of Maestro's general orchestration model. It explains:

- How a particular type of work should be handled across Maestro's phases
- Which task sets to create
- Which lists and tasks are needed
- How to ensure full coverage and an audit trail

You will create and update playbooks using the **playbooks domain** and its tools; the human user owns the domain knowledge, you help structure and refine it.

---

## 1. Where Playbooks Fit in Maestro

Maestro has three fixed domains:

- **Reference** – Built-in, read-only documentation bundled with the executable
- **Playbooks** – User-owned, reusable procedures and knowledge
- **Projects** – Active work: files, task sets, logs, and results

Playbooks live in the **playbooks domain** and are stored as files under a configured `playbooks_dir` (e.g., `~/.maestro/playbooks/<playbook_name>/*`). You interact with them using tools like:

- `playbook_list`, `playbook_create`, `playbook_delete`
- `playbook_file_list`, `playbook_file_get`, `playbook_file_put`, `playbook_file_delete`

Think of it this way:

- **Reference** = "How Maestro works in general"
- **Playbooks** = "How to apply Maestro to *this type of work*"
- **Projects** = "The actual instance of work we're doing right now"

Your job is to **create and evolve playbooks** so that future projects of the same type can follow a solid, repeatable process.

---

## 2. What a Good Playbook Contains

A playbook should answer:

> "If the user wants to perform this specific kind of analysis or project, what should happen in each Maestro phase, and how should the work be structured?"

At minimum, a playbook should cover:

1. **Domain Overview**
   - Brief explanation of the domain (e.g., type of evaluation or analysis)
   - Key concepts, entities (e.g., "requirements", "controls", "sections")
   - Mandatory constraints (e.g., no sampling, full coverage)

2. **Phase-by-Phase Strategy**
   Maestro has eight project phases (init, requirements, planning, list-building, task execution, quality assurance, verification/reporting, review).
   For each phase, the playbook should say:
   - What questions you should ask the user
   - What files or task sets should be created
   - What outputs should exist by the end of that phase

3. **Task Set Structure**
   - When to create task sets (e.g., one per major component, per document, or per aspect)
   - Path naming conventions (e.g., `analysis`, `analysis/security`, `qa`)
   - How task sets relate to the main project plan

4. **List Strategy**
   - What needs to be enumerated (e.g., every requirement, every clause, every section)
   - Whether to include **reusable lists** in the playbook (e.g., standard control frameworks)
   - How projects should use playbook lists (always copy to project first)
   - How to verify that all required items are captured (full coverage, no sampling)

5. **Task Design Strategy**
   - Types of tasks to create (e.g., "per-item analysis", "cross-check", "mapping tasks")
   - When to enable QA verification
   - How to use `list_create_tasks` to generate tasks from list items

6. **Runner Usage & Coverage**
   - How and when to use the automated runner
   - What should be auto-dispatched vs. manually reviewed
   - How to confirm every list item is associated with at least one completed task

7. **Reporting & Evidence**
   - What reports should be produced (using `task_report`)
   - Report format and filtering requirements
   - How to use results as an audit trail

---

## 3. Your Process for Creating a New Playbook

When the user wants to develop a playbook for a specific type of work, follow this process:

### 3.1 Discover the Domain & Goals

Ask the user:

- What is the **type of work**? (e.g., "conformance evaluation of X against Y")
- What are the **core documents** involved?
- What are the **mandatory rules**? (e.g., full coverage, types of findings)
- What **outputs** do they need? (e.g., detailed findings, summary, evidence listing)
- Are there **known steps or checklists** they already follow?
- What **title format** do they prefer for list items and tasks? Options include:
  - Section number and name (e.g., "4.1 Understanding the organization and its context")
  - Name only (e.g., "Understanding the organization and its context")
  - Number only (e.g., "4.1")
  - Custom format specific to their domain

Summarize this understanding and confirm with the user.

### 3.2 Create or Select a Playbook

Use `playbook_list` to see existing playbooks. If one for this domain doesn't exist:

- Create a new playbook: `playbook_create` with a clear name (e.g., `security-evaluation`, `compliance-audit`)

Decide on a primary playbook file, for example:

- `procedure.md` or
- `maestro_playbook.md`

### 3.3 Align with Maestro's Phases

For each phase, design domain-specific behavior:

1. **Project initialization**
   - How to name projects for this domain
   - Required metadata (title, description, scope)
   - Any initial project files to create
   - **Evidence import**: Ask the user if they have evidence files to import and use `file_import` to copy them into the project

2. **Requirements & understanding phase**
   - What "requirements" mean in this domain
   - Which documents define the requirements vs. which are being evaluated
   - Questions to clarify scope and constraints

3. **Planning phase**
   - How to decompose the work into task sets
   - What the project plan file should contain

4. **List-building phase**
   - How to build lists for this domain
   - Whether to include **reusable lists in the playbook**
   - **Important**: Document that projects must **copy playbook lists** using `list_copy`
   - How to verify that all items are captured
   - **Title format convention** for list items (document the user's preference from discovery)

5. **Task execution planning**
   - For each list, what tasks need to exist
   - Task set path organization (e.g., `analysis`, `analysis/security`)
   - Model selection rules (`llm_model_id`) for this domain
   - Whether to enable QA verification

6. **Execution, verification, and reporting**
   - How to use the runner for bulk tasks
   - How to summarize results using `task_report`
   - How to verify that every required item has been processed

### 3.4 Write the Playbook Document

Use `playbook_file_put` to create or update the main playbook document. A typical structure:

```
# <Playbook Name> – Maestro Playbook

## 1. Domain Overview
- Brief explanation of what this type of project is about
- Key entities (requirements, sections, controls, etc.)
- Mandatory constraints (e.g., full coverage, no sampling)

## 2. Project Structure in Maestro
- Recommended project naming
- When and how to create task sets
- Typical files in `projects/<project>/files/`

## 3. Phase-by-Phase Guidance

### 3.1 Phase 1 – Initialize Project
- Questions to ask the user when starting
- Fields to capture in project metadata
- Any standard files to create at the beginning

### 3.2 Phase 2 – Understand Requirements & Inputs
- How to interpret "requirements" in this domain
- Which documents define the source of truth
- Specific clarifications to obtain from the user

### 3.3 Phase 3 – Planning the Work
- How to decompose work into task sets
- Task set path conventions (e.g., `analysis`, `qa`)
- What goes into the main project plan file

### 3.4 Phase 4 – Build Lists
- Which documents need lists created from them
- What constitutes a list item (ID, content, source_doc, section)
- Whether the playbook provides reusable lists
- How to verify that all items are captured
- **Title format**: Document the chosen convention for item titles (e.g., "number and name", "name only", "number only")

**Title Format Convention**:

Unless the playbook already specifies a title format, ask the user for their preference before creating lists. Common formats:
- `"4.1 Understanding the organization"` – Section number + name (recommended for standards/frameworks)
- `"Understanding the organization"` – Name only
- `"4.1"` – Number only (when name is in content field)

Document the chosen format in the playbook so future projects follow the same convention.

**If this playbook includes reusable lists**:

> **Using Playbook Lists**: This playbook includes the `controls` list. Before starting work, copy it to your project:
> ```
> list_copy(from_list="controls", from_source="playbook", from_playbook="<this-playbook>", to_list="controls", to_project="<your-project>")
> ```

### 3.5 Phase 5 – Define and Execute Tasks
- Types of tasks (per-item analysis, cross-check tasks, summary tasks)
- Task set organization with path hierarchy
- Which tasks should be run via the runner
- Whether to enable QA verification for tasks

**Task Set Example**:
```
taskset_create(
  project="my-project",
  path="analysis",
  title="Analysis Tasks",
  description="Detailed requirement analysis"
)

list_create_tasks(
  project="my-project",
  path="analysis",
  list="requirements",
  type="analysis",
  title_template="Analyze {{id}}",
  prompt="Analyze this requirement...",
  llm_model_id="claude-sonnet",
  qa_enabled=true,
  qa_llm_model_id="claude-opus"
)
```

**Playbook Instructions File Path Format**:

When using `instructions_file_source="playbook"`, the path **MUST** include the playbook name:

- **Correct**: `"playbook-name/instructions/worker.md"`
- **Wrong**: `"instructions/worker.md"` ← causes error

### 3.6 Phase 6 – Quality Assurance
- How QA tasks should be structured for this domain
- What verification checks are needed
- Whether to use a different LLM for QA tasks
- How to handle issues found during QA

### 3.7 Phase 7 – Verify Completeness and Report
- How to confirm every list item is covered by at least one completed task
- How to generate reports using `task_report`
- Report format and filtering requirements
- How to document QA status

## 4. Quality Checks for This Playbook
- Checklist to ensure:
  - All required phases have domain-specific instructions
  - There is a clear list strategy
  - There is a clear task coverage strategy
  - Reporting expectations are defined
```

## 4. How to Evolve a Playbook Over Time

Once a playbook exists, treat it as a **living document**:

1. **Before starting a new project**
   - Read the relevant playbook via `playbook_file_get`
   - Ask the user whether anything has changed
   - Propose updates if needed

2. **After a project completes**
   - Ask the user what worked well and what was painful
   - Record improvements in the playbook
   - Update so future projects benefit

3. **Keep alignment with Maestro reference docs**
   - The playbook **extends** Maestro's concepts but should not contradict them

---

## 5. Task Sets in Playbooks

Maestro organizes tasks into **task sets** using hierarchical paths.

When designing a playbook, explicitly state:

- Whether multiple task sets are recommended
- Path naming conventions and purposes
- Which phase(s) they belong to

Examples of patterns:
- One task set per major document (e.g., `doc-a-analysis`, `doc-b-analysis`)
- One task set per analysis type (e.g., `analysis`, `cross-checks`)
- Separate task sets for work and QA (e.g., `analysis`, `qa`)

In the playbook, describe:
- **When** to call `taskset_create`
- **What** tasks belong in each task set
- **How** the main project plan references the task sets
- **Whether** tasks should run in parallel or sequentially

**Creating a Task Set**:
```
taskset_create(
  project="my-project",
  path="analysis",
  title="Analysis Tasks",
  description="Detailed item analysis tasks",
  parallel=true  # Enable parallel execution (uses config's max_concurrent)
)
```

**Parallel vs Sequential**:
- Use `parallel=true` when tasks are independent and can run concurrently
- Use `parallel=false` (default) when tasks must run sequentially (e.g., building indexes, L2 orchestration tasks)
- Override at runtime with `task_run(..., parallel="true")` or `parallel="false"`

---

## 6. Ensuring Full Coverage in Playbooks

For domains where full coverage (no sampling) is required, the playbook should clearly describe:

- **What "full coverage" means** in that domain
- **Which lists implement that coverage**
- **Which task sets implement that coverage** (via `list_create_tasks`)
- **How to verify coverage**:
  - Compare list item count to expected counts in source
  - Use `task_status` to confirm all tasks are completed
  - Use `task_report` to generate coverage reports

**For playbooks with reusable lists**: Emphasize that projects must copy playbook lists before use.

---

## 7. Multi-Tier Execution Architecture

Complex projects require more setup work than a single LLM session can reliably complete. Lists must be extracted from documents, task sets created, tasks generated from lists, QA configured, and runners launched. When an orchestrating LLM tries to do all of this directly, it often fails to complete—leaving the project in a partially-configured state.

The solution is a **multi-tier architecture** where each tier sets up work for the tier below, then delegates execution to the runner.

### 7.1 The Problem: Orchestrator Overwhelm

When a single LLM session tries to:
- Parse documents and extract dozens of items into lists
- Create multiple task sets with different purposes
- Generate hundreds of tasks from lists
- Configure QA for each task type
- Run everything and collect results

...it frequently runs out of context, loses track of progress, or simply stops mid-way. The project ends up partially complete with no clear path to resume.

### 7.2 The Solution: Three-Tier Delegation

Use a **three-tier architecture** where each tier has a focused responsibility:

| Tier | Role | Responsibility | Exits? |
|------|------|----------------|--------|
| **Tier 1 (Orchestrator)** | Project Manager | Talk to user, create project, create T2 task list, start runner | Maybe not (interactive session) |
| **Tier 2 (Setup Workers)** | Section Leads | Extract lists, copy playbook resources, create task sets, create T3 tasks, start runner | Always |
| **Tier 3 (Analysis Workers)** | Specialists | Execute focused analysis on single items | Always |

**Key insight**: Each tier creates tasks for the tier below, ensures QA is configured, starts the runner, and exits (or returns to user conversation for T1). The runner handles all execution, retries, and QA iterations.

### 7.3 Tier Responsibilities in Detail

**Tier 1 (Orchestrator)**:
- Discusses requirements with the user
- Creates the project and initial structure
- Creates a task set for T2 work (e.g., `setup/`)
- Creates T2 tasks with detailed instructions and QA enabled
- Starts the runner for T2 tasks
- Creates a final project QA/verification task
- Handles restart/recovery scenarios (with user help if needed)
- May remain active in conversation with user, or exit

**Tier 2 (Setup Workers)**:
- Execute setup tasks like:
  - Extract items from documents → create lists
  - Copy playbook lists to project
  - Create task sets for T3 work
  - Generate T3 tasks from lists (with QA enabled)
  - Start the runner for T3 tasks
- Each T2 task should have QA to verify its setup work
- Always exit after starting the runner

**Tier 3 (Analysis Workers)**:
- Execute focused, single-item analysis
- Example: "Check if SFR X exists in the ST"
- Produce structured output for reporting
- QA verifies accuracy of analysis
- Always exit after completing work

### 7.4 The Runner is Trustworthy

The runner handles:
- **Sequential or parallel execution** as configured
- **Automatic retries** on transient failures
- **QA iterations** when issues are found
- **Status tracking** for all tasks

**Trust the runner completely.** When a tier sets up tasks and starts the runner, the tier's job is done. If the runner executes successfully, the project completes correctly.

When the runner marks a task as:
- **done**: The task completed successfully (including QA if enabled)
- **failed**: The task failed after all retry attempts

There is nothing more that can be done programmatically. Failed tasks require human review. Maestro writes reports to disk; future versions will add user notifications.

### 7.5 QA Cascades Downward

Each tier is responsible for ensuring the tier below has appropriate QA:

**T1 configures QA for T2**:
```
task_create(
  project="my-project",
  path="setup",
  title="Extract SFRs from ST",
  prompt="Extract all SFRs from the Security Target...",
  qa_enabled=true,
  qa_prompt="Verify the extracted list is complete and accurate..."
)
```

**T2 configures QA for T3**:
```
list_create_tasks(
  project="my-project",
  path="analysis",
  list="sfrs",
  type="sfr-check",
  prompt="Check if this SFR exists in the ST...",
  instructions_file="my-playbook/instructions/check_sfr.md",
  instructions_file_source="playbook",
  qa_enabled=true,
  qa_instructions_file="my-playbook/instructions/qa_sfr_check.md",
  qa_instructions_file_source="playbook"
)
```

**If QA instructions don't exist in the playbook**, T2 should create them or use inline `qa_prompt`.

### 7.6 Fire and Forget

Tiers do **not** poll for status. They:
1. Set up a complete list of tasks
2. Ensure QA is configured
3. Start the runner
4. Exit (T2/T3) or return to user conversation (T1)

The goal is: **if the runner executes successfully, the project completes correctly.**

This means each tier must:
- Provide complete, detailed instructions to the tier below
- Include all necessary context (document paths, list names, expected outputs)
- Configure appropriate QA to catch errors
- Trust that the runner will handle execution

### 7.7 When to Use Two Tiers Instead

For **very simple projects**, a two-tier architecture may suffice:

- **T1**: Creates project, creates tasks directly (no setup tasks)
- **T2**: Executes analysis tasks

Use two tiers when:
- There's only one list to process
- The list already exists (no extraction needed)
- Task creation is straightforward (single `list_create_tasks` call)

Use three tiers (the default) when:
- Multiple lists must be extracted from documents
- Multiple task sets with different purposes
- Complex setup that benefits from QA verification

### 7.8 Handling Restarts and Recovery

T1 is responsible for handling restart scenarios:

1. **Check project state** on resume:
   - Use `task_status` to see what's complete/failed/stuck
   - Use `project_log_get` to understand what happened

2. **Identify stuck tasks**:
   - Tasks in `processing` status with no active runner may be stuck
   - Reset stuck tasks: `task_update(project="...", uuid="...", work_status="waiting")`

3. **Handle failures**:
   - Review failed task results
   - Discuss with user what went wrong
   - Create corrective tasks or adjust approach

4. **Resume execution**:
   - Call `task_run` to continue with remaining tasks

When in doubt, ask the user for guidance. Recovery scenarios can be complex and may require human judgment.

### 7.9 Designing Playbooks for Multi-Tier Execution

When authoring a playbook, structure it around the three tiers:

1. **Document T2 tasks** the orchestrator should create:
   - What setup work is needed?
   - What lists must be extracted or copied?
   - What task sets must be created?

2. **Provide T2 instruction files**:
   - `instructions/setup_*.md` - Instructions for setup workers
   - Include Maestro tool usage examples
   - Specify expected outputs

3. **Provide T3 instruction files**:
   - `instructions/analyze_*.md` - Instructions for analysis workers
   - `instructions/qa_*.md` - QA verification instructions

4. **Document the task set structure**:
   - `setup/` - T2 setup tasks
   - `analysis/` - T3 analysis tasks
   - Path conventions for different work types

---

## 8. Evidence Handling in Playbooks

For audits, evaluations, and assessments, **evidence** is the source material that workers analyze. Proper evidence handling is critical for accurate results.

### 8.1 What is Evidence?

Evidence includes:
- Documents being evaluated (policies, procedures, technical specs)
- Screenshots or logs demonstrating implementation
- Configuration files or code samples
- Interview notes or questionnaire responses
- External reference documents (standards, frameworks)

### 8.2 Evidence Location Patterns

Evidence can be stored in different locations:

| Location | Use When | Access Pattern |
|----------|----------|----------------|
| **Project files** | Evidence specific to this project | `project_file_get(project="...", path="evidence/...")` |
| **Imported files** | Evidence imported from external location | `project_file_get(project="...", path="imported/...")` |
| **Playbook files** | Reference docs shared across projects | `playbook_file_get(playbook="...", path="docs/...")` |
| **External paths** | Large files the user manages outside Maestro | User confirms path; worker accesses directly |

### 8.2.1 Importing Evidence with file_import

The **recommended approach** for evidence is to import it into the project using `file_import`. This:
- Creates a copy of evidence specific to this project
- Allows workers to use standard `project_file_*` tools
- Preserves symlinks (common in evidence archives)
- Isolates the project from changes to original files

```
file_import(
  source="/path/to/evidence/directory",
  project="my-audit",
  recursive=true
)
```

**Important notes:**
- Files are imported to `<project>/files/imported/`
- Workers access via `project_file_get(project="...", path="imported/...")`
- Symlinks are preserved (evidence archives often use symlinks for shared documents)
- Evidence should be in readable formats (text, markdown) - use `project_file_convert` to convert PDF/DOCX/XLSX to Markdown
- **Conversion note**: The x2md library is optimized for LLM consumption. Complex layouts and formatting may not be fully preserved in Markdown output.

**Always ask the user**: "Do you have evidence files to import for this project? If so, please provide the path."

### 8.3 Evidence Workflow

**Phase 01 (Project Initiation)**:
1. Ask user what evidence they have and where it's located
2. **Import evidence** using `file_import` if the user provides a path
3. Document evidence locations in `evidence-manifest.md`
4. Note any additional evidence that needs to be uploaded

**Phase 02 (Requirements)**:
1. **Verify** each evidence source is accessible before proceeding
2. For project files: `project_file_list` to confirm files exist
3. For playbook files: `playbook_file_list` to confirm files exist
4. For external paths: Confirm with user that files are in place
5. Update manifest with verification status
6. **Do NOT create tasks until evidence is verified**

**Task Execution**:
1. Include evidence paths in task prompts
2. Workers access evidence during analysis
3. Workers cite evidence in their outputs

### 8.4 Including Evidence in Task Prompts

Maestro assembles task prompts from multiple sources:

```
┌─────────────────────────────────────────────────────────────┐
│                    Final Task Prompt                        │
├─────────────────────────────────────────────────────────────┤
│  1. instructions_file content (from playbook/project/ref)   │
│  2. instructions_text (inline additions)                    │
│  3. prompt (task-specific text)                             │
│  4. list item context (if created from list)                │
└─────────────────────────────────────────────────────────────┘
```

Evidence paths can be included in any of these layers.

### 8.5 Two Patterns for Evidence Integration

#### Pattern A: Copy & Modify

Copy the playbook's instruction file to the project and modify it with project-specific evidence paths:

```
# 1. Copy playbook instructions to project
file_copy(
  from_source="playbook",
  from_playbook="security-audit",
  from_path="instructions/worker.md",
  to_source="project",
  to_project="my-audit",
  to_path="instructions/worker.md"
)

# 2. Edit the project copy to add evidence paths
project_file_edit(
  project="my-audit",
  path="instructions/worker.md",
  old_string="## Evidence Location",
  new_string="## Evidence Location\n\nEvidence files are located at:\n- `files/evidence/policies/` - Policy documents\n- `files/evidence/screenshots/` - Implementation evidence"
)

# 3. Create tasks using the project's instructions
list_create_tasks(
  project="my-audit",
  path="analysis",
  list="controls",
  instructions_file="instructions/worker.md",
  instructions_file_source="project",  # Uses project copy
  ...
)
```

**Best for**:
- Projects with complex evidence structures
- When evidence paths need extensive documentation
- When you want a permanent record of project-specific instructions

#### Pattern B: Layer

Use the playbook's instructions directly and add evidence info via `instructions_text`:

```
list_create_tasks(
  project="my-audit",
  path="analysis",
  list="controls",
  instructions_file="security-audit/instructions/worker.md",
  instructions_file_source="playbook",  # Uses playbook directly
  instructions_text="## Project-Specific Evidence\n\nEvidence files for this audit:\n- Policy documents: `files/evidence/policies/`\n- Screenshots: `files/evidence/screenshots/`\n\nReview ALL relevant evidence before making your assessment.",
  ...
)
```

**Best for**:
- Simpler evidence structures
- Quick project setup
- When playbook instructions don't need modification

### 8.6 Playbook Design for Evidence

When authoring playbooks, design instructions to accommodate evidence:

**Worker instruction template pattern**:
```markdown
# Analysis Instructions

## Task
[What the worker should do]

## Evidence Location

**Important**: The orchestrator will specify evidence locations in the project-specific instructions. Look for evidence in the paths provided.

[Placeholder for evidence paths - to be filled by orchestrator]

## How to Use Evidence
1. Read all relevant evidence files before making assessments
2. Cite specific documents and sections in your findings
3. If evidence is missing, note it explicitly

## Output Format
[Schema-compliant JSON format]
```

**The placeholder approach** lets playbooks provide general guidance while allowing each project to specify its unique evidence structure.

### 8.7 Evidence Verification Checklist

Before creating tasks, verify:

- [ ] All evidence files listed in manifest are accessible
- [ ] File paths in task prompts are correct
- [ ] Workers can access evidence from their execution context
- [ ] Evidence format matches what workers expect (PDF vs text, etc.)

---

## 9. Prompt Assembly Reference

This section provides a quick reference for how Maestro assembles prompts from different sources.

### 9.1 Prompt Components

| Component | Parameter | Source Options | Use For |
|-----------|-----------|----------------|---------|
| Instructions file | `instructions_file` | `project`, `playbook`, `reference` | Detailed reusable instructions |
| Inline instructions | `instructions_text` | (inline string) | Project-specific additions |
| Task prompt | `prompt` | (inline string) | Task-specific guidance |
| List item context | (automatic) | From source list | Item-specific data |

### 9.2 Assembly Order

The final prompt sent to the worker LLM is assembled in this order:

```
1. instructions_file content (if specified)
2. instructions_text (if specified)
3. prompt (if specified)
4. list item context (if task created from list):
   - Item ID
   - Item title
   - Item content
   - Item metadata (source_doc, section)
```

### 9.3 Common Patterns

**Pattern 1: Playbook instructions only**
```
task_create(
  instructions_file="my-playbook/instructions/worker.md",
  instructions_file_source="playbook"
)
```

**Pattern 2: Playbook instructions + inline additions**
```
task_create(
  instructions_file="my-playbook/instructions/worker.md",
  instructions_file_source="playbook",
  instructions_text="Evidence location: files/evidence/\nFocus area: Security controls"
)
```

**Pattern 3: Project instructions (copied and modified from playbook)**
```
task_create(
  instructions_file="instructions/worker.md",
  instructions_file_source="project"
)
```

**Pattern 4: Prompt only (simple tasks)**
```
task_create(
  prompt="Analyze this item for compliance with control requirement 3.4..."
)
```

**Pattern 5: Reference instructions (built-in defaults)**
```
task_create(
  instructions_file="templates/default-worker-instructions.md",
  instructions_file_source="reference"
)
```

---

## 10. Using This Guide

When asked to create or refine a playbook:

1. **Read this guide** from the reference domain
2. **Ask the user** about domain, documents, constraints, outputs
3. **Design the playbook** following the structure above
4. **Write it** using `playbook_file_put`
5. **Reference it** from future project sessions

Your goal is to turn user expertise + Maestro's orchestration model into **reusable, phase-aligned instructions** that make each new project easier, safer, and more thorough.

---

## 11. Preventing Hallucinations and Ensuring Accuracy

LLMs can produce outputs that appear correct but contain fabricated information. When designing playbooks, include explicit guidance to prevent hallucinations in **both worker instructions and QA verification procedures**.

### 11.1 Understanding the Hallucination Problem

Worker LLMs can produce:

- **Fabricated evidence**: Citing sections or quotes that don't exist
- **Incorrect references**: Pointing to the wrong section for a claim
- **Made-up quotes**: Text presented as quotes that isn't in the source
- **Overstated conclusions**: Claiming certainty when evidence is ambiguous

### 11.2 Designing Worker Instructions

When creating task instructions, include explicit requirements:

**Citation Requirements**:
```markdown
For every factual claim, provide:
- **Document**: [exact document name and version]
- **Location**: [section number, page number]
- **Quote**: "[exact text from document]"
- **Analysis**: [your interpretation, clearly separate]
```

**Quote Accuracy**:
```markdown
When quoting source material:
- Copy text EXACTLY as it appears
- Use quotation marks to indicate quoted text
- NEVER paraphrase and present as direct quote
```

**Fact vs. Analysis Separation**:
```markdown
Structure each output as:

**Evidence** (facts from source):
- Document X, Section Y states: "[exact quote]"

**Analysis** (your interpretation):
- This indicates that [interpretation]

**Verdict**: [conclusion]
```

### 11.3 Designing QA Verification

QA tasks must independently verify outputs against source documents.

**QA Task Structure**:
```markdown
**Verification steps**:

1. **Evidence verification**:
   - For each citation, locate the referenced document and section
   - Verify the section/page number exists
   - Check that quotes match the source exactly

2. **Reference accuracy**:
   - Verify each claim is supported by the cited evidence
   - Confirm no claims are made without citations

3. **Hallucination detection**:
   - Flag any section numbers or quotes that don't exist
   - Flag any claims not supported by cited evidence
```

**QA Output Format**:
```markdown
QA Verdict: VERIFIED | ISSUES_FOUND | CRITICAL_ERROR

Evidence Checks:
- Citation 1: CONFIRMED | NOT_FOUND | MISQUOTED
- Citation 2: CONFIRMED | NOT_FOUND | MISQUOTED

Hallucinations Detected:
- [any fabricated evidence]

Recommendation: ACCEPT | REVISE | REJECT
```

### 11.4 Using Different LLMs for QA

**Best practice**: Use a different model for QA when possible:

```
task_create(
  ...
  llm_model_id="claude-sonnet",
  qa_enabled=true,
  qa_llm_model_id="claude-opus"
)
```

Different models have different failure modes, making independent verification more reliable.

### 11.5 Handling QA Findings

**VERIFIED**: Output is accurate
- Proceed to final reporting

**ISSUES_FOUND**: Minor problems detected
- Document issues
- Create revised task if needed
- Re-run QA on corrected output

**CRITICAL_ERROR**: Major hallucination
- Flag for human review
- Do NOT include in final deliverables
- Document error pattern for prevention

---

**Remember**: The cost of undetected hallucinations in final deliverables far exceeds the cost of verification. Build accuracy measures into every playbook from the start.

---

## 12. Mandatory JSON Schemas and Markdown Templates

**All playbooks MUST define JSON schemas for worker outputs.** Maestro validates every worker response against the schema and rejects non-conforming responses. This ensures consistent, structured output that can be automatically transformed into reports.

### 12.1 Why JSON Schemas Are Mandatory

JSON schemas provide:

1. **Structured Output**: Workers produce predictable, parseable data
2. **Validation**: Maestro rejects malformed responses before they pollute results
3. **Automated Reporting**: Results can be automatically transformed into human-readable reports
4. **QA Consistency**: QA workers know exactly what fields to verify
5. **Cross-Task Compatibility**: All tasks in a playbook produce compatible output

**Without schemas**, worker output varies unpredictably, reports require manual assembly, and QA cannot systematically verify fields.

### 12.2 Schema Validation Flow

When a worker completes a task:

```
┌─────────────┐     ┌──────────────┐     ┌────────────────┐
│   Worker    │────▶│   Validate   │────▶│ Store Result   │
│  Response   │     │   vs Schema  │     │ (if valid)     │
└─────────────┘     └──────┬───────┘     └────────────────┘
                          │
                          │ Invalid
                          ▼
                   ┌──────────────┐     ┌────────────────┐
                   │ Return Error │────▶│ Worker Retries │
                   │  to Worker   │     │ with Feedback  │
                   └──────────────┘     └────────────────┘
```

1. **Worker produces JSON response**
2. **Maestro validates against schema**
3. **If invalid**: Error returned to worker with specific validation failures
4. **Worker corrects and resubmits** (within retry limits)
5. **If valid**: Result stored and available for reporting

### 12.3 Required Playbook Files

Every playbook MUST include these files in the `schemas/` directory:

| File | Purpose |
|------|---------|
| `schemas/worker_response.json` | JSON Schema for worker task output |
| `schemas/qa_response.json` | JSON Schema for QA verification output |
| `templates/worker_report.md` | Markdown template for worker results in reports |
| `templates/qa_report.md` | Markdown template for QA results in reports |

**Field names MUST match** between JSON schemas and markdown templates.

### 12.4 JSON Schema Structure

Worker response schemas should follow this pattern:

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "$id": "playbook-name/worker_response",
  "title": "Worker Response Schema",
  "description": "Schema for worker task output in this playbook",
  "type": "object",
  "properties": {
    "item_id": {
      "type": "string",
      "description": "Identifier for the item being evaluated"
    },
    "status": {
      "type": "string",
      "enum": ["complete", "information required", "review required"],
      "description": "Assessment status"
    },
    "test": {
      "type": "string",
      "description": "Brief description of testing methodology"
    },
    "evidence": {
      "type": "array",
      "description": "Structured list of evidence documents",
      "items": {
        "type": "object",
        "required": ["num", "document"],
        "properties": {
          "num": { "type": "integer", "description": "Evidence item number" },
          "document": { "type": "string", "description": "Document name" },
          "section": { "type": "string", "description": "Section within document" },
          "path": { "type": "string", "description": "Full path for QA verification" }
        }
      }
    },
    "observations": {
      "type": "string",
      "description": "Brief observations about the finding"
    },
    "requests": {
      "type": "string",
      "description": "Information requests if status is 'information required'"
    },
    "summary": {
      "type": "string",
      "description": "Brief summary for reports"
    },
    "rationale": {
      "type": "string",
      "description": "Internal reasoning with evidence path references for QA"
    }
  },
  "required": ["item_id", "status", "summary", "rationale"],
  "additionalProperties": false
}
```

**Key schema elements:**

- **`enum`**: Constrain status fields to valid values
- **`required`**: Ensure critical fields are present
- **`additionalProperties: false`**: Reject unexpected fields
- **`description`**: Guide workers on field purpose
- **`evidence` array**: Structured format with paths enables QA verification

### 12.5 QA Response Schema

QA schemas verify the worker's output. **Document verification is mandatory** - QA must check each evidence item cited by the worker:

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "$id": "playbook-name/qa_response",
  "title": "QA Response Schema",
  "type": "object",
  "properties": {
    "item_id": {
      "type": "string",
      "description": "Identifier being verified"
    },
    "verdict": {
      "type": "string",
      "enum": ["pass", "fail", "escalate"],
      "description": "QA verdict: pass = work acceptable, fail = send back to worker, escalate = cannot be resolved by QA"
    },
    "document_verification": {
      "type": "array",
      "description": "Verification of each evidence document cited by worker",
      "items": {
        "type": "object",
        "required": ["evidence_num", "document", "exists", "supports_finding"],
        "properties": {
          "evidence_num": { "type": "integer", "description": "Evidence item number from worker" },
          "document": { "type": "string", "description": "Document name" },
          "section": { "type": "string", "description": "Section referenced" },
          "path": { "type": "string", "description": "Path to document" },
          "exists": { "type": "boolean", "description": "Document found at path" },
          "supports_finding": { "type": "boolean", "description": "Content supports worker's claim" },
          "notes": { "type": "string", "description": "Verification notes" }
        }
      }
    },
    "comments": {
      "type": "string",
      "description": "Required summary of QA verification work (must always be provided)"
    },
    "issues": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["type", "description"],
        "properties": {
          "type": { "type": "string", "enum": ["evidence_not_found", "unsupported_claim", "example_value_used", "missing_path", "other"] },
          "evidence_num": { "type": "integer" },
          "description": { "type": "string" },
          "severity": { "type": "string", "enum": ["critical", "major", "minor"] }
        }
      }
    }
  },
  "required": ["item_id", "verdict", "document_verification", "comments"]
}
```

**Automatic QA failure conditions:**
- Document cited in evidence cannot be found at the specified path
- Section does not contain content supporting the worker's claim
- Worker used example/placeholder values (e.g., "DocumentName.docx", "Section 42")
- Evidence paths are missing or incomplete

### 12.6 Markdown Templates

Markdown templates transform JSON results into human-readable report sections. They use Go template syntax with field names matching the JSON schema.

**Worker report template (`templates/worker_report.md`):**

```markdown
### {{.item_id}}

**Status**: {{.status}}

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

{{.summary}}

**Evidence Paths** (for QA verification)
{{range .evidence}}
{{.num}}. {{.path}}, {{.section}}
{{end}}
```

**QA report template (`templates/qa_report.md`):**

```markdown
### QA: {{.item_id}}

**Verdict**: {{.verdict}}

**Document Verification**
| # | Document | Section | Exists | Supports |
|---|----------|---------|--------|----------|
{{range .document_verification}}| {{.evidence_num}} | {{.document}} | {{.section}} | {{if .exists}}✅{{else}}❌{{end}} | {{if .supports_finding}}✅{{else}}❌{{end}} |
{{end}}

**QA Comments**
{{.comments}}

{{if .issues}}
**Issues Found**
{{range .issues}}
- [{{.type}}] {{.description}}{{if .severity}} ({{.severity}}){{end}}
{{end}}
{{end}}
```

### 12.7 Field Matching Requirements

**Critical**: JSON schema field names MUST match template placeholders.

| JSON Schema Field | Template Placeholder |
|-------------------|---------------------|
| `"item_id"` | `{{.item_id}}` |
| `"status"` | `{{.status}}` |
| `"evidence[].num"` | `{{.num}}` (in range) |
| `"evidence[].document"` | `{{.document}}` (in range) |
| `"evidence[].path"` | `{{.path}}` (in range) |
| `"document_verification[].exists"` | `{{.exists}}` (in range) |
| `"issues[].type"` | `{{.type}}` (in range) |

**Common errors:**

- Schema uses `item_id` but template uses `{{.itemId}}` → **Mismatch**
- Schema has `status` but template uses `{{.verdict}}` → **Mismatch**
- Schema array field `issues` but template expects `findings` → **Mismatch**
- Schema has `document_verification` but template uses `{{.evidence_checks}}` → **Mismatch**

### 12.8 Specifying Schemas in Tasks

When creating tasks, reference the playbook schemas:

```
task_create(
  project="my-project",
  path="analysis",
  title="Analyze REQ-001",
  prompt="...",
  worker_response_schema="my-playbook/schemas/worker_response.json",
  worker_response_schema_source="playbook",
  qa_enabled=true,
  qa_response_schema="my-playbook/schemas/qa_response.json",
  qa_response_schema_source="playbook"
)
```

Or set defaults at the task set level:

```
taskset_create(
  project="my-project",
  path="analysis",
  title="Analysis Tasks",
  worker_response_schema="my-playbook/schemas/worker_response.json",
  worker_response_schema_source="playbook",
  worker_report_template="my-playbook/templates/worker_report.md",
  worker_report_template_source="playbook",
  qa_response_schema="my-playbook/schemas/qa_response.json",
  qa_response_schema_source="playbook",
  qa_report_template="my-playbook/templates/qa_report.md",
  qa_report_template_source="playbook"
)
```

### 12.9 Worker Instructions Must Reference Schema

Worker instruction files MUST tell the worker to produce JSON matching the schema:

```markdown
## Output Format

**You MUST respond with valid JSON matching the schema in `schemas/worker_response.json`.**

```json
{
  "item_id": "<copy from task>",
  "verdict": "Pass | Fail | Partial | N/A",
  "evidence": "<document/section references>",
  "summary": "<brief summary for report>",
  "rationale": "<internal explanation for QA>"
}
```

**Field requirements:**
- `item_id`: Copy exactly from the task context
- `verdict`: Must be one of the allowed values
- `evidence`: Cite specific documents and sections
- `summary`: Professional, suitable for client delivery
- `rationale`: Internal notes for QA (not in final report)

**Your entire response must be valid JSON. No markdown, no additional text.**
```

### 12.10 Validation Error Handling

When Maestro rejects a response, the worker receives feedback:

```
VALIDATION ERROR: Response does not match required schema.

Errors:
- Field 'verdict': Value 'Maybe' is not in enum [Pass, Fail, Partial, N/A]
- Field 'rationale': Required field missing

Expected schema: schemas/worker_response.json

Please correct your response and resubmit valid JSON.
```

The worker should:
1. Read the specific validation errors
2. Correct the JSON structure
3. Resubmit a valid response
4. If unable to correct, produce best-effort JSON with required fields

### 12.11 Playbook Checklist

Before deploying a playbook, verify:

- [ ] `schemas/worker_response.json` exists and is valid JSON Schema
- [ ] `schemas/qa_response.json` exists and is valid JSON Schema
- [ ] `templates/worker_report.md` exists with matching field names
- [ ] `templates/qa_report.md` exists with matching field names
- [ ] Worker instructions reference the schema and provide examples
- [ ] QA instructions reference the QA schema
- [ ] Field names are consistent across all files
- [ ] Required fields make sense for the domain
- [ ] Enum values cover all valid options

### 12.12 Default Schemas

If a playbook doesn't specify schemas, Maestro uses the default schemas from `reference/templates/`. However, **all production playbooks should define domain-specific schemas** for proper validation and reporting.

### 12.13 Multi-Template Reports

By default, a single worker report template produces one report file. For projects requiring multiple report variants (e.g., client-facing vs internal), use a **JSON manifest** instead of a single `.md` template.

**Creating a template manifest (`templates/worker-reports.json`):**

```json
[
  {"suffix": "Report", "file": "worker-report-client.md"},
  {"suffix": "Internal", "file": "worker-report-internal.md"}
]
```

For QA templates, create a corresponding `templates/qa-reports.json`:

```json
[
  {"suffix": "Report", "file": "qa-report-client.md"},
  {"suffix": "Internal", "file": "qa-report-internal.md"}
]
```

**How it works:**
- If `worker_report_template` ends in `.json`, Maestro parses it as a manifest
- If it ends in `.md`, Maestro uses it as a single template (backwards compatible)
- Each manifest entry specifies a `suffix` (report filename suffix) and `file` (template path)
- Template file paths are relative to the manifest location

**Using the manifests in a task set:**

```
taskset_create(
  project="my-project",
  path="analysis",
  title="Analysis Tasks",
  worker_report_template="my-playbook/templates/worker-reports.json",
  qa_report_template="my-playbook/templates/qa-reports.json"
)
```

**Generated report files:**
- `<prefix>Report.md` - Client-facing report (clean, professional)
- `<prefix>Internal.md` - Internal report (full details for team review)

**Recommended template file naming:**

| File | Content | Audience |
|------|---------|----------|
| `worker-report-client.md` | Professional findings, evidence citations | Client delivery |
| `worker-report-internal.md` | Full details including rationale, evidence paths, QA data | Internal team |
| `qa-report-client.md` | Minimal or empty (QA details not for clients) | Client delivery |
| `qa-report-internal.md` | Full QA verification, document checks, issues | Internal team |

**Example: Client template (clean output):**

```markdown
### {{.section}} {{.title}}

| **Status**: {{.status}} | **Task**: {{._task_id}} |

**Requirement**
{{.requirement}}

**Evidence**
{{if .evidence}}{{range .evidence}}
{{.num}}. {{.document}}, {{.section}}
{{end}}{{else}}None{{end}}

**Observations**
{{if .observations}}{{.observations}}{{else}}None{{end}}
```

**Example: Internal template (full details):**

```markdown
### {{.section}} {{.title}}

| **Status**: {{.status}} | **QA**: {{._qa_verdict}} | **Task**: {{._task_id}} |

**Requirement**
{{.requirement}}

**Evidence**
{{if .evidence}}{{range .evidence}}
{{.num}}. {{.document}}, {{.section}}
{{end}}{{else}}None{{end}}

**Observations**
{{if .observations}}{{.observations}}{{else}}None{{end}}

---

**For Internal Review**

**Rationale**
{{if .rationale}}{{.rationale}}{{else}}None{{end}}

**Evidence Locations**
{{if .evidence}}{{range .evidence}}
{{.num}}. `{{.path}}` — {{.document}}, {{.section}}
{{end}}{{else}}None{{end}}

**QA Comments**
{{if ._qa_result.comments}}{{._qa_result.comments}}{{else}}None{{end}}
```

**Playbook instructions for multi-template:**

In your playbook, instruct the LLM to use the manifests:

> When creating the task set, use `worker_report_template="my-playbook/templates/worker-reports.json"` and `qa_report_template="my-playbook/templates/qa-reports.json"` to generate both client-facing and internal report variants automatically.

---

## 13. Playbook Best Practices and Lessons Learned

This section captures practical lessons from creating real playbooks. Follow these guidelines to avoid common pitfalls.

### 13.1 External Reference Documents Are Critical

**Problem**: Derived documents (checklists, summaries, spreadsheets) often contain paraphrased or abbreviated content that doesn't match the authoritative source.

**Solution**: Always obtain and reference the **authoritative source document**:

- For security audits → the actual standard or framework document
- For compliance evaluations → the official framework documentation
- For assessments → the authoritative requirements specification

**Example**: A checklist CSV might say "Determine external and internal issues" but the authoritative standard says "4.1 Understanding the organization and its context". The playbook must use the exact standard terminology.

**Best practice**: Store the authoritative source document path in the playbook documentation so workers can reference it.

### 13.2 List Item Structure

List items have two key text fields that serve different purposes:

| Field | Purpose | Example |
|-------|---------|---------|
| `title` | Short display name for reports and task lists | "4.1 Understanding the organization and its context" |
| `content` | Full details for worker context | "**Section:** 4.1\n**Requirement:** Did the organization determine external and internal issues?\n**Assessment Guidance:** Review documentation..." |

**Best practices**:

- **Title**: Should identify the item clearly. Include section numbers for standards-based work.
- **Content**: Should contain everything a worker needs to perform the task without additional lookups.
- **source_doc** and **section**: Use these metadata fields to track where items came from.

### 13.3 Multiple Checklist Items Per Control

A source standard may have fewer items than the checklist derived from it:

- A security framework might have **70 controls**
- A thorough audit checklist might have **130 items or areas of focus** (multiple questions per control)

**This is expected and correct.** Multiple checklist items can share the same control title:

```
item-12: "3.15 Change management" → "Is there a change management policy?"
item-13: "3.15 Change management" → "Are there formal change control procedures?"
item-14: "3.15 Change management" → "Are critical applications tested after changes?"
```

**Document this in the playbook** so users understand the relationship between source controls and checklist items.

### 13.4 Verification is Non-Negotiable

**Always verify** list contents before proceeding:

1. **Count verification**: Does the list item count match the expected count from the source?
2. **Sample verification**: Spot-check 5-10 items to confirm titles and content are correct
3. **Coverage verification**: Are all sections/controls from the source represented?

**Verification commands**:
```
list_get_summary(list="controls", source="playbook", playbook="my-playbook")
# Check item_count matches expected

list_item_get(list="controls", id="item-001", source="playbook", playbook="my-playbook")
# Spot-check individual items
```

### 13.5 Playbook Naming Conventions

Use descriptive names that indicate both the domain and the type of work:

| Good | Bad | Why |
|------|-----|-----|
| `security-audit` | `security` | Clarifies this is for audits, not general security work |
| `compliance-assessment` | `compliance` | Too generic; multiple assessment types exist |
| `vendor-review` | `review` | Specifies what is being reviewed |

**Naming rules**:
- Use lowercase with hyphens
- Include the domain or work type
- Include the activity (audit, assessment, review, check)
- Keep it concise but unambiguous

### 13.6 Handling User Uncertainty

Users may not know what they need. **Ask clarifying questions**, but don't wait indefinitely for answers you can reasonably infer.

**Questions to always ask**:
- What is the authoritative source document?
- What format do you want for item titles? (number + name, name only, etc.)
- Are there any items that should be excluded from scope?
- What output format do you need for the final report?

**Decisions you can make**:
- Standard playbook directory structure
- JSON schema field names (use consistent conventions)
- Task set path organization
- QA verification approach

### 13.7 Iterative Refinement

Playbooks improve through use. After each project:

1. **Ask what worked** and what was painful
2. **Update instructions** based on worker performance
3. **Refine schemas** if fields were missing or unused
4. **Improve templates** based on report feedback
5. **Document lessons** in the playbook itself

**Version your playbooks** by noting significant changes in a changelog section.

---

## 14. Standard Playbook Directory Structure

Every playbook should follow a consistent directory structure. This makes playbooks predictable and easier to maintain.

### 14.1 Required Structure

```
playbook-name/
├── procedure.md              # Main playbook document (how to use this playbook)
├── schemas/
│   ├── worker_response.json  # JSON schema for worker output
│   └── qa_response.json      # JSON schema for QA output
├── templates/
│   ├── worker_response.md    # Report template for worker results
│   └── qa_response.md        # Report template for QA results
├── instructions/
│   ├── worker.md             # Instructions for analysis workers
│   └── qa.md                 # Instructions for QA workers
└── lists/                    # Optional: reusable lists
    └── controls.json         # Example: standard controls list
```

### 14.2 Directory Purposes

| Directory | Purpose | When to Use |
|-----------|---------|-------------|
| `schemas/` | JSON Schema definitions for structured output | **Always** - every playbook needs schemas |
| `templates/` | Markdown templates for report generation | **Always** - needed for `task_report` |
| `instructions/` | Detailed instructions for worker LLMs | **Always** - guides worker behavior |
| `lists/` | Reusable lists (controls, requirements, etc.) | When the playbook covers a standard framework |

### 14.3 File Naming Conventions

**Schemas**: Name after the response type
- `worker_response.json` - for analysis task output
- `qa_response.json` - for QA verification output
- `setup_response.json` - for T2 setup task output (if different)

**Templates**: Match schema names
- `worker_response.md` - renders `worker_response.json` fields
- `qa_response.md` - renders `qa_response.json` fields

**Instructions**: Name by task type or tier
- `worker.md` or `execute_task.md` - T3 analysis instructions
- `qa.md` or `verify_task.md` - QA verification instructions
- `setup.md` - T2 setup instructions (if needed)

### 14.4 The procedure.md File

The main playbook document should include:

```markdown
# [Playbook Name] – Maestro Playbook

## Overview
- What this playbook is for
- What framework/standard it covers
- Prerequisites and required documents

## Authoritative Sources
- List the official source documents
- Note where to obtain them

## Title Format Convention
- How list item titles should be formatted
- Example: "Section number and name (e.g., '9.1 Access Control Policy')"

## List Structure
- What lists this playbook provides
- How to copy them to projects
- Item count and coverage notes

## Task Set Organization
- Recommended task set paths
- What each task set contains

## Phase-by-Phase Guidance
[Detailed guidance for each Maestro phase]

## Changelog
- v1.0 - Initial release
- v1.1 - Updated control titles to match standard exactly
```

### 14.5 Lists in Playbooks

When including reusable lists:

1. **Create with `list_create`**:
```
list_create(
  list="controls",
  source="playbook",
  playbook="security-audit",
  name="Security Audit Controls",
  description="Comprehensive list of framework requirements..."
)
```

2. **Document the expected count** in procedure.md
3. **Note the title format** used
4. **Explain the relationship** between list items and source controls

### 14.6 Example: Minimal Playbook

A minimal but complete playbook:

```
minimal-playbook/
├── procedure.md
├── schemas/
│   ├── worker_response.json
│   └── qa_response.json
├── templates/
│   ├── worker_response.md
│   └── qa_response.md
└── instructions/
    ├── worker.md
    └── qa.md
```

**6 files minimum** for a functional playbook with QA support.

---

## 15. Taking Charge as the AI Assistant

Users often don't understand the technical details of playbook creation. **You must take charge** of the process while keeping the user informed and getting their input on domain-specific decisions.

### 15.1 Your Role

You are the **technical expert** on Maestro and playbook structure. The user is the **domain expert** on their specific work (the standard, the process, the requirements).

**You decide**:
- Directory structure and file organization
- JSON schema field names and types
- Template formatting
- Task set organization
- Technical implementation details

**User decides**:
- Which standard/framework to use
- What the authoritative source documents are
- Title format preferences
- Scope (what's in/out)
- Report format requirements
- Domain-specific terminology

### 15.2 Don't Wait for Permission

When creating a playbook, don't ask permission for every step. Instead:

**Bad** (passive):
```
"Would you like me to create a schemas directory?"
"Should I add a JSON schema for worker responses?"
"Do you want me to create an instructions file?"
```

**Good** (proactive):
```
"I'm creating the playbook with the standard structure:
- schemas/ for JSON validation
- templates/ for report generation
- instructions/ for worker guidance

I'll need your input on the title format for list items.
Do you prefer section numbers with names (e.g., '4.1 Understanding the organization')
or just names (e.g., 'Understanding the organization')?"
```

### 15.3 Inform, Then Act

Tell the user what you're about to do, then do it:

```
"I'm going to:
1. Create the security-audit playbook
2. Set up the standard directory structure
3. Create JSON schemas based on typical audit workflows
4. Add the 137 checklist items from your CSV

The items will use titles in the format '5.1 Policies for information security'
matching the exact standard terminology.

Creating now..."
```

### 15.4 Ask the Right Questions

Focus questions on things only the user can answer:

**Essential questions**:
- "What is the authoritative source document for this standard?"
- "How do you want item titles formatted?"
- "Are any sections out of scope for this playbook?"
- "What verdict categories make sense for this domain?" (Pass/Fail vs Compliant/Non-compliant, etc.)

**Don't ask**:
- "Should I use JSON schemas?" (Yes, always)
- "Where should I put the templates?" (Standard location)
- "What should I name the instructions file?" (Follow conventions)

### 15.5 Handle Ambiguity

When something is unclear:

1. **State your assumption** clearly
2. **Proceed with a reasonable default**
3. **Note that it can be changed**

```
"I don't see a preference stated for verdict categories.
I'll use 'Conformant', 'Non-conformant', and 'Partial' which are standard for compliance audits.
Let me know if you'd prefer different terminology."
```

### 15.6 Provide Progress Updates

For longer operations, keep the user informed:

```
"Creating list items from the checklist CSV...
- Processed 50 of 137 items
- Processed 100 of 137 items
- Complete. 137 items created.

Verifying titles match standard terminology..."
```

### 15.7 Summarize What Was Done

After completing work, provide a clear summary:

```
"Playbook 'security-audit' is ready. Created:

Files:
- procedure.md (main documentation)
- schemas/worker_response.json
- schemas/qa_response.json
- templates/worker_response.md
- templates/qa_response.md
- instructions/worker.md
- instructions/qa.md

Lists:
- controls: 137 items covering all framework requirements
  - Management clauses (4.1-10.2): 25 items
  - Control objectives (5.1-8.34): 112 items

Title format: Section number + exact standard name
Example: '5.1 Policies for information security'

Ready to use with: list_copy(from_list='controls', from_source='playbook',
from_playbook='security-audit', to_list='controls', to_project='<your-project>')
"
```

---

## 16. Using Maestro Tasks for Playbook Creation

When creating a playbook involves processing many items (extracting hundreds of requirements, creating large lists, updating many entries), consider using Maestro's own task system to help.

### 16.1 When to Use Tasks for Playbook Work

**Use Maestro tasks when**:
- Extracting 50+ items from source documents
- Processing multiple source documents into lists
- Bulk updating list items (titles, content, metadata)
- Complex transformations requiring careful review

**Handle directly when**:
- Creating playbook structure (directories, files)
- Writing schemas and templates
- Small lists (< 50 items)
- Simple, straightforward operations

### 16.2 Example: Extracting Requirements

To extract requirements from a large document:

1. **Create a temporary project** for playbook creation work:
```
project_create(
  name="playbook-creation-audit",
  title="Audit Playbook Creation",
  description="Temporary project for creating security-audit playbook"
)
```

2. **Create a task set for extraction work**:
```
taskset_create(
  project="playbook-creation-audit",
  path="extraction",
  title="Extract Controls from Standard"
)
```

3. **Create tasks for each section/chapter**:
```
task_create(
  project="playbook-creation-audit",
  path="extraction",
  title="Extract Clause 4 requirements",
  prompt="Read the standard document Clause 4 and extract all requirements.
  For each requirement, provide:
  - section: The clause number (e.g., '4.1')
  - title: Exact clause title from the standard
  - content: The requirement text and assessment guidance

  Output as JSON array.",
  llm_model_id="claude-sonnet"
)
```

4. **Run the tasks** and collect results
5. **Use results to populate the playbook list**

### 16.3 Example: Bulk Title Updates

When list items need title corrections (e.g., matching exact standard terminology):

1. **Export current titles** for review:
```
list_get_summary(list="controls", source="playbook", playbook="my-playbook", limit=200)
```

2. **Create a correction task**:
```
task_create(
  project="playbook-creation",
  path="corrections",
  title="Verify and correct control titles",
  prompt="Compare each list item title against the authoritative standard document.
  For each item that needs correction, output:
  - id: The item ID
  - current_title: Current title
  - correct_title: Title exactly as it appears in the standard

  Reference document: [path to standard document]",
  llm_model_id="claude-opus"
)
```

3. **Apply corrections** from task results

### 16.4 Self-Referential Workflow

The general pattern:

```
┌─────────────────────────────────────────────────────────────┐
│                    Playbook Creation                         │
├─────────────────────────────────────────────────────────────┤
│  1. Create temporary project for playbook work              │
│  2. Create task set for extraction/transformation           │
│  3. Create tasks for each major piece of work               │
│  4. Run tasks (parallel if independent)                     │
│  5. Collect and validate results                            │
│  6. Use results to create/update playbook content           │
│  7. Archive or delete temporary project                     │
└─────────────────────────────────────────────────────────────┘
```

### 16.5 Benefits of Task-Based Playbook Creation

- **Parallelization**: Multiple extraction tasks can run simultaneously
- **Auditability**: Task results document what was extracted and how
- **Reliability**: If extraction fails partway, completed work is preserved
- **Quality**: QA can be enabled on extraction tasks to verify accuracy
- **Resumability**: Work can be paused and resumed across sessions

### 16.6 Cleanup

After playbook creation is complete:

```
# Verify playbook is correct
playbook_file_list(playbook="security-audit")
list_get_summary(list="controls", source="playbook", playbook="security-audit")

# Delete temporary project
project_delete(name="playbook-creation-audit")
```

---

## 17. Common Mistakes to Avoid

### 17.1 Content Mistakes

| Mistake | Problem | Solution |
|---------|---------|----------|
| Using paraphrased titles | Doesn't match source standard | Always use exact terminology from authoritative source |
| Missing section numbers | Hard to trace back to source | Include section/control numbers in titles |
| Incomplete content field | Workers lack context | Include everything worker needs in content field |
| Wrong item count | Missing coverage | Verify count matches source document |

### 17.2 Structure Mistakes

| Mistake | Problem | Solution |
|---------|---------|----------|
| Missing schemas | No validation, inconsistent output | Always create schemas first |
| Schema/template mismatch | Report generation fails | Use identical field names |
| No QA schema | Can't verify worker output | Always create QA schema |
| Instructions don't reference schema | Workers produce wrong format | Explicitly tell workers to output JSON matching schema |

### 17.3 Process Mistakes

| Mistake | Problem | Solution |
|---------|---------|----------|
| Not verifying items | Errors discovered late | Always verify count and sample items |
| Waiting for user on every decision | Slow, frustrating | Make technical decisions, ask only domain questions |
| Not documenting title format | Future confusion | Document format in procedure.md |
| No changelog | Can't track evolution | Add changelog section to procedure.md |

### 17.4 Technical Mistakes

| Mistake | Problem | Solution |
|---------|---------|----------|
| Wrong playbook path format | Task creation fails | Always use `playbook-name/path/file.md` format |
| Forgetting to copy playbook lists | List not found in project | Document `list_copy` requirement |
| Inconsistent naming | Confusion, errors | Follow naming conventions consistently |
| No authoritative source reference | Can't verify accuracy | Store source document path in playbook |

---

## Summary

Creating effective playbooks requires:

1. **Understanding the domain** through user consultation
2. **Following standard structure** for predictability
3. **Using authoritative sources** for accurate content
4. **Verifying everything** before considering complete
5. **Taking charge** of technical decisions
6. **Documenting conventions** for future use
7. **Iterating based on usage** to improve over time

The goal is a playbook that any future LLM session can pick up and use correctly, producing consistent, high-quality results for the user's domain.

---

## Phase-by-Phase Playbook Authoring Checklist

Use this checklist when building a new playbook to ensure nothing is missed.

### Phase 1: Initialization Files

- [ ] `files/readme.md` - Playbook overview and usage instructions
- [ ] `files/phases/phase_01_init.md` - Project initialization guidance
- [ ] Domain-specific concepts documented
- [ ] Required user inputs listed
- [ ] Prerequisites and dependencies stated

### Phase 2: Requirements/Scope Files

- [ ] `files/phases/phase_02_scope.md` - Scope definition guidance
- [ ] Questions to ask user about scope documented
- [ ] Full coverage requirements stated (no sampling)
- [ ] Constraints and exclusions documented

### Phase 3: Planning Files

- [ ] `files/phases/phase_03_planning.md` - Planning guidance
- [ ] Task set structure defined
- [ ] Path naming conventions documented
- [ ] Task types and their purposes listed

### Phase 4: List Building Files

- [ ] `files/phases/phase_04_lists.md` - List building guidance
- [ ] List naming conventions documented
- [ ] Item title format specified (ID-first vs title-first)
- [ ] Source document to list mapping defined
- [ ] Reusable lists in `lists/` directory if applicable
- [ ] Instructions for copying playbook lists to projects

### Phase 5: Task Execution Files

- [ ] `files/phases/phase_05_execution.md` - Execution guidance
- [ ] `files/prompts/` directory with assessment prompts
- [ ] `files/schemas/worker_response.json` - Response schema
- [ ] `files/templates/worker_report.md` - Report template
- [ ] Prompt references schema with examples
- [ ] Runner configuration guidance

### Phase 6: Quality Assurance Files

- [ ] `files/phases/phase_06_qa.md` - QA guidance
- [ ] `files/prompts/qa-review.md` - QA prompt
- [ ] `files/schemas/qa_response.json` - QA response schema
- [ ] `files/templates/qa_report.md` - QA report template
- [ ] QA criteria defined (when to pass/fail/escalate)

### Phase 7: Reporting Files

- [ ] `files/phases/phase_07_reporting.md` - Reporting guidance
- [ ] Report structure and sections defined
- [ ] `files/templates/disclaimer.md` - AI disclosure for reports
- [ ] Multi-template manifests if multiple report variants needed
- [ ] Report naming conventions documented

### Phase 8: Review Phase Files

- [ ] `files/phases/phase_08_review.md` - Review guidance
- [ ] Supervisor review workflow documented
- [ ] `supervisor_update` usage explained
- [ ] List item completion tracking documented
- [ ] Lessons learned capture process defined
- [ ] Control/item-specific guidance capture process defined
- [ ] Playbook update procedures for future improvements

### Cross-Cutting Verification

- [ ] All schemas are valid JSON Schema
- [ ] Field names consistent across schemas and templates
- [ ] Template variables match schema field names
- [ ] Required fields defined appropriately
- [ ] Enum values cover all valid options
- [ ] Path formats documented (`playbook-name/path/file.md`)
- [ ] All file references tested and valid

### Playbook Metadata

- [ ] Playbook name follows naming conventions
- [ ] Version or date in readme
- [ ] Author/maintainer noted
- [ ] Related playbooks or dependencies listed

---

## Example Playbooks

For a complete example of playbook structure including all phases, templates, and the review phase, see:

- `playbook-examples/it-audit-playbook.md` - IT audit playbook with review phase

Use `reference_get(path="playbook-examples/it-audit-playbook.md")` to read the example.
