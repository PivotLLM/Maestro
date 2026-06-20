# Maestro MCP Server - Comprehensive Test Plan

This document provides a step-by-step test plan for validating Maestro's MCP tools. Each test includes explicit instructions and expected results to enable any LLM assistant to execute the tests and verify correct behavior.

## Prerequisites

- Maestro MCP server must be running and accessible
- The assistant must have access to Maestro MCP tools (prefixed with `mcp__Maestro__`)

## Test Execution Guidelines

1. Execute tests in order - later tests may depend on resources created in earlier tests
2. Verify each expected result before proceeding to the next test
3. If a test fails, note the failure and continue with remaining tests where possible
4. Clean up all test data at the end (Section 12)

---

## Section 1: Health Check

### Test 1.1: Verify System Health

**Steps:**
1. Call `mcp__Maestro__health` with no parameters

**Expected Results:**
- Response contains `"healthy": true`
- Response contains `"status": "healthy"`
- Response contains `"base_dir"` with a valid path
- Response contains `"config_path"` with a valid path
- Response contains `"enabled_llms"` with a number >= 0

---

## Section 2: Project Management

### Test 2.1: Create a Test Project

**Steps:**
1. Call `mcp__Maestro__project_create` with:
   - `name`: `"test-project-001"`
   - `title`: `"Test Project for Validation"`
   - `description`: `"A comprehensive test project for validating Maestro functionality"`
   - `status`: `"in_progress"`

**Expected Results:**
- Response contains `"name": "test-project-001"`
- Response contains `"title": "Test Project for Validation"`
- Response contains `"description": "A comprehensive test project for validating Maestro functionality"`
- Response contains `"status": "in_progress"`
- Response contains a `uuid` field with a valid UUID
- Response contains `created_at` and `updated_at` timestamps

### Test 2.2: Retrieve Project Details

**Steps:**
1. Call `mcp__Maestro__project_get` with:
   - `name`: `"test-project-001"`

**Expected Results:**
- Response contains all fields from Test 2.1

### Test 2.3: List Projects

**Steps:**
1. Call `mcp__Maestro__project_list` with no parameters

**Expected Results:**
- Response contains `"projects"` array
- The array includes an entry with `"name": "test-project-001"`

### Test 2.4: Update Project

**Steps:**
1. Call `mcp__Maestro__project_update` with:
   - `name`: `"test-project-001"`
   - `title`: `"Updated Test Project Title"`
   - `description`: `"Updated description for testing"`

**Expected Results:**
- Response contains `"title": "Updated Test Project Title"`
- Response contains `"description": "Updated description for testing"`
- `updated_at` timestamp is more recent than `created_at`

### Test 2.5: Append to Project Log

**Steps:**
1. Call `mcp__Maestro__project_log_append` with:
   - `project`: `"test-project-001"`
   - `message`: `"Test log entry: Project validation started"`

**Expected Results:**
- Response contains `"logged": true`
- Response contains `"project": "test-project-001"`

### Test 2.6: Retrieve Project Log

**Steps:**
1. Call `mcp__Maestro__project_log_get` with:
   - `project`: `"test-project-001"`

**Expected Results:**
- Response contains `"project": "test-project-001"`
- Response contains `"events"` array
- The events array includes entries containing "Project created" and "Test log entry: Project validation started"

---

## Section 3: List Management

### Test 3.1: Create a List in Project

**Steps:**
1. Call `mcp__Maestro__list_create` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements"`
   - `name`: `"Project Requirements"`
   - `description`: `"Requirements identified during analysis"`

**Expected Results:**
- Response contains `"created": true`
- Response contains `"list": "requirements"`
- Response contains `"name": "Project Requirements"`

### Test 3.2: Verify List Exists

**Steps:**
1. Call `mcp__Maestro__list_list` with:
   - `project`: `"test-project-001"`

**Expected Results:**
- Response contains `"lists"` array with at least one entry
- Entry contains `"list": "requirements"`
- Entry contains `"name": "Project Requirements"`
- Entry contains `"item_count": 0`

### Test 3.3: Add First Item to List

**Steps:**
1. Call `mcp__Maestro__list_item_add` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements"`
   - `id`: `"REQ-001"`
   - `content`: `"The system shall authenticate users via secure token-based authentication"`
   - `source_doc`: `"specs/auth.md"`
   - `section`: `"Authentication Requirements"`

**Expected Results:**
- Response contains `"added": true`
- Response contains `"list": "requirements"`
- Response contains `"id": "REQ-001"`

### Test 3.4: Add Second Item to List

**Steps:**
1. Call `mcp__Maestro__list_item_add` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements"`
   - `id`: `"REQ-002"`
   - `content`: `"The system shall encrypt all data at rest using AES-256 encryption"`
   - `source_doc`: `"specs/security.md"`
   - `section`: `"Data Protection"`

**Expected Results:**
- Response contains `"added": true`
- Response contains `"id": "REQ-002"`

### Test 3.5: Add Third Item to List

**Steps:**
1. Call `mcp__Maestro__list_item_add` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements"`
   - `id`: `"REQ-003"`
   - `content`: `"The system shall implement rate limiting on all API endpoints"`
   - `source_doc`: `"specs/security.md"`
   - `section`: `"API Security"`

**Expected Results:**
- Response contains `"added": true`
- Response contains `"id": "REQ-003"`

### Test 3.6: Add Fourth Item to List

**Steps:**
1. Call `mcp__Maestro__list_item_add` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements"`
   - `id`: `"REQ-004"`
   - `content`: `"The system shall log all authentication attempts for audit purposes"`
   - `source_doc`: `"specs/auth.md"`
   - `section`: `"Audit Requirements"`

**Expected Results:**
- Response contains `"added": true`
- Response contains `"id": "REQ-004"`

### Test 3.7: Attempt to Add Duplicate Item ID

**Steps:**
1. Call `mcp__Maestro__list_item_add` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements"`
   - `id`: `"REQ-001"`
   - `content`: `"This should fail because REQ-001 already exists"`

**Expected Results:**
- Response is an error
- Error message contains "already exists"

### Test 3.8: Get List Summary

**Steps:**
1. Call `mcp__Maestro__list_get_summary` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements"`

**Expected Results:**
- Response contains `"name": "Project Requirements"`
- Response contains `"item_count": 4`
- Response contains `"items"` array with 4 entries
- Each item has `id` and `content` fields
- Content may be truncated (ending with "...")

### Test 3.9: Get Full List

**Steps:**
1. Call `mcp__Maestro__list_get` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements"`

**Expected Results:**
- Response contains `"name": "Project Requirements"`
- Response contains `"items"` array with 4 entries
- Each item contains full `content` (not truncated)
- Items include `source_doc` and `section` fields where provided

### Test 3.10: Get Single Item

**Steps:**
1. Call `mcp__Maestro__list_item_get` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements"`
   - `id`: `"REQ-001"`

**Expected Results:**
- Response contains `"id": "REQ-001"`
- Response contains `"content": "The system shall authenticate users via secure token-based authentication"`
- Response contains `"source_doc": "specs/auth.md"`
- Response contains `"section": "Authentication Requirements"`

---

## Section 4: List Item Search

### Test 4.1: Search by Content Query

**Steps:**
1. Call `mcp__Maestro__list_item_search` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements"`
   - `query`: `"encrypt"`

**Expected Results:**
- Response contains `"total_count": 1`
- Response contains `"items"` array with one entry
- The entry has `"id": "REQ-002"`

### Test 4.2: Search by ID Query

**Steps:**
1. Call `mcp__Maestro__list_item_search` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements"`
   - `query`: `"REQ-003"`

**Expected Results:**
- Response contains `"total_count": 1`
- Response contains item with `"id": "REQ-003"`

### Test 4.3: Search by Source Document Filter

**Steps:**
1. Call `mcp__Maestro__list_item_search` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements"`
   - `source_doc`: `"specs/security.md"`

**Expected Results:**
- Response contains `"total_count": 2`
- Response contains items with IDs "REQ-002" and "REQ-003"

### Test 4.4: Search by Section Filter

**Steps:**
1. Call `mcp__Maestro__list_item_search` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements"`
   - `section`: `"Authentication Requirements"`

**Expected Results:**
- Response contains `"total_count": 1`
- Response contains item with `"id": "REQ-001"`

### Test 4.5: Search with No Results

**Steps:**
1. Call `mcp__Maestro__list_item_search` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements"`
   - `query`: `"xyznonexistent123"`

**Expected Results:**
- Response contains `"total_count": 0`
- Response contains `"items"` as empty array or with 0 entries

---

## Section 5: List Item Modifications

### Test 5.1: Update Item Content

**Steps:**
1. Call `mcp__Maestro__list_item_update` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements"`
   - `id`: `"REQ-001"`
   - `content`: `"The system shall authenticate users via OAuth 2.0 with PKCE flow"`

**Expected Results:**
- Response contains `"updated": true`
- Response contains `"id": "REQ-001"`

### Test 5.2: Verify Item Update

**Steps:**
1. Call `mcp__Maestro__list_item_get` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements"`
   - `id`: `"REQ-001"`

**Expected Results:**
- Response contains `"content": "The system shall authenticate users via OAuth 2.0 with PKCE flow"`
- Original `source_doc` and `section` are preserved

### Test 5.3: Rename Item ID

**Steps:**
1. Call `mcp__Maestro__list_item_rename` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements"`
   - `id`: `"REQ-004"`
   - `new_id`: `"REQ-004-AUDIT"`

**Expected Results:**
- Response contains `"renamed": true`

### Test 5.4: Verify Old ID No Longer Exists

**Steps:**
1. Call `mcp__Maestro__list_item_get` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements"`
   - `id`: `"REQ-004"`

**Expected Results:**
- Response is an error
- Error message contains "not found"

### Test 5.5: Verify New ID Exists

**Steps:**
1. Call `mcp__Maestro__list_item_get` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements"`
   - `id`: `"REQ-004-AUDIT"`

**Expected Results:**
- Response contains `"id": "REQ-004-AUDIT"`
- Response contains the original content about audit logging

### Test 5.6: Remove Item

**Steps:**
1. Call `mcp__Maestro__list_item_remove` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements"`
   - `id`: `"REQ-004-AUDIT"`

**Expected Results:**
- Response contains `"removed": true`

### Test 5.7: Verify Item Removal

**Steps:**
1. Call `mcp__Maestro__list_get_summary` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements"`

**Expected Results:**
- Response contains `"item_count": 3`
- Items array does not contain "REQ-004-AUDIT"

---

## Section 6: Task Set and Task Creation

### Test 6.1: Create a Task Set

**Steps:**
1. Call `mcp__Maestro__taskset_create` with:
   - `project`: `"test-project-001"`
   - `path`: `"analysis"`
   - `title`: `"Analysis Tasks"`
   - `description`: `"Tasks for analyzing requirements"`

**Expected Results:**
- Response contains `"path": "analysis"`
- Response contains `"title": "Analysis Tasks"`
- Response contains `"tasks": []` (empty array initially)

### Test 6.2: Create Tasks from List Items

**Steps:**
1. Call `mcp__Maestro__list_create_tasks` with:
   - `list`: `"requirements"`
   - `list_project`: `"test-project-001"`
   - `project`: `"test-project-001"`
   - `path`: `"analysis"`
   - `type`: `"analysis"`
   - `title_template`: `"Analyze {{id}}"`
   - `prompt`: `"Review and analyze the following requirement:"`

**Expected Results:**
- Response contains `"tasks_created": 3`
- Response contains `"list_name": "Project Requirements"`
- Response contains `"task_ids"` array with 3 task IDs

### Test 6.3: Verify Tasks Created

**Steps:**
1. Call `mcp__Maestro__task_list` with:
   - `project`: `"test-project-001"`
   - `path`: `"analysis"`

**Expected Results:**
- Response contains `"total": 3`
- Response contains `"tasks"` array with 3 entries
- Each task has `"type": "analysis"`
- Each task has work status `"waiting"`
- Task titles follow pattern "Analyze REQ-XXX"

### Test 6.4: Verify Task Prompt Contains Item Context

**Steps:**
1. Call `mcp__Maestro__task_get` with:
   - `project`: `"test-project-001"`
   - `path`: `"analysis"`
   - `id`: `0` (first task ID)

**Expected Results:**
- Response contains task with `"prompt"` field
- Prompt contains "Review and analyze the following requirement:"
- Prompt contains "=== LIST ITEM ==="
- Prompt contains the item's ID, content, source_doc, and section

---

## Section 7: Task Workflow

### Test 7.1: Create Manual Task

**Steps:**
1. Call `mcp__Maestro__task_create` with:
   - `project`: `"test-project-001"`
   - `path`: `"analysis"`
   - `title`: `"Manual Analysis Task"`
   - `type`: `"manual"`
   - `prompt`: `"Perform manual review of the system"`

**Expected Results:**
- Response contains task with `"title": "Manual Analysis Task"`
- Response contains `"type": "manual"`
- Response contains work with `"status": "waiting"`

### Test 7.2: Get Task Status

**Steps:**
1. Call `mcp__Maestro__task_status` with:
   - `project`: `"test-project-001"`
   - `path`: `"analysis"`

**Expected Results:**
- Response contains status counts
- Response contains `"waiting"` count >= 4

### Test 7.3: List Task Sets

**Steps:**
1. Call `mcp__Maestro__taskset_list` with:
   - `project`: `"test-project-001"`

**Expected Results:**
- Response contains `"task_sets"` array
- Array includes task set with `"path": "analysis"`

### Test 7.4: Update Task

**Steps:**
1. Call `mcp__Maestro__task_update` with:
   - `project`: `"test-project-001"`
   - `uuid`: (UUID from task created in Test 7.1)
   - `title`: `"Updated Manual Task"`

**Expected Results:**
- Response contains task with `"title": "Updated Manual Task"`

### Test 7.5: Delete Task

**Steps:**
1. Call `mcp__Maestro__task_delete` with:
   - `project`: `"test-project-001"`
   - `uuid`: (UUID from task created in Test 7.1)

**Expected Results:**
- Response contains `"deleted": true`

---

## Section 8: Playbook Management

### Test 8.1: Create a Playbook

**Steps:**
1. Call `mcp__Maestro__playbook_create` with:
   - `name`: `"test-playbook-001"`

**Expected Results:**
- Response contains `"created": true`
- Response contains `"playbook": "test-playbook-001"`

### Test 8.2: Create List in Playbook

**Steps:**
1. Call `mcp__Maestro__list_create` with:
   - `source`: `"playbook"`
   - `playbook`: `"test-playbook-001"`
   - `list`: `"checklist"`
   - `name`: `"Standard Checklist"`
   - `description`: `"Reusable checklist for common tasks"`

**Expected Results:**
- Response contains `"created": true`
- Response contains `"list": "checklist"`

### Test 8.3: Add Items to Playbook List

**Steps:**
1. Call `mcp__Maestro__list_item_add` with:
   - `source`: `"playbook"`
   - `playbook`: `"test-playbook-001"`
   - `list`: `"checklist"`
   - `id`: `"CHECK-001"`
   - `content`: `"Verify all unit tests pass"`

**Expected Results:**
- Response contains `"added": true`
- Response contains `"id": "CHECK-001"`

### Test 8.4: Verify Playbook List

**Steps:**
1. Call `mcp__Maestro__list_get` with:
   - `source`: `"playbook"`
   - `playbook`: `"test-playbook-001"`
   - `list`: `"checklist"`

**Expected Results:**
- Response contains `"name": "Standard Checklist"`
- Response contains `"items"` array with 1 entry
- Entry has `"id": "CHECK-001"`

### Test 8.5: List Playbooks

**Steps:**
1. Call `mcp__Maestro__playbook_list` with no parameters

**Expected Results:**
- Response contains `"playbooks"` array
- Array includes `"test-playbook-001"`

---

## Section 9: Reference Documentation (Read-Only)

### Test 9.1: List Reference Files

**Steps:**
1. Call `mcp__Maestro__reference_list` with no parameters

**Expected Results:**
- Response contains `"items"` array
- Array includes files like "readme.md"

### Test 9.2: Get Reference File

**Steps:**
1. Call `mcp__Maestro__reference_get` with:
   - `path`: `"readme.md"`

**Expected Results:**
- Response contains `"path": "readme.md"`
- Response contains `"content"` with the file contents
- Content includes text about "Maestro"

### Test 9.3: Attempt to Create List in Reference (Should Fail)

**Steps:**
1. Call `mcp__Maestro__list_create` with:
   - `source`: `"reference"`
   - `list`: `"test"`
   - `name`: `"Should Fail"`

**Expected Results:**
- Response is an error
- Error message contains "read-only" or similar indication that reference is immutable

---

## Section 10: Error Handling

### Test 10.1: Get Non-Existent Project

**Steps:**
1. Call `mcp__Maestro__project_get` with:
   - `name`: `"nonexistent-project-xyz"`

**Expected Results:**
- Response is an error
- Error message contains "not found"

### Test 10.2: Get Non-Existent List

**Steps:**
1. Call `mcp__Maestro__list_get` with:
   - `project`: `"test-project-001"`
   - `list`: `"nonexistent"`

**Expected Results:**
- Response is an error
- Error message contains "not found"

### Test 10.3: Invalid List Name (Path Traversal)

**Steps:**
1. Call `mcp__Maestro__list_create` with:
   - `project`: `"test-project-001"`
   - `list`: `"../invalid"`
   - `name`: `"Should Fail"`

**Expected Results:**
- Response is an error
- Error message contains "cannot contain path separators"

### Test 10.4: Create Duplicate Project

**Steps:**
1. Call `mcp__Maestro__project_create` with:
   - `name`: `"test-project-001"`
   - `title`: `"Duplicate"`

**Expected Results:**
- Response is an error
- Error message contains "already exists"

### Test 10.5: Get Non-Existent Task Set

**Steps:**
1. Call `mcp__Maestro__taskset_get` with:
   - `project`: `"test-project-001"`
   - `path`: `"nonexistent"`

**Expected Results:**
- Response is an error
- Error message contains "not found"

---

## Section 11: List Rename and Delete

### Test 11.1: Rename List

**Steps:**
1. Call `mcp__Maestro__list_rename` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements"`
   - `new_list`: `"requirements-v2"`

**Expected Results:**
- Response contains `"renamed": true`

### Test 11.2: Verify Old List Name Gone

**Steps:**
1. Call `mcp__Maestro__list_get` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements"`

**Expected Results:**
- Response is an error
- Error message contains "not found"

### Test 11.3: Verify New List Name Exists

**Steps:**
1. Call `mcp__Maestro__list_get` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements-v2"`

**Expected Results:**
- Response contains `"name": "Project Requirements"`
- Response contains all previously added items

### Test 11.4: Delete List

**Steps:**
1. Call `mcp__Maestro__list_delete` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements-v2"`

**Expected Results:**
- Response contains `"deleted": true`

### Test 11.5: Verify List Deleted

**Steps:**
1. Call `mcp__Maestro__list_get` with:
   - `project`: `"test-project-001"`
   - `list`: `"requirements-v2"`

**Expected Results:**
- Response is an error
- Error message contains "not found"

---

## Section 12: Cleanup

### Test 12.1: Delete Task Set

**Steps:**
1. Call `mcp__Maestro__taskset_delete` with:
   - `project`: `"test-project-001"`
   - `path`: `"analysis"`

**Expected Results:**
- Response contains `"deleted": true`

### Test 12.2: Delete Test Project

**Steps:**
1. Call `mcp__Maestro__project_delete` with:
   - `name`: `"test-project-001"`

**Expected Results:**
- Response contains `"deleted": true`
- Response contains `"project": "test-project-001"`

### Test 12.3: Delete Test Playbook

**Steps:**
1. Call `mcp__Maestro__playbook_delete` with:
   - `name`: `"test-playbook-001"`

**Expected Results:**
- Response contains `"deleted": true`
- Response contains `"playbook": "test-playbook-001"`

### Test 12.4: Verify Project Deleted

**Steps:**
1. Call `mcp__Maestro__project_get` with:
   - `name`: `"test-project-001"`

**Expected Results:**
- Response is an error
- Error message contains "not found"

### Test 12.5: Verify Playbook Deleted

**Steps:**
1. Call `mcp__Maestro__playbook_list` with no parameters

**Expected Results:**
- Response `"playbooks"` array does not contain `"test-playbook-001"`

---

## Test Results Summary Template

After executing all tests, fill in the following summary:

| Section | Tests Passed | Tests Failed | Notes |
|---------|--------------|--------------|-------|
| 1. Health Check | /1 | | |
| 2. Project Management | /6 | | |
| 3. List Management | /10 | | |
| 4. List Item Search | /5 | | |
| 5. List Item Modifications | /7 | | |
| 6. Task Set and Task Creation | /4 | | |
| 7. Task Workflow | /5 | | |
| 8. Playbook Management | /5 | | |
| 9. Reference Documentation | /3 | | |
| 10. Error Handling | /5 | | |
| 11. List Rename and Delete | /5 | | |
| 12. Cleanup | /5 | | |
| **TOTAL** | **/61** | | |

## Failure Investigation

If any tests fail, document:

1. **Test ID**: Which test failed
2. **Actual Result**: What happened
3. **Expected Result**: What should have happened
4. **Error Message**: Full error text if applicable
5. **Possible Cause**: Initial assessment of why it might have failed
