# Phase 01 – Project Initiation

## Purpose

Help you (the LLM) either:

- Discover and resume an existing project, or
- Create a new project with proper metadata and playbook guidance,

so that all subsequent work has a clear, persistent home and follows established procedures when available.

## When to Use

Use this phase when:

- The user first asks you to "use Maestro" or "conduct an analysis" with Maestro.
- The user asks to resume a named project.
- You need to determine whether you are working with an existing project or creating a new one.

## High-Level Goals

- Identify whether a relevant project already exists.
- Check for existing playbooks that may apply to this work.
- Consult with the user about playbook selection.
- If no applicable playbook exists, recommend creating one and work with the user to build it if they agree.
- If resuming:
  - Load project metadata, plan, and task state.
  - Review any playbook associated with the project.
- If new:
  - Create a new project name and metadata.
  - Select and apply relevant playbook(s) if available, or create a new playbook if needed.
- End this phase with:
  - A chosen `project` name,
  - Basic project metadata created,
  - Playbook guidance identified or created (if applicable).

## Step-by-Step Checklist

1. **Check for existing projects**
   - Use `project_list` to list known projects.
   - If any exist, summarize them to the user and ask:
     - "Would you like to resume one of these projects or start a new project?"

2. **Check for existing playbooks**
   - Use `playbook_list` to list available playbooks.
   - If playbooks exist, review their names and descriptions.
   - Ask the user: "I found the following playbooks: [list]. Do any of these apply to the work you want to do?"
   - If the user indicates a playbook applies:
     - Read the playbook's main document (e.g., `playbook_file_get(playbook="<name>", path="procedure.md")`).
     - Follow the playbook's guidance for project structure, naming, and initial setup.
     - Reference the playbook throughout subsequent phases.
   - If no playbook applies or none exist:
     - **Recommend creating a playbook**: "I don't see an existing playbook for this type of work. Would you like me to help you create one?"
     - If the user agrees to create a playbook:
       - Read `reference/authoring-playbooks.md` for guidance on playbook creation.
       - Work with the user to design the playbook.
       - Create the playbook using `playbook_create`.
       - Create initial playbook files using `playbook_file_put`.
       - Once the playbook is created, use it for this project.
     - If the user declines:
       - Proceed with general Maestro phases.

3. **If the user chooses to resume**
   - Identify the chosen project name.
   - Load project metadata with `project_get(name="<project>")`.
   - Check the project description/metadata for any playbook references.
   - If a playbook was used, reload it: `playbook_file_get(playbook="<name>", path="procedure.md")`.
   - Load the project's task sets with `taskset_list(project="<project>")`.
   - Review recent activity with `project_log_get(project="<project>")`.
   - Load any plan documents from project files:
     - `project_file_get(project="<project>", path="plan.md")` (if it exists)
   - Summarize the current state to the user and then transition to the next relevant phase.

4. **If the user chooses to start a new project**
   - If a playbook was selected in step 2:
     - Follow the playbook's guidance for project naming conventions.
     - Follow the playbook's guidance for initial project metadata.
     - Note the playbook being used in the project description.
   - Ask the user for:
     - A short, descriptive project name (following playbook conventions if applicable).
     - A concise statement of the goal.
   - **Identify the disclaimer template**:
     - If using a playbook, check if it has a disclaimer template (typically `templates/disclaimer.md`).
     - If no playbook or no disclaimer exists, inform the user:
       > "Every Maestro project requires a disclaimer template for reports. I recommend including an AI disclosure statement. Would you like me to create one, or would you prefer to use 'none'?"
     - AI disclosure is strongly recommended for professional work.
   - Create the project with `project_create`:
     - `name`: the project name
     - `title`: human-readable title
     - `description`: goal/objective + " [Using playbook: <playbook-name>]" if applicable
     - `disclaimer_template`: **REQUIRED** - either `"playbook-name/templates/disclaimer.md"` or `"none"`
   - If using a playbook, create any initial files recommended by the playbook.
   - Optionally write additional notes to project files using `project_file_put`.

5. **Gather evidence and input files** (for audits, evaluations, assessments)
   - Ask the user about evidence or input documents:
     - "What evidence or input documents do you have for this project?"
     - "Where are these files located on your filesystem?"
   - **Import external files** using `file_import`:
     - Import files from anywhere on the filesystem into the project
     - Files are imported to `files/imported/` directory
     - Use `recursive=true` for directories
     - Use `convert=true` to automatically convert PDF, DOCX, XLSX to Markdown
     - Symlinks that point outside the imported folder are automatically removed for security
     ```
     file_import(
       project="my-project",
       source="/path/to/evidence/folder",
       recursive=true,
       convert=true
     )
     ```
   - **Extract zip archives** using `project_file_extract`:
     - Evidence packages often come as zip files (e.g., Drata exports)
     - Extract in place: `archive.zip` → `archive/` folder
     - Use `convert=true` to convert extracted files to Markdown
     ```
     project_file_extract(
       project="my-project",
       path="imported/evidence.zip",
       convert=true
     )
     ```
   - **Optionally delete archives** after extraction:
     - **Always ask the user first**: "Would you like me to delete the original zip files now that they've been extracted?"
     - Only delete if the user confirms
     ```
     project_file_delete(
       project="my-project",
       path="imported/evidence.zip"
     )
     ```
   - **Convert documents** using `project_file_convert`:
     - Convert PDF, DOCX, XLSX files to Markdown for easier processing
     - Use `recursive=true` for directories
     - **Note**: Conversion is optimized for LLM consumption. Due to Markdown limitations, complex layouts and formatting may not be fully preserved.
     ```
     project_file_convert(
       project="my-project",
       path="imported/documents",
       recursive=true
     )
     ```
   - Record the evidence location(s) in the project:
     - Create `evidence-manifest.md` or similar documenting what evidence exists and where
     - Include paths that will be referenced in task prompts
   - **Important**: You will verify evidence accessibility in Phase 02.

6. **Confirm setup**
   - Tell the user:
     - The project name,
     - That metadata has been created,
     - Whether a playbook is being used (and which one),
     - **If files are needed**: Inform the user that a `files/` directory exists at `<base_dir>/projects/<project-name>/files/`, and ask them to place any documents there.
     - That you will now move to the requirements phase.

## Typical Tools Used

- `project_list` – discover existing projects
- `playbook_list` – discover available playbooks
- `playbook_file_get` – read playbook documentation
- `playbook_create` – create a new playbook (if needed)
- `playbook_file_put` – write playbook files (if creating a playbook)
- `reference_get` – read authoring-playbooks.md guide (if creating a playbook)
- `project_get` – load project metadata
- `project_create` – create a new project
- `project_log_get` – review project history
- `taskset_list` – load project task sets
- `project_file_get` – load project plan/notes
- `project_file_put` – create additional project files
- `file_import` – import external files into the project
- `project_file_extract` – extract zip archives within the project
- `project_file_convert` – convert PDF, DOCX, XLSX to Markdown

## Expected Outputs

For a new project:

- Project created via `project_create` with:
  - Project name,
  - User-provided title and description,
  - Playbook reference in description (if applicable),
  - Status set to `pending` (default),
  - **`disclaimer_template`** set (playbook path or `"none"`).
- Playbook selected and loaded (if existing playbook was chosen).
- **OR** new playbook created (if user agreed to create one).
- Understanding of which playbook guidance to follow in subsequent phases.
- User informed about where to place files (if needed for the project).
- Evidence locations documented (for audits/evaluations):
  - `evidence-manifest.md` or similar in project files
  - Clear record of where evidence will be found

For a resumed project:

- A clear understanding of the project's current state, ready to continue in the next phase.
- Playbook reloaded (if the project was using one).
- Awareness of playbook guidance for continuing work.

## Key Decision Points

This phase requires user consultation on:

1. **Resume vs. new project**: Which existing project to resume, or whether to start fresh.
2. **Playbook selection**: Whether an existing playbook applies to their work.
3. **Playbook creation**: If no playbook exists, whether to create one now.
4. **Evidence and inputs**: What documents/files are needed and where they are located.

Always present options clearly and respect the user's choice.
