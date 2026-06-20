# Phase 04 – Build Lists

## Purpose

Enumerate **all items** that must be processed (e.g., requirements, controls, sections) and store them as structured lists in the project.

These lists act as the source of truth for "what must be checked" and will drive per-item task creation in Phase 05.

## When to Use

Use this phase when:

- The project involves checking a finite set of items, and
- The user has indicated that **every item** must be addressed (no sampling), or
- You need explicit lists of items to organize the work.

## High-Level Goals

- Identify or create lists of items to be processed.
- Ensure lists are complete and items are uniquely identified.
- Store lists in the project (not in playbooks) for consistency and auditability.

## Step-by-Step Checklist

### 1. Check for Existing Playbook Lists

Before creating lists from scratch, check if a relevant playbook contains a reusable list:

```
list_list(source="playbook", playbook="<playbook-name>")
```

**If a playbook list exists**, copy it to your project:

```
list_copy(
  from_list="controls",
  from_source="playbook",
  from_playbook="compliance-framework",
  to_list="controls",
  to_project="my-project"
)
```

**Important**: Always copy playbook lists to your project rather than referencing them directly. This ensures:
- **Consistency**: The list remains stable even if the playbook is updated mid-project
- **Isolation**: Project-specific modifications don't affect the shared playbook
- **Auditability**: The project contains a complete record of what was processed

### 2. Create Lists from Source Documents

If no suitable playbook list exists, extract items from source documents:

1. **Identify sources for item lists**
   - From the plan (`plan.md`) and requirements, determine:
     - Which documents contain the items
     - Whether there will be one list or multiple lists (e.g., requirements vs controls)

2. **Create the list**
   ```
   list_create(
     list="requirements",
     name="Project Requirements",
     description="Requirements extracted from specification document",
     project="my-project"
   )
   ```

3. **Extract and add items**
   - For each source document, extract items and add them:
   ```
   list_item_add(
     list="requirements",
     project="my-project",
     id="REQ-001",
     content="The system shall authenticate all users",
     source_doc="specification.pdf",
     section="3.1 Security"
   )
   ```
   - You can use `llm_dispatch` to delegate bulk extraction to a worker LLM if helpful.

### 3. Validate Completeness

Review the lists for:
- Missing items
- Duplicate IDs (the system prevents these, but check your extraction logic)
- Obvious parsing issues

Use `list_get_summary` to check item counts:
```
list_get_summary(list="requirements", project="my-project")
```

Use `list_item_search` to find specific items or verify content:
```
list_item_search(list="requirements", project="my-project", query="authentication")
```

### 4. Inform the User

Summarize:
- How many items were identified
- Which lists were created
- Whether lists were copied from playbooks or created fresh

Ask the user if they have any concerns about completeness.

Log the milestone:
```
project_log_append(
  project="my-project",
  message="Lists built: 45 requirements, 114 controls (copied from compliance playbook)"
)
```

### 5. Prepare for Per-Item Tasks

Note in the plan that:
- Each item in each list will have a corresponding task in Phase 05
- Use `list_create_tasks` to generate tasks from list items
- Tasks can reference list items via `list_refs` for context injection

## Typical Tools Used

- `list_list` – check for existing lists in playbooks
- `list_copy` – copy playbook lists to the project
- `list_create` – create new lists
- `list_item_add` – add items to lists
- `list_get_summary` – verify item counts
- `list_item_search` – search and validate items
- `llm_dispatch` – delegate item extraction to a worker LLM
- `project_log_append` – record milestones

## Expected Outputs

- One or more lists in the project:
  - `requirements` (stored as `requirements.json`)
  - `controls` (stored as `controls.json`)
  - etc.
- Each list should contain:
  - A complete set of items
  - Stable, unique IDs
  - Content and/or titles
  - Optional metadata (source_doc, section)
- Project log entry noting lists were built

## Best Practices

1. **Always copy playbook lists** – Never reference playbook lists directly for task creation. Copy them to ensure project isolation.

2. **Use meaningful IDs** – IDs like `REQ-001`, `CTRL-001` are easier to track than auto-generated ones.

3. **Include source references** – The `source_doc` and `section` fields help with traceability.

4. **Validate before proceeding** – It's easier to fix list issues now than after tasks are created.
