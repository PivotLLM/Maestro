# Maestro User Guide

**Version 0.1 DRAFT**

---

## Legal Notices

Copyright (c) 2026 Tenebris Technologies Inc. All rights reserved.

This software is proprietary and confidential. Unauthorized copying, distribution, or use is strictly prohibited.

This product incorporates open source software. A list of open source components and their applicable licenses can be found in `opensource.md`.

---

## About This Guide

This guide is intended for end users who wish to use Maestro for their projects. It covers installation, configuration, and day-to-day use.

For technical details including configuration schemas, API references, and tool specifications, please see `technical.md`.

---

## What is Maestro?

Maestro is an orchestration tool that helps you work with AI assistants (like Claude) on complex, multi-step projects. Instead of trying to accomplish everything in a single conversation, Maestro provides structure, tracking, and persistence so that large projects can be completed reliably.

Think of Maestro as giving your AI assistant the ability to act as a **project manager** rather than just a chatbot. With Maestro, the AI can:

- Create and follow a structured plan
- Break work into manageable tasks
- Track progress and maintain state across sessions
- Ensure every item is processed (no skipping or sampling)
- Verify quality through automated QA checks
- Generate comprehensive reports
- Save everything so work can be resumed later

**You don't need to be technical to use Maestro.** You describe what you want to accomplish, and the AI handles the details using Maestro's tools.

---

## When to Use Maestro

Maestro is designed for work that is:

- **Complex** – Multiple phases, many items to review, cross-references between documents
- **Thorough** – Every item must be checked; nothing can be skipped
- **Auditable** – You need to show what was done and how decisions were made
- **Repeatable** – You'll do similar work again and want a consistent process

**Good examples:**

- Security evaluations against a requirements document
- Compliance assessments against security frameworks
- Internal audits and audit readiness reviews
- Document reviews against a checklist or standard
- Any "check all 100 items" type of work

**When Maestro is overkill:**

- Quick questions or simple summaries
- One-off tasks that don't need tracking
- Casual experimentation

When in doubt, ask your AI assistant: *"Is this a good use case for Maestro, or should we just handle it as a normal chat?"*

---

## Key Concepts

Before getting started, it helps to understand a few terms:

### Playbooks

A **playbook** is a saved procedure and set of supporting resources for a specific type of work. For example, you might have a playbook for "Security Audit" or "Compliance Assessment."

Playbooks are reusable—once you create a good process, you can use it for similar projects in the future. They can, and usually should, be improved over time as you learn what works best.

### Projects

A **project** is one piece of work. For example, "Review Product X against Standard Y" would be a project.

Each project has its own workspace on disk containing:
- Input documents
- Task lists
- Results and reports
- A log of what was done

Projects persist across sessions. If you close your chat and come back later, your project is still there.

### Lists

A **list** is a structured collection of items that need to be processed. For example, a list might contain all the requirements from a specification document, or all the controls from a compliance framework.

Lists ensure nothing gets missed. The AI can create one task for each item in a list, guaranteeing complete coverage.

### Tasks and Task Sets

A **task** is a single unit of work—for example, "Evaluate requirement REQ-001 against the evidence."

Tasks are organized into **task sets** using paths like `analysis`, `analysis/security`, or `qa`. This keeps related work grouped together.

Tasks can include **quality assurance (QA)**, where a separate check verifies the work was done correctly.

### The Runner

The **runner** is Maestro's automated task executor. Instead of the AI doing tasks one by one in the conversation, the runner can process many tasks in parallel, storing results as it goes. This includes handling QA for tasks that include them.

### Worker Responses and Reports

When the runner executes a task, it sends instructions to a worker AI. The worker must return a **structured response** (in JSON format) that follows a predefined schema. This ensures:

- **Consistency**: All results have the same structure, making them easy to compare
- **Quality**: Responses that don't match the schema are automatically rejected and the worker is asked to try again
- **Automation**: Results can be combined into a report automatically

You don't need to worry about JSON or schemas—this happens behind the scenes. What matters is that:

1. **Workers are told exactly what to return** in their instructions
2. **Invalid responses are caught automatically** and corrected
3. **Reports are generated from structured data** so they're consistent and professional

Playbooks include the  **JSON schemas** that and **markdown templates** that define how worker results appear in the report. When you ask for a report, Maestro transforms each task result into readable text using these templates.

---

## Installation

### Step 1: Download Maestro

Obtain the Maestro executable for your operating system and place it somewhere accessible:

- **Mac/Linux:** `/usr/local/bin/maestro` (may require administrator privileges)
- **Windows:** `C:\Program Files\Maestro\maestro.exe` or add to your PATH

### Step 2: First Run

Run Maestro once from a terminal or command prompt:

```
maestro
```

This creates the default configuration:
- A `~/.maestro` directory (or `%USERPROFILE%\.maestro` on Windows)
- A `playbooks` folder for your reusable procedures
- A `projects` folder for your work
- A starter configuration file at `~/.maestro/config.json`

### Step 3: Verify Installation

You can verify the installation by running:

```
maestro --version
```

---

## Configuring Maestro

Before using Maestro with an AI assistant, you need to configure at least one LLM (Large Language Model) that will do the actual work. Maestro currently supports Claude Code and OpenAI's Codex, but any command capable of accepting a prompt on stdin or as an argument will work.

### Editing the Configuration

Open `~/.maestro/config.json` in any text editor. You'll see a structure like this:

```json
{
  "version": 1,
  "base_dir": "~/.maestro",
  "chroot": "~/.maestro/data",
  "playbooks_dir": "data/playbooks",
  "projects_dir": "data/projects",
  "reference_dirs": [
     {"path": "data/reference", "mount": "user"}
  ],
  "mark_non_destructive": false,
  "default_llm": "claude",
  "llms": [
    {
      "id": "claude",
      "display_name": "Claude-CLI",
      "type": "command",
      "stdin": true,
      "command": "/PATH/TO/claude",
      "args": [
        "-p"
      ],
      "description": "Claude Code",
      "enabled": false,
      "recovery": {
        "rate_limit_patterns": [
          "you've hit your limit",
          "quota exceeded",
          "429",
          "will rest at"
        ],
        "test_prompt": "Respond with only OK",
        "test_schedule_seconds": [
          30,
          60,
          120,
          300,
          900,
          1800,
          3600
        ],
        "abort_after_seconds": 43200
      }
    },
    {
      "id": "gpt",
      "display_name": "Codex-CLI",
      "type": "command",
      "stdin": true,
      "command": "/usr/local/bin/codex",
      "args": [
        "exec",
        "--skip-git-repo-check"
      ],
      "description": "OpenAI Codex featuring GPT5",
      "enabled": false,
      "recovery": {
        "rate_limit_patterns": [
          "rate limit",
          "quota exceeded",
          "429",
          "too many requests"
        ],
        "test_prompt": "Respond with only OK",
        "test_schedule_seconds": [
          20,
          60,
          120,
          300,
          900,
          1800,
          3600
        ],
        "abort_after_seconds": 43200
      }
    }
  ],
  "runner": {
    "max_concurrent": 10,
    "max_rounds": 5,
    "round_delay_seconds": 30,
    "limits": {
      "max_retries": 5,
      "max_worker": 2,
      "max_qa": 2
    },
    "retry_delay_seconds": 60,
    "rate_limit": {
      "max_requests": 25,
      "period_seconds": 60
    }
  },
  "logging": {
    "file": "maestro.log",
    "level": "INFO"
  }
}
```

### Setting Up LLMs

Maestro uses command-line LLM tools. You can configure any CLI tool that accepts a prompt and returns a response.

**Prompt delivery options:**

1. **Command-line argument:** Use `{{PROMPT}}` placeholder in the `args` array. The prompt will be substituted into that position.

2. **Standard input:** Set `"stdin": true` and the prompt will be piped to the command's stdin instead. This is preferable to the command line, especially for large prompts.

**Example with stdin:**
```json
{
  "id": "claude-code-stdin",
  "type": "command",
  "command": "claude",
  "args": ["-p"],
  "stdin": true,
  "description": "Claude Code with stdin",
  "enabled": false
}
```

### Enabling an LLM

LLMs are disabled by default for safety. To enable one, change `"enabled": false` to `"enabled": true`.

You need at least one enabled LLM for Maestro to function.

### Optional Settings

**Custom directories:** You can change where playbooks and projects are stored by setting `playbooks_dir` and `projects_dir` to absolute paths. This is useful for storing them on a shared drive or in a version-controlled repository.

**Reduced confirmation prompts:** Set `"mark_non_destructive": true` to signal to AI clients that Maestro only modifies its own directories. This may reduce how often you're asked to confirm operations.

**Security boundary:** Set `"chroot": "/path/to/directory"` to restrict all Maestro file operations to a specific directory. All other paths in the configuration must be within this directory.

---

## Configuring Your AI Client

Maestro works with AI assistants that support the MCP (Model Context Protocol). Below are instructions for common clients.

### Claude Code

Claude Code is Anthropic's command-line AI assistant. To add Maestro:

```bash
claude mcp add --transport stdio Maestro --scope user /usr/local/bin/maestro
```

Adjust the path to wherever you installed the Maestro executable.

**To avoid repeated permission prompts**, edit `~/.claude/settings.json` and add `"mcp__Maestro"` to the allow list:

```json
{
  "permissions": {
    "allow": [
      "mcp__Maestro"
    ]
  }
}
```

Note: There are two underscore characters between "mcp" and "Maestro".

After changing configuration files, restart Claude Code for changes to take effect.

### Claude Desktop

Claude Desktop is Anthropic's graphical AI assistant. To add Maestro:

1. Open **Settings** → **Developer**
2. Click **Edit Config** to open the configuration file
3. Add Maestro to the `mcpServers` section:

```json
{
  "mcpServers": {
    "Maestro": {
      "command": "/usr/local/bin/maestro",
      "args": [],
      "timeout": 600000
    }
  }
}
```

Adjust the path as needed. Restart Claude Desktop after saving.

Note: Claude Desktop currently requires manual approval of tools the first time they're used.

### OpenAI Codex

Codex is OpenAI's command-line assistant. To add Maestro, edit `~/.codex/config.toml`:

```toml
[mcp_servers.Maestro]
command = "/usr/local/bin/maestro"
args = []
```

---

## Getting Started with a Project

Once Maestro is configured and connected to your AI assistant, you're ready to start a project.

### Step 1: Tell the AI to Use Maestro

Start a conversation with your AI assistant and say something like:

> "We're going to use Maestro. Please use the Maestro reference tool to read readme.md and follow the instructions."

The AI will load Maestro's documentation and learn how to use its tools.

### Step 2: Describe Your Goal

Explain what you want to accomplish. Be specific about:

- What you're reviewing or evaluating
- What documents or materials are involved
- What the output should look like
- Any constraints or priorities

**Example:**

> "I need to evaluate our new product against the security requirements in this specification document. Every requirement must be checked, and I need a report showing the status of each one with supporting evidence."

### Step 3: Answer Questions and Approve the Plan

The AI will typically:

- Ask for a **project name**
- Ask clarifying questions about scope and priorities
- Propose a **plan** breaking the work into phases and tasks
- Show you where files will be stored

Review the plan. Ask for changes if needed. When you're satisfied, approve it.

### Step 4: Provide Your Materials

Upload or provide access to your input documents—requirements, specifications, evidence files, etc.

**Importing Evidence Files**

If you have evidence files on your computer (such as exports from compliance platforms like Drata), the AI can import them directly:

1. Tell the AI where the files are located on your filesystem
2. The AI will use Maestro's import tools to bring them into the project
3. Zip files can be extracted automatically
4. PDF, Word, and Excel files can be converted to Markdown for easier processing

**Conversion Note**: Document conversion is optimized for LLM analysis. Due to Markdown limitations, complex layouts, tables, and formatting may not be fully preserved. The converted files are intended to make content accessible to the AI, not for human distribution.

Common import scenarios:
- **Folder of documents**: The AI imports the entire folder, converting files as needed
- **Zip archive**: The AI extracts the archive and processes the contents
- **Mixed evidence**: Import, extract, and convert in one workflow

**Security Note**: Symbolic links that point outside the project folder are automatically removed for security. This prevents accidentally exposing files from elsewhere on your system.

The AI will organize these and extract lists of items to process.

### Step 5: Let Maestro Work

Once the plan is approved and materials are loaded:

- The AI creates task sets and tasks
- The runner executes tasks (possibly in parallel)
- Results are stored as each task completes
- The AI may pause to ask questions or report progress

You can step in at any time to adjust priorities, answer questions, or review interim results.

### Step 6: Review the Report

When processing is complete, the AI will generate a report showing:

- Overall summary and statistics
- Results for each task
- QA findings (if QA was enabled)
- Any issues that need attention

Everything is saved to disk, so you can:

- Review the report later
- Share it with others
- Re-run parts of the process if needed
- Use the project as a baseline for future work

---

## Working with Playbooks

If you'll be doing similar work again, consider creating a playbook.

### Using an Existing Playbook

Ask the AI to check for relevant playbooks:

> "Are there any playbooks that would help with this type of work?"

If one exists, the AI can use it to guide the project.

### Creating a New Playbook

After completing a project, you can save the process as a playbook:

> "This worked well. Can we save this process as a playbook for future use?"

The AI will work with you to document the process, decisions, and templates so they can be reused.

### Improving a Playbook

As you use a playbook for multiple projects, you'll learn what works and what doesn't. You can ask the AI to update the playbook with improvements:

> "Let's update the playbook to include the new checklist items we identified."

---

## Reviewing and Updating Findings

After AI-generated work is complete, you may want to review the results and make corrections. Maestro provides a structured review workflow.

### Viewing Results

Ask the AI to show you the findings:

> "Show me the results for the assessment tasks."

The AI will retrieve each finding, showing you:
- The task title and status
- The AI's assessment (observations, evidence, requests)
- QA verification results (if QA was enabled)

### Updating a Finding

If you need to correct or update a finding, tell the AI:

> "Update finding A.8.15. Change the status to Complete. The observation should be that Elastic Cloud provides log protection."

The AI will use your corrections to update the finding. When a finding is updated:

1. **Your changes become the official finding** - Your corrected response replaces the AI's
2. **QA is marked as N/A** - Previous QA verification is no longer relevant since you've changed the finding
3. **Audit trail is preserved** - The original AI response is kept in the history for reference
4. **Reports are regenerated** - New reports reflect your updates

### What Happens to QA

When you update a finding:
- The previous QA verification data is cleared (it evaluated the old response, not yours)
- Reports will show "QA: N/A" for updated findings
- This is intentional—your judgment as supervisor takes precedence over automated QA

### Regenerating Reports

After making updates, the AI should regenerate reports to reflect your changes. If reports look stale, ask:

> "Please regenerate the reports for this project."

This creates new report files with the current date and time.

### Report Disclaimers

When you create a project, you must specify a **disclaimer template**. This is a markdown file that appears at the beginning of every report generated for that project.

**What to include in a disclaimer:**

- **AI Disclosure**: We strongly recommend disclosing that AI tools were used to assist with the project for transparency and professional ethics.
- **Methodology Statement**: Brief explanation of how the evaluation was conducted
- **Scope Limitations**: What was and wasn't covered by the assessment
- **Date and Version**: When the work was done and what was reviewed
- **Terms of Use**: Any restrictions on how the report can be used

**Setting the disclaimer:**

When creating a project, the LLM includes the path of a the markdown document that contains it. 

If you are sure you don't wish one, the field can be set to `"none"`.

**Why disclaimers are mandatory:**

Reports produced by Maestro may be shared with clients, auditors, or stakeholders. The disclaimer ensures every report has appropriate context about methodology, limitations, and the use of AI assistance.

**Example disclaimer:**

```markdown
## Disclaimer

This assessment was conducted using Maestro, an AI-assisted analysis tool.
While AI technology was used to help structure and process the evaluation,
all findings were reviewed and validated by qualified personnel.

This report represents findings as of the assessment date and is based on
information available at that time. It should not be relied upon for
purposes beyond its intended scope.
```

---

## Resuming Work

If you close your session and come back later, your project is still there.

Start a new conversation and say:

> "Let's resume work on the [project name] project using Maestro."

The AI will load the project state and you can continue where you left off.

---

## Tips for Success

**Be specific about scope.** Before the AI starts creating tasks, make sure you've agreed on exactly what's in and out of scope.

**Review the plan.** Take time to review the proposed plan before approving. It's easier to make changes now than after work has started.

**Check progress periodically.** For long-running projects, ask for status updates to make sure things are on track.

**Use QA for important work.** Enable quality assurance for tasks where accuracy matters. It adds time but catches errors.

**Save good processes.** If you develop a good workflow, save it as a playbook so you don't have to reinvent it next time.

---

## Troubleshooting

### "No enabled LLMs found"

You need to enable at least one LLM in your configuration. Edit `~/.maestro/config.json` and set `"enabled": true` for the LLM you want to use.

### "API key not found" or authentication errors

Check that:
1. Your API key is correctly set (either as an environment variable or directly in the config)
2. If using an environment variable, restart your terminal after setting it
3. The API key is valid and has not expired

### AI assistant doesn't recognize Maestro

Make sure:
1. Maestro is properly configured in your AI client (see "Configuring Your AI Client")
2. You've restarted your AI client after adding Maestro
3. The path to the Maestro executable is correct

### Project not found when resuming

Project names are case-sensitive. Make sure you're using the exact name. You can ask the AI to list all projects to see what's available.

### Tasks are failing

Check the project log file (in your project directory) for error messages. Common causes include:
- LLM rate limiting (too many requests)
- Network connectivity issues
- Invalid task prompts

### What happens if my AI session ends while tasks are running?

**Maestro continues running until all tasks complete.** When the runner is executing tasks, Maestro will:

1. **Complete all queued tasks** – even if your AI session ends or times out
2. **Write the report** – results are saved to the project's reports directory
3. **Exit cleanly** – only after all work is finished

This is intentional behavior for reliability. In complex multi-tier workflows, the orchestrating AI may start a task runner and then end its session. Maestro ensures that work continues to completion and reports are generated.

**What this means for you:**
- Don't worry if your AI chat session ends during a long task run
- Results will be saved in the project's `reports/` directory
- You can check progress using `task_status` in a new session
- Log files in `log.txt` show what happened

**To stop a running task set early**, ask your AI to use the `runner_stop` command (if available) or wait for completion.

---

## Getting Help

If you encounter issues:

- Ask your AI assistant for help with Maestro-specific questions
- Check `technical.md` for detailed configuration and tool information
- Review the project log files in `~/.maestro/projects/[project-name]/log.txt`

---

## Summary

Maestro transforms your AI assistant from a simple chatbot into a capable project manager. By providing structure, persistence, and verification, it enables you to tackle complex work with confidence that nothing will be missed.

**The workflow is simple:**

1. Tell the AI to use Maestro
2. Describe your goal
3. Review and approve the plan
4. Provide your materials
5. Let Maestro work (with your oversight)
6. Review the results

You focus on decisions and judgment. Maestro and the AI handle the planning, tracking, and documentation.
