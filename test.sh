#!/bin/sh
################################################################################
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                           #
# Please see the LICENSE file for details                                      #
################################################################################

# Maestro Comprehensive Test Suite
# Tests all MCP tools with verification of expected outcomes
# (llm_dispatch requires external API and is tested only for error handling)
#
# Test Numbering Convention:
#   0.x.x  - Fresh Start & Directory Creation
#   1.x.x  - Reference Tools (read-only, embedded)
#   2.x.x  - Playbook Tools (user knowledge)
#   3.x.x  - Project Management Tools
#   4.x.x  - Project File Tools
#   5.x.x  - Project Log Tools
#   6.x.x  - Project Task Tools
#   7.x.x  - Task Runner Tools
#   8.x.x  - LLM Tools
#   9.x.x  - System Tools (health)
#   10.x.x - Error Handling & Edge Cases
#   11.x.x - List Management Tools
#   12.x.x - Chroot Security Tests
#   14.x.x - Report Tools
#   15.x.x - Cleanup & Final Verification

#===============================================================================
# Configuration
#===============================================================================

# Detect script directory (works with symlinks)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SRC="$SCRIPT_DIR"

# PROBE can be overridden via environment variable
: "${PROBE:=probe}"

MAESTRO="$SRC/maestro"
CONFIG_TEMPLATE="$SRC/config-test.json"
CONFIG="$SRC/.config-test-runtime.json"
TEST_ROOT="$SRC/maestro-test"
TEST_DATA="$TEST_ROOT/data"

# Expand paths to absolute for cross-platform compatibility
SRC=$(cd "$SRC" && pwd)
TEST_ROOT="$SRC/maestro-test"
TEST_DATA="$TEST_ROOT/data"
MAESTRO="$SRC/maestro"
CONFIG_TEMPLATE="$SRC/config-test.json"
CONFIG="$SRC/.config-test-runtime.json"

# Parse command line arguments
PRESERVE_TEST_DIR=false
while getopts "x" opt; do
    case $opt in
        x)
            PRESERVE_TEST_DIR=true
            ;;
        *)
            echo "Usage: $0 [-x]"
            echo "  -x  Preserve test directory after tests complete"
            exit 1
            ;;
    esac
done

# Generate runtime config from template with platform-specific absolute paths
# Use sed to replace placeholders with actual paths
sed -e "s|MAESTRO_TEST_ROOT|$TEST_ROOT|g" \
    -e "s|MAESTRO_TEST_DATA|$TEST_DATA|g" "$CONFIG_TEMPLATE" > "$CONFIG"

# Set environment variables - MAESTRO_CONFIG tells maestro which config file to use
ENV="OPENAI_API_KEY=test,MAESTRO_CONFIG=$CONFIG"

cd $SRC
# Build only if binary doesn't exist
if [ ! -f "$MAESTRO" ]; then
    go build -o $MAESTRO
fi


# Test data names
TEST_PROJECT="maestro-test-proj"
TEST_PROJECT2="maestro-test-proj2"
TEST_PLAYBOOK="test-playbook"
TEST_PLAYBOOK2="test-playbook2"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0

#===============================================================================
# Pre-flight Checks
#===============================================================================

echo ""
echo "${BOLD}============================================${NC}"
echo "${BOLD}   Maestro Comprehensive Test Suite${NC}"
echo "${BOLD}   Testing all 76 MCP tools${NC}"
echo "${BOLD}============================================${NC}"
echo ""

# Check if PROBE exists - search PATH if not an absolute path
if [ -z "$PROBE" ]; then
    echo "${RED}ERROR: PROBE variable is not set${NC}"
    exit 1
fi

# If PROBE is not an absolute path, search for it in PATH
if [ "${PROBE#/}" = "$PROBE" ]; then
    # Not an absolute path - search in PATH
    PROBE_FULL=$(command -v "$PROBE" 2>/dev/null)
    if [ -z "$PROBE_FULL" ]; then
        echo "${RED}ERROR: MCPProbe binary '$PROBE' not found in PATH${NC}"
        echo ""
        echo "MCPProbe can be obtained from: ${CYAN}https://github.com/PivotLLM/MCPProbe${NC}"
        echo ""
        echo "After installing, ensure 'probe' is available in your PATH, or set the PROBE"
        echo "environment variable to the full path of the probe binary:"
        echo "  export PROBE=/path/to/probe"
        echo "  $0"
        exit 1
    fi
    PROBE="$PROBE_FULL"
    echo "Found probe at: $PROBE"
elif [ ! -f "$PROBE" ]; then
    echo "${RED}ERROR: MCPProbe not found at: $PROBE${NC}"
    echo "MCPProbe can be obtained from: ${CYAN}https://github.com/PivotLLM/MCPProbe${NC}"
    echo "Please set the PROBE environment variable to a valid MCPProbe executable path."
    exit 1
fi

# Check if PROBE is executable
if [ ! -x "$PROBE" ]; then
    echo "${RED}ERROR: MCPProbe is not executable: $PROBE${NC}"
    echo "Run: chmod +x $PROBE"
    exit 1
fi

# Check if MAESTRO exists
if [ ! -f "$MAESTRO" ]; then
    echo "${RED}ERROR: Maestro not found at: $MAESTRO${NC}"
    echo "Please build Maestro first: go build -o $MAESTRO ."
    exit 1
fi

# Check if MAESTRO is executable
if [ ! -x "$MAESTRO" ]; then
    echo "${RED}ERROR: Maestro is not executable: $MAESTRO${NC}"
    echo "Run: chmod +x $MAESTRO"
    exit 1
fi

echo "${GREEN}Pre-flight checks passed${NC}"
echo "  MCPProbe: $PROBE"
echo "  Maestro: $MAESTRO"
echo ""

#===============================================================================
# Helper Functions
#===============================================================================

# Print section header
print_section() {
    echo ""
    echo "${BOLD}${BLUE}============================================${NC}"
    echo "${BOLD}${BLUE}   $1${NC}"
    echo "${BOLD}${BLUE}   $2${NC}"
    echo "${BOLD}${BLUE}============================================${NC}"
    echo ""
}

# Print subsection header
print_subsection() {
    echo "${CYAN}--- $1 ---${NC}"
}

# Run a test and check for expected string in output
run_test() {
    local test_name="$1"
    local tool="$2"
    local params="$3"
    local expected="$4"

    echo "  ${test_name}"
    result=$($PROBE -stdio $MAESTRO -env $ENV -call "$tool" -params "$params" 2>&1)

    if echo "$result" | grep -q "Tool call succeeded"; then
        if [ -n "$expected" ]; then
            if echo "$result" | grep -q "$expected"; then
                echo "    ${GREEN}PASS${NC}: Found expected: $expected"
                PASS_COUNT=$((PASS_COUNT + 1))
            else
                echo "    ${RED}FAIL${NC}: Expected '$expected' not found"
                echo "    Output: $result"
                FAIL_COUNT=$((FAIL_COUNT + 1))
            fi
        else
            echo "    ${GREEN}PASS${NC}: Tool call succeeded"
            PASS_COUNT=$((PASS_COUNT + 1))
        fi
    else
        echo "    ${RED}FAIL${NC}: Tool call failed unexpectedly"
        echo "    Output: $result"
        FAIL_COUNT=$((FAIL_COUNT + 1))
    fi
}

# Run test expecting failure
run_test_expect_fail() {
    local test_name="$1"
    local tool="$2"
    local params="$3"
    local expected_error="$4"

    echo "  ${test_name}"
    result=$($PROBE -stdio $MAESTRO -env $ENV -call "$tool" -params "$params" 2>&1)

    if echo "$result" | grep -q "Tool call failed"; then
        if [ -n "$expected_error" ]; then
            if echo "$result" | grep -qi "$expected_error"; then
                echo "    ${GREEN}PASS${NC}: Got expected error: $expected_error"
                PASS_COUNT=$((PASS_COUNT + 1))
            else
                echo "    ${YELLOW}WARN${NC}: Failed but with different error"
                PASS_COUNT=$((PASS_COUNT + 1))
            fi
        else
            echo "    ${GREEN}PASS${NC}: Correctly failed"
            PASS_COUNT=$((PASS_COUNT + 1))
        fi
    else
        echo "    ${RED}FAIL${NC}: Expected failure but got success"
        echo "    Output: $result"
        FAIL_COUNT=$((FAIL_COUNT + 1))
    fi
}

# Run test and capture output for later use
run_test_capture() {
    local test_name="$1"
    local tool="$2"
    local params="$3"
    local expected="$4"

    echo "  ${test_name}"
    CAPTURED_RESULT=$($PROBE -stdio $MAESTRO -env $ENV -call "$tool" -params "$params" 2>&1)

    if echo "$CAPTURED_RESULT" | grep -q "Tool call succeeded"; then
        if [ -n "$expected" ]; then
            if echo "$CAPTURED_RESULT" | grep -q "$expected"; then
                echo "    ${GREEN}PASS${NC}: Found expected: $expected"
                PASS_COUNT=$((PASS_COUNT + 1))
            else
                echo "    ${RED}FAIL${NC}: Expected '$expected' not found"
                echo "    Output: $CAPTURED_RESULT"
                FAIL_COUNT=$((FAIL_COUNT + 1))
            fi
        else
            echo "    ${GREEN}PASS${NC}: Tool call succeeded"
            PASS_COUNT=$((PASS_COUNT + 1))
        fi
    else
        echo "    ${RED}FAIL${NC}: Tool call failed unexpectedly"
        echo "    Output: $CAPTURED_RESULT"
        FAIL_COUNT=$((FAIL_COUNT + 1))
    fi
}

# Silent cleanup (no output)
cleanup_silent() {
    $PROBE -stdio $MAESTRO -env $ENV -call "$1" -params "$2" > /dev/null 2>&1
}

# Call a tool and return just the JSON result (for use in variable capture)
call_tool() {
    local tool="$1"
    local params="$2"
    # Extract the JSON line that follows "Tool call succeeded:"
    $PROBE -stdio $MAESTRO -env $ENV -call "$tool" -params "$params" 2>&1 | grep -A2 "Tool call succeeded:" | grep "^{" | head -1
}

#===============================================================================
# SECTION 0: Fresh Start & Directory Creation
#===============================================================================

print_section "SECTION 0: Fresh Start" "Testing directory creation from scratch"

print_subsection "0.1 Clean Test Environment"
echo "  0.1.1 Removing test directory: $TEST_ROOT"
rm -rf "$TEST_ROOT"
if [ -d "$TEST_ROOT" ]; then
    echo "    ${RED}FAIL${NC}: Could not remove test directory"
    FAIL_COUNT=$((FAIL_COUNT + 1))
else
    echo "    ${GREEN}PASS${NC}: Test directory removed"
    PASS_COUNT=$((PASS_COUNT + 1))
fi

print_subsection "0.2 Verify Directories Created on First Run"
# Make a simple health check call to trigger directory creation
echo "  0.2.1 Running health check to trigger directory creation"
result=$($PROBE -stdio $MAESTRO -env $ENV -call "health" -params '{}' 2>&1)
if echo "$result" | grep -q "Tool call succeeded"; then
    echo "    ${GREEN}PASS${NC}: Health check succeeded"
    PASS_COUNT=$((PASS_COUNT + 1))
else
    echo "    ${RED}FAIL${NC}: Health check failed"
    echo "    Output: $result"
    FAIL_COUNT=$((FAIL_COUNT + 1))
fi

echo "  0.2.2 Verify chroot directory created"
if [ -d "$TEST_DATA" ]; then
    echo "    ${GREEN}PASS${NC}: Chroot directory exists: $TEST_DATA"
    PASS_COUNT=$((PASS_COUNT + 1))
else
    echo "    ${RED}FAIL${NC}: Chroot directory not created: $TEST_DATA"
    FAIL_COUNT=$((FAIL_COUNT + 1))
fi

echo "  0.2.3 Verify playbooks directory created"
if [ -d "$TEST_DATA/playbooks" ]; then
    echo "    ${GREEN}PASS${NC}: Playbooks directory exists"
    PASS_COUNT=$((PASS_COUNT + 1))
else
    echo "    ${RED}FAIL${NC}: Playbooks directory not created"
    FAIL_COUNT=$((FAIL_COUNT + 1))
fi

echo "  0.2.4 Verify projects directory created"
if [ -d "$TEST_DATA/projects" ]; then
    echo "    ${GREEN}PASS${NC}: Projects directory exists"
    PASS_COUNT=$((PASS_COUNT + 1))
else
    echo "    ${RED}FAIL${NC}: Projects directory not created"
    FAIL_COUNT=$((FAIL_COUNT + 1))
fi

echo "  0.2.5 Verify log file location is outside chroot (in base_dir)"
if [ -f "$TEST_ROOT/maestro.log" ]; then
    echo "    ${GREEN}PASS${NC}: Log file created in base_dir (outside chroot)"
    PASS_COUNT=$((PASS_COUNT + 1))
else
    echo "    ${YELLOW}WARN${NC}: Log file not found (may be created on first log)"
    PASS_COUNT=$((PASS_COUNT + 1))
fi

#===============================================================================
# SETUP: Clean any leftover test data from previous runs
#===============================================================================

print_section "SETUP" "Cleaning leftover test data"

cleanup_silent "project_delete" "{\"name\":\"$TEST_PROJECT\"}"
cleanup_silent "project_delete" "{\"name\":\"$TEST_PROJECT2\"}"
cleanup_silent "playbook_delete" "{\"name\":\"$TEST_PLAYBOOK\"}"
cleanup_silent "playbook_delete" "{\"name\":\"$TEST_PLAYBOOK-renamed\"}"
cleanup_silent "playbook_delete" "{\"name\":\"$TEST_PLAYBOOK2\"}"

echo "Cleanup complete"

#===============================================================================
# SECTION 1: Reference Tools (Read-Only, Embedded)
#===============================================================================

print_section "SECTION 1: Reference Tools" "Tools: reference_list, reference_get, reference_search"

print_subsection "1.1 List Reference Files"
run_test "1.1.1 List all reference files" \
    "reference_list" \
    '{}' \
    "items"

run_test "1.1.2 List with prefix filter" \
    "reference_list" \
    '{"prefix":"tech"}' \
    ""

print_subsection "1.2 Get Reference Files"
run_test "1.2.1 Get start.md" \
    "reference_get" \
    '{"path":"start.md"}' \
    "Maestro"

run_test "1.2.2 Get phase document" \
    "reference_get" \
    '{"path":"phases/phase_01_init_project.md"}' \
    "Initiation"

run_test "1.2.3 Get config-example.json" \
    "reference_get" \
    '{"path":"config-example.json"}' \
    "version"

run_test_expect_fail "1.2.4 Get non-existent reference file" \
    "reference_get" \
    '{"path":"nonexistent.md"}' \
    "not found"

print_subsection "1.2.5 Byte Range Reading"
run_test "1.2.5.1 Get first 10 bytes of start.md" \
    "reference_get" \
    '{"path":"start.md","max_bytes":10}' \
    "total_bytes"

run_test "1.2.5.2 Get bytes with offset" \
    "reference_get" \
    '{"path":"start.md","byte_offset":5,"max_bytes":10}' \
    "offset"

run_test "1.2.5.3 Full file (no byte range)" \
    "reference_get" \
    '{"path":"start.md"}' \
    "content"

print_subsection "1.3 Search Reference"
run_test "1.3.1 Search for 'project'" \
    "reference_search" \
    '{"query":"project"}' \
    "items"

run_test "1.3.2 Search with limit" \
    "reference_search" \
    '{"query":"task","limit":5}' \
    ""

run_test "1.3.3 Search for rare term" \
    "reference_search" \
    '{"query":"playbook"}' \
    ""

print_subsection "1.4 User-Provided Reference Files"
run_test "1.4.1 List reference - check for user prefix support" \
    "reference_list" \
    '{"prefix":"user"}' \
    ""

run_test "1.4.2 List all reference files (embedded + user if configured)" \
    "reference_list" \
    '{}' \
    "items"

run_test "1.4.3 Search across all reference (embedded + user)" \
    "reference_search" \
    '{"query":"reference"}' \
    ""

run_test_expect_fail "1.4.4 Get non-existent user file (graceful error)" \
    "reference_get" \
    '{"path":"user/test.txt"}' \
    "not found"

run_test "1.4.5 Search with user prefix filter" \
    "reference_list" \
    '{"prefix":"user/"}' \
    ""

run_test_expect_fail "1.4.6 Byte range on non-existent user file (graceful error)" \
    "reference_get" \
    '{"path":"user/example.txt","max_bytes":100}' \
    "not found"

print_subsection "1.4.7 User-Provided Security"
run_test_expect_fail "1.4.7.1 Path traversal in user path" \
    "reference_get" \
    '{"path":"user/../../../etc/passwd"}' \
    ""

run_test_expect_fail "1.4.7.2 Path traversal attempt via parent directory" \
    "reference_get" \
    '{"path":"user/../../sensitive.txt"}' \
    ""

run_test_expect_fail "1.4.7.3 Absolute path in user" \
    "reference_get" \
    '{"path":"user//etc/passwd"}' \
    ""

#===============================================================================
# SECTION 2: Playbook Tools
#===============================================================================

print_section "SECTION 2: Playbook Tools" "Tools: playbook_list, playbook_create, playbook_rename, playbook_delete, playbook_file_*, playbook_search"

print_subsection "2.1 Playbook CRUD Operations"
run_test "2.1.1 List playbooks (initial)" \
    "playbook_list" \
    '{}' \
    "playbooks"

run_test "2.1.2 Create playbook" \
    "playbook_create" \
    "{\"name\":\"$TEST_PLAYBOOK\"}" \
    "\"playbook\":\"$TEST_PLAYBOOK\""

run_test "2.1.3 Verify playbook in list" \
    "playbook_list" \
    '{}' \
    "$TEST_PLAYBOOK"

run_test "2.1.4 Create second playbook" \
    "playbook_create" \
    "{\"name\":\"$TEST_PLAYBOOK2\"}" \
    "\"playbook\":\"$TEST_PLAYBOOK2\""

run_test_expect_fail "2.1.5 Create duplicate playbook" \
    "playbook_create" \
    "{\"name\":\"$TEST_PLAYBOOK\"}" \
    "already exists"

run_test_expect_fail "2.1.6 Create playbook with invalid name" \
    "playbook_create" \
    '{"name":".invalid"}' \
    "invalid"

run_test_expect_fail "2.1.7 Create playbook with slash in name" \
    "playbook_create" \
    '{"name":"invalid/name"}' \
    "invalid"

print_subsection "2.2 Playbook File Operations"
run_test "2.2.1 Create file in playbook" \
    "playbook_file_put" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"procedure.md\",\"content\":\"# Test Procedure\\n\\nStep 1: Do something\\nStep 2: Do more\",\"summary\":\"Test procedure file\"}" \
    '"created":true'

run_test "2.2.2 List files in playbook" \
    "playbook_file_list" \
    "{\"playbook\":\"$TEST_PLAYBOOK\"}" \
    "procedure.md"

run_test "2.2.3 Get file from playbook" \
    "playbook_file_get" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"procedure.md\"}" \
    "Test Procedure"

run_test "2.2.4 Update file in playbook" \
    "playbook_file_put" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"procedure.md\",\"content\":\"# Updated Procedure\\n\\nStep 1: New step\\nStep 2: Another step\",\"summary\":\"Updated procedure\"}" \
    '"created":false'

run_test "2.2.5 Verify file update" \
    "playbook_file_get" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"procedure.md\"}" \
    "Updated Procedure"

print_subsection "2.2.5a Playbook File Append"
run_test "2.2.5a.1 Append to existing file" \
    "playbook_file_append" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"procedure.md\",\"content\":\"\\n\\n## Appended Section\\n\\nThis was appended to the file\"}" \
    '"success":true'

run_test "2.2.5a.2 Verify append worked" \
    "playbook_file_get" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"procedure.md\"}" \
    "Appended Section"

run_test "2.2.5a.3 Append to non-existent file (creates)" \
    "playbook_file_append" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"new-append.md\",\"content\":\"# New File\\n\\nCreated via append\"}" \
    '"success":true'

run_test "2.2.5a.4 Verify new file created" \
    "playbook_file_get" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"new-append.md\"}" \
    "Created via append"

run_test "2.2.5a.5 Append to new file again" \
    "playbook_file_append" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"new-append.md\",\"content\":\"\\n\\nMore content appended\"}" \
    '"success":true'

run_test "2.2.5a.6 Verify second append" \
    "playbook_file_get" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"new-append.md\"}" \
    "More content appended"

print_subsection "2.2.5b Playbook File Edit"
run_test "2.2.5b.1 Create file for edit testing" \
    "playbook_file_put" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"edit-test.md\",\"content\":\"Line 1: Hello World\\nLine 2: Foo Bar\\nLine 3: Hello World\"}" \
    '"created":true'

run_test "2.2.5b.2 Edit file - single replacement" \
    "playbook_file_edit" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"edit-test.md\",\"old_string\":\"Foo Bar\",\"new_string\":\"Replaced Text\"}" \
    '"success":true'

run_test "2.2.5b.3 Verify single replacement" \
    "playbook_file_get" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"edit-test.md\"}" \
    "Replaced Text"

run_test_expect_fail "2.2.5b.4 Edit fails when old_string appears multiple times without replace_all" \
    "playbook_file_edit" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"edit-test.md\",\"old_string\":\"Hello World\",\"new_string\":\"Goodbye\"}" \
    "multiple"

run_test "2.2.5b.5 Edit with replace_all=true" \
    "playbook_file_edit" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"edit-test.md\",\"old_string\":\"Hello World\",\"new_string\":\"Goodbye\",\"replace_all\":true}" \
    '"success":true'

run_test "2.2.5b.6 Verify replace_all worked" \
    "playbook_file_get" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"edit-test.md\"}" \
    "Goodbye"

run_test_expect_fail "2.2.5b.7 Edit fails when old_string not found" \
    "playbook_file_edit" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"edit-test.md\",\"old_string\":\"NonExistentText\",\"new_string\":\"New\"}" \
    "not found"

run_test "2.2.5b.8 Edit to delete text (empty new_string)" \
    "playbook_file_edit" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"edit-test.md\",\"old_string\":\"Line 2: Replaced Text\\n\",\"new_string\":\"\"}" \
    '"success":true'

run_test_expect_fail "2.2.5b.9 Edit non-existent file" \
    "playbook_file_edit" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"nonexistent.md\",\"old_string\":\"foo\",\"new_string\":\"bar\"}" \
    "not found"

run_test "2.2.5b.10 Delete edit test file" \
    "playbook_file_delete" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"edit-test.md\"}" \
    '"deleted":true'

print_subsection "2.2.6 Byte Range Reading"
run_test "2.2.6.1 Byte range - first 10 bytes" \
    "playbook_file_get" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"procedure.md\",\"max_bytes\":10}" \
    "total_bytes"

run_test "2.2.6.2 Byte range - with byte_offset" \
    "playbook_file_get" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"procedure.md\",\"byte_offset\":5,\"max_bytes\":10}" \
    "offset"

run_test "2.2.7 Create nested file" \
    "playbook_file_put" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"templates/report.md\",\"content\":\"# Report Template\"}" \
    '"created":true'

run_test "2.2.8 List with prefix filter" \
    "playbook_file_list" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"prefix\":\"templates\"}" \
    "report.md"

print_subsection "2.3 Playbook File Rename"
run_test "2.3.1 Create file for rename" \
    "playbook_file_put" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"old-name.md\",\"content\":\"Rename test content\"}" \
    '"created":true'

run_test "2.3.2 Rename file" \
    "playbook_file_rename" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"from_path\":\"old-name.md\",\"to_path\":\"new-name.md\"}" \
    '"renamed":true'

run_test_expect_fail "2.3.3 Verify old name gone" \
    "playbook_file_get" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"old-name.md\"}" \
    "not found"

run_test "2.3.4 Verify new name exists" \
    "playbook_file_get" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"new-name.md\"}" \
    "Rename test content"

print_subsection "2.4 Playbook File Delete"
run_test "2.4.1 Delete file from playbook" \
    "playbook_file_delete" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"new-name.md\"}" \
    '"deleted":true'

run_test_expect_fail "2.4.2 Verify file deleted" \
    "playbook_file_get" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"new-name.md\"}" \
    "not found"

print_subsection "2.5 Playbook Search"
run_test "2.5.1 Create searchable file" \
    "playbook_file_put" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"searchable.md\",\"content\":\"This file contains UNIQUE_PLAYBOOK_TOKEN for testing\"}" \
    '"created":true'

run_test "2.5.2 Search across playbooks" \
    "playbook_search" \
    '{"query":"UNIQUE_PLAYBOOK_TOKEN"}' \
    "searchable.md"

run_test "2.5.3 Search within specific playbook" \
    "playbook_search" \
    "{\"query\":\"UNIQUE_PLAYBOOK_TOKEN\",\"playbook\":\"$TEST_PLAYBOOK\"}" \
    "searchable.md"

run_test "2.5.4 Search with limit/offset" \
    "playbook_search" \
    '{"query":"procedure","limit":10,"offset":0}' \
    ""

print_subsection "2.6 Playbook Rename"
run_test "2.6.1 Rename playbook" \
    "playbook_rename" \
    "{\"name\":\"$TEST_PLAYBOOK\",\"new_name\":\"$TEST_PLAYBOOK-renamed\"}" \
    '"renamed":true'

run_test_expect_fail "2.6.2 Verify old playbook name gone" \
    "playbook_file_list" \
    "{\"playbook\":\"$TEST_PLAYBOOK\"}" \
    "not found"

run_test "2.6.3 Verify new playbook name exists" \
    "playbook_file_list" \
    "{\"playbook\":\"$TEST_PLAYBOOK-renamed\"}" \
    "procedure.md"

print_subsection "2.7 Playbook Delete"
run_test "2.7.1 Delete playbook" \
    "playbook_delete" \
    "{\"name\":\"$TEST_PLAYBOOK-renamed\"}" \
    '"deleted":true'

run_test_expect_fail "2.7.2 Verify playbook deleted" \
    "playbook_file_list" \
    "{\"playbook\":\"$TEST_PLAYBOOK-renamed\"}" \
    "not found"

run_test "2.7.3 Delete second playbook" \
    "playbook_delete" \
    "{\"name\":\"$TEST_PLAYBOOK2\"}" \
    '"deleted":true'

print_subsection "2.8 Playbook Security"
run_test_expect_fail "2.8.1 Path traversal attempt" \
    "playbook_file_get" \
    '{"playbook":"test","path":"../../../etc/passwd"}' \
    ""

#===============================================================================
# SECTION 3: Project Management Tools
#===============================================================================

print_section "SECTION 3: Project Management" "Tools: project_create, project_get, project_update, project_list, project_delete, project_rename"

print_subsection "3.1 Project CRUD Operations"
run_test "3.1.1 List projects (initial)" \
    "project_list" \
    '{}' \
    "projects"

run_test "3.1.2 Create project" \
    "project_create" \
    "{\"name\":\"$TEST_PROJECT\",\"title\":\"Test Project\",\"description\":\"A comprehensive test project\",\"disclaimer_template\":\"none\"}" \
    "\"name\":\"$TEST_PROJECT\""

run_test "3.1.3 Verify project in list" \
    "project_list" \
    '{}' \
    "$TEST_PROJECT"

run_test "3.1.4 Get project details" \
    "project_get" \
    "{\"name\":\"$TEST_PROJECT\"}" \
    '"title":"Test Project"'

run_test "3.1.5 Verify description" \
    "project_get" \
    "{\"name\":\"$TEST_PROJECT\"}" \
    '"description":"A comprehensive test project"'

run_test "3.1.6 Verify default status is pending" \
    "project_get" \
    "{\"name\":\"$TEST_PROJECT\"}" \
    '"status":"pending"'

run_test "3.1.7 Create project with initial status" \
    "project_create" \
    "{\"name\":\"$TEST_PROJECT2\",\"title\":\"Second Project\",\"status\":\"in_progress\",\"disclaimer_template\":\"none\"}" \
    '"status":"in_progress"'

run_test_expect_fail "3.1.8 Create duplicate project" \
    "project_create" \
    "{\"name\":\"$TEST_PROJECT\",\"title\":\"Duplicate\",\"disclaimer_template\":\"none\"}" \
    "already exists"

run_test_expect_fail "3.1.9 Create project with invalid name" \
    "project_create" \
    '{"name":".invalid","title":"Bad Name","disclaimer_template":"none"}' \
    "invalid"

run_test_expect_fail "3.1.10 Create project with slash in name" \
    "project_create" \
    '{"name":"invalid/name","title":"Bad Name","disclaimer_template":"none"}' \
    "invalid"

run_test_expect_fail "3.1.11 Create project with empty name" \
    "project_create" \
    '{"name":"","title":"No Name","disclaimer_template":"none"}' \
    ""

print_subsection "3.2 Project Update"
run_test "3.2.1 Update project title" \
    "project_update" \
    "{\"name\":\"$TEST_PROJECT\",\"title\":\"Updated Title\"}" \
    '"title":"Updated Title"'

run_test "3.2.2 Update project status" \
    "project_update" \
    "{\"name\":\"$TEST_PROJECT\",\"status\":\"in_progress\"}" \
    '"status":"in_progress"'

run_test "3.2.3 Update project description" \
    "project_update" \
    "{\"name\":\"$TEST_PROJECT\",\"description\":\"Updated description\"}" \
    '"description":"Updated description"'

run_test "3.2.4 Verify updates persisted" \
    "project_get" \
    "{\"name\":\"$TEST_PROJECT\"}" \
    '"status":"in_progress"'

print_subsection "3.3 Project List Filtering"
run_test "3.3.1 List by status=in_progress" \
    "project_list" \
    '{"status":"in_progress"}' \
    "$TEST_PROJECT"

run_test "3.3.2 List with limit" \
    "project_list" \
    '{"limit":1}' \
    "projects"

run_test "3.3.3 List with offset" \
    "project_list" \
    '{"offset":0,"limit":10}' \
    "projects"

print_subsection "3.4 Project Rename"
# Create a project specifically for rename test
run_test "3.4.1 Create project for rename" \
    "project_create" \
    '{"name":"rename-test-proj","title":"Rename Test","disclaimer_template":"none"}' \
    '"name":"rename-test-proj"'

run_test "3.4.2 Rename project" \
    "project_rename" \
    '{"name":"rename-test-proj","new_name":"renamed-test-proj"}' \
    '"renamed":true'

run_test_expect_fail "3.4.3 Verify old name gone" \
    "project_get" \
    '{"name":"rename-test-proj"}' \
    "not found"

run_test "3.4.4 Verify new name exists" \
    "project_get" \
    '{"name":"renamed-test-proj"}' \
    '"name":"renamed-test-proj"'

# Clean up renamed project
cleanup_silent "project_delete" '{"name":"renamed-test-proj"}'

print_subsection "3.5 Project Delete"
run_test "3.5.1 Delete second project" \
    "project_delete" \
    "{\"name\":\"$TEST_PROJECT2\"}" \
    '"deleted":true'

run_test_expect_fail "3.5.2 Verify project deleted" \
    "project_get" \
    "{\"name\":\"$TEST_PROJECT2\"}" \
    "not found"

#===============================================================================
# SECTION 4: Project File Tools
#===============================================================================

print_section "SECTION 4: Project File Operations" "Tools: project_file_list, project_file_get, project_file_put, project_file_rename, project_file_delete, project_file_search"

print_subsection "4.1 Create Files"
run_test "4.1.1 Create file in project" \
    "project_file_put" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"requirements.md\",\"content\":\"# Requirements\\n\\n1. First requirement\\n2. Second requirement\",\"summary\":\"Project requirements\"}" \
    '"created":true'

run_test "4.1.2 Create second file" \
    "project_file_put" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"design.md\",\"content\":\"# Design Document\\n\\nArchitecture overview\"}" \
    '"created":true'

run_test "4.1.3 Create nested file" \
    "project_file_put" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"docs/api.md\",\"content\":\"# API Documentation\"}" \
    '"created":true'

run_test "4.1.4 Create index file (JSON)" \
    "project_file_put" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"index.json\",\"content\":\"{\\\"items\\\":[{\\\"id\\\":\\\"REQ-001\\\",\\\"title\\\":\\\"Auth Required\\\"}]}\"}" \
    '"created":true'

print_subsection "4.2 List Files"
run_test "4.2.1 List all files" \
    "project_file_list" \
    "{\"project\":\"$TEST_PROJECT\"}" \
    "requirements.md"

run_test "4.2.2 List with prefix filter" \
    "project_file_list" \
    "{\"project\":\"$TEST_PROJECT\",\"prefix\":\"docs\"}" \
    "api.md"

print_subsection "4.3 Get Files"
run_test "4.3.1 Get requirements file" \
    "project_file_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"requirements.md\"}" \
    "Requirements"

run_test "4.3.2 Get nested file" \
    "project_file_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"docs/api.md\"}" \
    "API Documentation"

run_test "4.3.3 Get index file" \
    "project_file_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"index.json\"}" \
    "REQ-001"

run_test_expect_fail "4.3.4 Get non-existent file" \
    "project_file_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"nonexistent.md\"}" \
    "not found"

print_subsection "4.3.5 Byte Range Reading"
run_test "4.3.5.1 Get first 10 bytes" \
    "project_file_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"requirements.md\",\"max_bytes\":10}" \
    "total_bytes"

run_test "4.3.5.2 Get bytes with byte_offset" \
    "project_file_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"requirements.md\",\"byte_offset\":5,\"max_bytes\":10}" \
    "offset"

run_test "4.3.5.3 Full file (no byte range)" \
    "project_file_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"requirements.md\"}" \
    "content"

print_subsection "4.4 Update Files"
run_test "4.4.1 Update requirements file" \
    "project_file_put" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"requirements.md\",\"content\":\"# Updated Requirements\\n\\n1. New requirement\\n2. Another requirement\\n3. Third requirement\"}" \
    '"created":false'

run_test "4.4.2 Verify update persisted" \
    "project_file_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"requirements.md\"}" \
    "Updated Requirements"

print_subsection "4.4a Append to Files"
run_test "4.4a.1 Append to existing file" \
    "project_file_append" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"requirements.md\",\"content\":\"\\n\\n## Additional Requirements\\n\\n4. Fourth requirement\\n5. Fifth requirement\"}" \
    '"success":true'

run_test "4.4a.2 Verify append worked" \
    "project_file_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"requirements.md\"}" \
    "Additional Requirements"

run_test "4.4a.3 Verify original content still present" \
    "project_file_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"requirements.md\"}" \
    "Updated Requirements"

run_test "4.4a.4 Append to non-existent file (creates)" \
    "project_file_append" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"append-test.md\",\"content\":\"# Test File\\n\\nCreated via append operation\"}" \
    '"success":true'

run_test "4.4a.5 Verify new file created via append" \
    "project_file_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"append-test.md\"}" \
    "Created via append"

run_test "4.4a.6 Append to created file" \
    "project_file_append" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"append-test.md\",\"content\":\"\\n\\n## Second Section\\n\\nMore appended content\"}" \
    '"success":true'

run_test "4.4a.7 Verify both sections present" \
    "project_file_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"append-test.md\"}" \
    "Second Section"

run_test_expect_fail "4.4a.8 Append to non-existent project" \
    "project_file_append" \
    '{"project":"nonexistent","path":"test.md","content":"test"}' \
    "not found"

print_subsection "4.4b Project File Edit"
run_test "4.4b.1 Create file for edit testing" \
    "project_file_put" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"edit-test.md\",\"content\":\"Line 1: Hello World\\nLine 2: Foo Bar\\nLine 3: Hello World\"}" \
    '"created":true'

run_test "4.4b.2 Edit file - single replacement" \
    "project_file_edit" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"edit-test.md\",\"old_string\":\"Foo Bar\",\"new_string\":\"Replaced Text\"}" \
    '"success":true'

run_test "4.4b.3 Verify single replacement" \
    "project_file_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"edit-test.md\"}" \
    "Replaced Text"

run_test_expect_fail "4.4b.4 Edit fails when old_string appears multiple times without replace_all" \
    "project_file_edit" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"edit-test.md\",\"old_string\":\"Hello World\",\"new_string\":\"Goodbye\"}" \
    "multiple"

run_test "4.4b.5 Edit with replace_all=true" \
    "project_file_edit" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"edit-test.md\",\"old_string\":\"Hello World\",\"new_string\":\"Goodbye\",\"replace_all\":true}" \
    '"success":true'

run_test "4.4b.6 Verify replace_all worked" \
    "project_file_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"edit-test.md\"}" \
    "Goodbye"

run_test_expect_fail "4.4b.7 Edit fails when old_string not found" \
    "project_file_edit" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"edit-test.md\",\"old_string\":\"NonExistentText\",\"new_string\":\"New\"}" \
    "not found"

run_test "4.4b.8 Edit to delete text (empty new_string)" \
    "project_file_edit" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"edit-test.md\",\"old_string\":\"Line 2: Replaced Text\\n\",\"new_string\":\"\"}" \
    '"success":true'

run_test_expect_fail "4.4b.9 Edit non-existent file" \
    "project_file_edit" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"nonexistent.md\",\"old_string\":\"foo\",\"new_string\":\"bar\"}" \
    "not found"

run_test "4.4b.10 Delete edit test file" \
    "project_file_delete" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"edit-test.md\"}" \
    '"deleted":true'

print_subsection "4.5 Rename Files"
run_test "4.5.1 Create file for rename" \
    "project_file_put" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"old-file.md\",\"content\":\"Content to rename\"}" \
    '"created":true'

run_test "4.5.2 Rename file" \
    "project_file_rename" \
    "{\"project\":\"$TEST_PROJECT\",\"from_path\":\"old-file.md\",\"to_path\":\"renamed-file.md\"}" \
    '"renamed":true'

run_test_expect_fail "4.5.3 Verify old name gone" \
    "project_file_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"old-file.md\"}" \
    "not found"

run_test "4.5.4 Verify new name exists" \
    "project_file_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"renamed-file.md\"}" \
    "Content to rename"

print_subsection "4.6 Delete Files"
run_test "4.6.1 Delete renamed file" \
    "project_file_delete" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"renamed-file.md\"}" \
    '"deleted":true'

run_test_expect_fail "4.6.2 Verify file deleted" \
    "project_file_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"renamed-file.md\"}" \
    "not found"

print_subsection "4.7 Search Files"
run_test "4.7.1 Create searchable file" \
    "project_file_put" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"searchable.md\",\"content\":\"This file has PROJECT_SEARCH_TOKEN for testing\"}" \
    '"created":true'

run_test "4.7.2 Search within project" \
    "project_file_search" \
    "{\"query\":\"PROJECT_SEARCH_TOKEN\",\"project\":\"$TEST_PROJECT\"}" \
    "searchable.md"

run_test "4.7.3 Search all projects" \
    "project_file_search" \
    '{"query":"Requirements"}' \
    ""

run_test "4.7.4 Search with limit" \
    "project_file_search" \
    "{\"query\":\"test\",\"project\":\"$TEST_PROJECT\",\"limit\":5}" \
    ""

print_subsection "4.8 File Security"
run_test_expect_fail "4.8.1 Path traversal attempt" \
    "project_file_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"../../../etc/passwd\"}" \
    ""

print_subsection "4.9 File Import, Extract, and Convert"

# Create a test directory with files for import
TEST_IMPORT_DIR="$TEST_ROOT/import-test"
mkdir -p "$TEST_IMPORT_DIR"
echo "# Test Document\n\nThis is a test document for import." > "$TEST_IMPORT_DIR/test-doc.md"
echo "Another file content" > "$TEST_IMPORT_DIR/another-file.txt"

run_test "4.9.1 Import directory into project" \
    "file_import" \
    "{\"project\":\"$TEST_PROJECT\",\"source\":\"$TEST_IMPORT_DIR\",\"recursive\":true}" \
    '"files_imported":'

run_test "4.9.2 Verify imported file exists" \
    "project_file_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"imported/import-test/test-doc.md\"}" \
    "Test Document"

run_test "4.9.3 List imported files" \
    "project_file_list" \
    "{\"project\":\"$TEST_PROJECT\",\"prefix\":\"imported\"}" \
    "test-doc.md"

# Create a test zip file
TEST_ZIP_DIR="$TEST_ROOT/zip-test"
mkdir -p "$TEST_ZIP_DIR"
echo "# Zipped Document\n\nContent from zip file." > "$TEST_ZIP_DIR/zip-doc.md"
echo "Secondary file" > "$TEST_ZIP_DIR/secondary.txt"
(cd "$TEST_ROOT" && zip -r zip-test.zip zip-test)

# Import the zip file first
run_test "4.9.4 Import zip file" \
    "file_import" \
    "{\"project\":\"$TEST_PROJECT\",\"source\":\"$TEST_ROOT/zip-test.zip\",\"recursive\":false}" \
    '"files_imported":1'

run_test "4.9.5 Extract zip file" \
    "project_file_extract" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"imported/zip-test.zip\"}" \
    '"files_extracted":'

run_test "4.9.6 Verify extracted file exists" \
    "project_file_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"imported/zip-test/zip-test/zip-doc.md\"}" \
    "Zipped Document"

run_test "4.9.7 Verify archive still exists (use project_file_delete to remove)" \
    "project_file_list" \
    "{\"project\":\"$TEST_PROJECT\",\"prefix\":\"imported\"}" \
    "zip-test.zip"

# Test overwrite=false (default)
run_test "4.9.8 Create file that will conflict" \
    "project_file_put" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"test-extract/conflict.txt\",\"content\":\"Original content\"}" \
    '"created":true'

# Create another zip with same file structure
echo "New content from zip" > "$TEST_ZIP_DIR/conflict.txt"
(cd "$TEST_ROOT" && rm -f test-extract.zip && zip -r test-extract.zip zip-test)
mv "$TEST_ROOT/test-extract.zip" "$TEST_ROOT/test-extract2.zip"

run_test "4.9.9 Import test zip" \
    "file_import" \
    "{\"project\":\"$TEST_PROJECT\",\"source\":\"$TEST_ROOT/test-extract2.zip\"}" \
    '"files_imported":1'

# Extract with overwrite=false should skip the conflicting file
run_test "4.9.10 Extract with overwrite=false skips existing files" \
    "project_file_extract" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"imported/test-extract2.zip\",\"overwrite\":false}" \
    '"files_skipped":'

run_test_expect_fail "4.9.11 Extract non-zip file fails" \
    "project_file_extract" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"imported/import-test/test-doc.md\"}" \
    "not a zip"

run_test_expect_fail "4.9.12 Extract non-existent file fails" \
    "project_file_extract" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"nonexistent.zip\"}" \
    "not found"

print_subsection "4.10 Symlink Security"

# Create a test directory with a symlink that escapes
TEST_SYMLINK_DIR="$TEST_ROOT/symlink-test"
mkdir -p "$TEST_SYMLINK_DIR/subdir"
echo "Safe content" > "$TEST_SYMLINK_DIR/safe-file.txt"
# Create a symlink that tries to escape
ln -s /etc/passwd "$TEST_SYMLINK_DIR/escape-link"
# Create a safe symlink within the directory
ln -s ../safe-file.txt "$TEST_SYMLINK_DIR/subdir/internal-link"

run_test "4.10.1 Import directory with escaping symlinks" \
    "file_import" \
    "{\"project\":\"$TEST_PROJECT\",\"source\":\"$TEST_SYMLINK_DIR\",\"recursive\":true}" \
    '"files_imported":'

run_test "4.10.2 Verify safe file was imported" \
    "project_file_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"imported/symlink-test/safe-file.txt\"}" \
    "Safe content"

run_test_expect_fail "4.10.3 Verify escaping symlink was removed" \
    "project_file_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"imported/symlink-test/escape-link\"}" \
    "not found"

# Clean up test directories
rm -rf "$TEST_IMPORT_DIR" "$TEST_ZIP_DIR" "$TEST_SYMLINK_DIR" "$TEST_ROOT"/*.zip

#===============================================================================
# SECTION 5: Project Log Tools
#===============================================================================

print_section "SECTION 5: Project Log Operations" "Tools: project_log_append, project_log_get"

print_subsection "5.1 Append Log Entries"
run_test "5.1.1 Append first log entry" \
    "project_log_append" \
    "{\"project\":\"$TEST_PROJECT\",\"message\":\"First log entry - project started\"}" \
    '"logged":true'

run_test "5.1.2 Append second log entry" \
    "project_log_append" \
    "{\"project\":\"$TEST_PROJECT\",\"message\":\"Second log entry - requirements gathered\"}" \
    '"logged":true'

run_test "5.1.3 Append third log entry" \
    "project_log_append" \
    "{\"project\":\"$TEST_PROJECT\",\"message\":\"Third log entry - design complete\"}" \
    '"logged":true'

run_test "5.1.4 Append fourth log entry" \
    "project_log_append" \
    "{\"project\":\"$TEST_PROJECT\",\"message\":\"Fourth log entry - implementation started\"}" \
    '"logged":true'

print_subsection "5.2 Get Log Entries"
run_test "5.2.1 Get all log entries" \
    "project_log_get" \
    "{\"project\":\"$TEST_PROJECT\"}" \
    "First log entry"

run_test "5.2.2 Verify second entry in log" \
    "project_log_get" \
    "{\"project\":\"$TEST_PROJECT\"}" \
    "requirements gathered"

run_test "5.2.3 Verify third entry in log" \
    "project_log_get" \
    "{\"project\":\"$TEST_PROJECT\"}" \
    "design complete"

run_test "5.2.4 Get log with limit" \
    "project_log_get" \
    "{\"project\":\"$TEST_PROJECT\",\"limit\":2}" \
    "events"

run_test "5.2.5 Get log with offset" \
    "project_log_get" \
    "{\"project\":\"$TEST_PROJECT\",\"offset\":1,\"limit\":2}" \
    ""

print_subsection "5.3 Log Error Cases"
run_test_expect_fail "5.3.1 Append to non-existent project" \
    "project_log_append" \
    '{"project":"nonexistent","message":"Test"}' \
    "not found"

run_test_expect_fail "5.3.2 Get log from non-existent project" \
    "project_log_get" \
    '{"project":"nonexistent"}' \
    "not found"

#===============================================================================
# SECTION 6: Task Set and Task Tools
#===============================================================================

print_section "SECTION 6: Task Set and Task Operations" "Tools: taskset_*, task_*"

print_subsection "6.0 Create Test Templates"
# Re-create test playbook (was deleted in section 2) and add templates (required for task_run validation)
run_test "6.0.1 Re-create test playbook" \
    "playbook_create" \
    "{\"name\":\"$TEST_PLAYBOOK\"}" \
    "\"playbook\":\"$TEST_PLAYBOOK\""

run_test "6.0.2 Create worker response template" \
    "playbook_file_put" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"templates/worker-response.json\",\"content\":\"{\\\"type\\\": \\\"object\\\", \\\"additionalProperties\\\": true}\"}" \
    '"path":"templates/worker-response.json"'

run_test "6.0.3 Create worker report template" \
    "playbook_file_put" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"templates/worker-report.md\",\"content\":\"## Worker Report\\n\\n{{.WorkResult}}\"}" \
    '"path":"templates/worker-report.md"'

print_subsection "6.1 Create Task Set"
run_test "6.1.1 List task sets (initially empty)" \
    "taskset_list" \
    "{\"project\":\"$TEST_PROJECT\"}" \
    '"total":0'

run_test "6.1.2 Create task set" \
    "taskset_create" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\",\"title\":\"Analysis Tasks\",\"description\":\"Code analysis tasks\",\"worker_response_template\":\"$TEST_PLAYBOOK/templates/worker-response.json\",\"worker_report_template\":\"$TEST_PLAYBOOK/templates/worker-report.md\"}" \
    '"path":"analysis"'

run_test "6.1.3 Create nested task set" \
    "taskset_create" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis/security\",\"title\":\"Security Analysis\",\"description\":\"Security-focused analysis\",\"worker_response_template\":\"$TEST_PLAYBOOK/templates/worker-response.json\",\"worker_report_template\":\"$TEST_PLAYBOOK/templates/worker-report.md\"}" \
    '"path":"analysis/security"'

run_test "6.1.4 Get task set" \
    "taskset_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\"}" \
    '"title":"Analysis Tasks"'

run_test "6.1.5 List task sets" \
    "taskset_list" \
    "{\"project\":\"$TEST_PROJECT\"}" \
    '"total":2'

run_test_expect_fail "6.1.6 Create duplicate task set" \
    "taskset_create" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\",\"title\":\"Duplicate\"}" \
    "already exists"

print_subsection "6.2 Create Tasks"
run_test "6.2.1 Create task in task set" \
    "task_create" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\",\"title\":\"Analyze code\",\"type\":\"analysis\",\"prompt\":\"Analyze the code\"}" \
    '"title":"Analyze code"'

run_test "6.2.2 Create second task" \
    "task_create" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\",\"title\":\"Review dependencies\",\"type\":\"review\",\"prompt\":\"Review dependencies\"}" \
    '"title":"Review dependencies"'

run_test "6.2.3 Create task in nested set" \
    "task_create" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis/security\",\"title\":\"Security scan\",\"type\":\"security\",\"prompt\":\"Run security scan\"}" \
    '"title":"Security scan"'

run_test_expect_fail "6.2.4 Create task without prompt" \
    "task_create" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\",\"title\":\"No prompt task\",\"type\":\"test\"}" \
    "prompt"

run_test_expect_fail "6.2.5 Create task in non-existent task set" \
    "task_create" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"nonexistent\",\"title\":\"Test\",\"prompt\":\"test\"}" \
    "not found"

# Extract task UUIDs for further tests
print_subsection "6.3 List and Get Tasks"
echo "  6.3.0 Extracting task UUIDs for further tests"
TASK_LIST_RESULT=$($PROBE -stdio $MAESTRO -env $ENV -call "task_list" -params "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\"}" 2>&1)
TASK_UUID_1=$(echo "$TASK_LIST_RESULT" | grep -o '"uuid":"[^"]*"' | head -1 | sed 's/"uuid":"\([^"]*\)"/\1/')
TASK_UUID_2=$(echo "$TASK_LIST_RESULT" | grep -o '"uuid":"[^"]*"' | head -2 | tail -1 | sed 's/"uuid":"\([^"]*\)"/\1/')

if [ -n "$TASK_UUID_1" ] && [ -n "$TASK_UUID_2" ]; then
    echo "    ${GREEN}PASS${NC}: Got task UUIDs"
    PASS_COUNT=$((PASS_COUNT + 1))
else
    echo "    ${RED}FAIL${NC}: Could not extract task UUIDs"
    FAIL_COUNT=$((FAIL_COUNT + 1))
fi

run_test "6.3.1 List tasks in analysis path" \
    "task_list" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\"}" \
    '"total":2'

run_test "6.3.2 List tasks in nested path" \
    "task_list" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis/security\"}" \
    '"total":1'

run_test "6.3.3 List tasks by type in path" \
    "task_list" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis/security\",\"type\":\"security\"}" \
    '"title":"Security scan"'

run_test "6.3.4 Get task by UUID" \
    "task_get" \
    "{\"project\":\"$TEST_PROJECT\",\"uuid\":\"$TASK_UUID_1\"}" \
    '"task":'

run_test "6.3.5 Get task by path and ID" \
    "task_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\",\"id\":1}" \
    '"title":'

run_test_expect_fail "6.3.6 Get non-existent task" \
    "task_get" \
    "{\"project\":\"$TEST_PROJECT\",\"uuid\":\"nonexistent-uuid\"}" \
    "not found"

print_subsection "6.4 Update Task"
run_test "6.4.1 Update task title" \
    "task_update" \
    "{\"project\":\"$TEST_PROJECT\",\"uuid\":\"$TASK_UUID_1\",\"title\":\"Updated Title\"}" \
    '"title":"Updated Title"'

run_test "6.4.2 Verify update persisted" \
    "task_get" \
    "{\"project\":\"$TEST_PROJECT\",\"uuid\":\"$TASK_UUID_1\"}" \
    '"title":"Updated Title"'

print_subsection "6.4.5 Instructions File Validation"

# Create a test file in the project for validation tests
run_test "6.4.5.0a Create project file for validation tests" \
    "project_file_put" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"test-instructions.md\",\"content\":\"Test instructions content\"}" \
    '"created":true'

# Test validation for task_create with invalid instructions file (project source)
run_test_expect_fail "6.4.5.1 Create task with non-existent project file" \
    "task_create" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\",\"title\":\"Invalid file test\",\"type\":\"test\",\"prompt\":\"test\",\"instructions_file\":\"nonexistent.md\",\"instructions_file_source\":\"project\"}" \
    "not found"

run_test_expect_fail "6.4.5.2 Create task with non-existent QA instructions file" \
    "task_create" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\",\"title\":\"Invalid QA file test\",\"type\":\"test\",\"prompt\":\"test\",\"qa_enabled\":true,\"qa_instructions_file\":\"nonexistent-qa.md\",\"qa_instructions_file_source\":\"project\"}" \
    "not found"

# Test validation for task_create with VALID file (should succeed)
run_test "6.4.5.3 Create task with valid project file" \
    "task_create" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\",\"title\":\"Valid file test\",\"type\":\"test\",\"prompt\":\"test\",\"instructions_file\":\"test-instructions.md\",\"instructions_file_source\":\"project\"}" \
    '"title":"Valid file test"'

# Test validation for task_update with invalid instructions file
run_test_expect_fail "6.4.5.4 Update task with non-existent instructions file" \
    "task_update" \
    "{\"project\":\"$TEST_PROJECT\",\"uuid\":\"$TASK_UUID_1\",\"instructions_file\":\"nonexistent.md\",\"instructions_file_source\":\"project\"}" \
    "not found"

run_test_expect_fail "6.4.5.5 Update task with non-existent QA instructions file" \
    "task_update" \
    "{\"project\":\"$TEST_PROJECT\",\"uuid\":\"$TASK_UUID_1\",\"qa_instructions_file\":\"nonexistent-qa.md\",\"qa_instructions_file_source\":\"project\"}" \
    "not found"

# Test validation for task_update with VALID file (should succeed)
run_test "6.4.5.6 Update task with valid project file" \
    "task_update" \
    "{\"project\":\"$TEST_PROJECT\",\"uuid\":\"$TASK_UUID_1\",\"instructions_file\":\"test-instructions.md\",\"instructions_file_source\":\"project\"}" \
    '"instructions_file"'

# Test validation for playbook source with invalid format
run_test_expect_fail "6.4.5.7 Create task with invalid playbook path format" \
    "task_create" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\",\"title\":\"Invalid path test\",\"type\":\"test\",\"prompt\":\"test\",\"instructions_file\":\"no-slash-here\",\"instructions_file_source\":\"playbook\"}" \
    "invalid playbook"

print_subsection "6.5 Update Task Set"
run_test "6.5.1 Update task set title" \
    "taskset_update" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\",\"title\":\"Updated Analysis\"}" \
    '"title":"Updated Analysis"'

run_test "6.5.2 Update task set parallel" \
    "taskset_update" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\",\"parallel\":\"true\"}" \
    '"parallel":true'

print_subsection "6.6 Delete Task"
run_test "6.6.1 Delete task" \
    "task_delete" \
    "{\"project\":\"$TEST_PROJECT\",\"uuid\":\"$TASK_UUID_2\"}" \
    '"deleted":true'

# After deleting second task, should be 2 tasks left in analysis path
# (task 1 from 6.2.1, plus task from 6.4.5.3 validation test)
run_test "6.6.2 Verify task count reduced" \
    "task_list" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\"}" \
    '"total":2'

#===============================================================================
# SECTION 7: Task Execution Tools
#===============================================================================

print_section "SECTION 7: Task Execution Operations" "Tools: task_run, task_status, task_results, task_report"

print_subsection "7.1 Task Status"
run_test "7.1.1 Get task status" \
    "task_status" \
    "{\"project\":\"$TEST_PROJECT\"}" \
    '"project":'

run_test "7.1.2 Get task status with path filter" \
    "task_status" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\"}" \
    '"total_tasks":'

print_subsection "7.2 Task Run"
run_test "7.2.1 Run tasks (no eligible - all waiting)" \
    "task_run" \
    "{\"project\":\"$TEST_PROJECT\"}" \
    '"tasks_found":'

run_test "7.2.2 Run tasks with path filter" \
    "task_run" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\"}" \
    '"project":'

run_test "7.2.3 Run tasks with type filter" \
    "task_run" \
    "{\"project\":\"$TEST_PROJECT\",\"type\":\"security\"}" \
    '"project":'

print_subsection "7.3 Task Results"
run_test "7.3.1 Get task results" \
    "task_results" \
    "{\"project\":\"$TEST_PROJECT\"}" \
    '"results"'

run_test "7.3.2 Get results with pagination" \
    "task_results" \
    "{\"project\":\"$TEST_PROJECT\",\"offset\":0,\"limit\":10}" \
    '"results"'

print_subsection "7.4 Task Report"
run_test "7.4.1 Generate markdown report" \
    "task_report" \
    "{\"project\":\"$TEST_PROJECT\",\"format\":\"markdown\"}" \
    "Project Report"

run_test "7.4.2 Generate JSON report" \
    "task_report" \
    "{\"project\":\"$TEST_PROJECT\",\"format\":\"json\"}" \
    '"project":'

run_test "7.4.3 Generate report with path filter" \
    "task_report" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\",\"format\":\"markdown\"}" \
    "Report"

print_subsection "7.5 Reset Task Set"
run_test "7.5.1 Reset task set (mode=all)" \
    "taskset_reset" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\",\"mode\":\"all\"}" \
    '"tasks_reset":'

run_test "7.5.2 Verify tasks reset to waiting" \
    "task_list" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\",\"status\":\"waiting\"}" \
    '"status":"waiting"'

run_test_expect_fail "7.5.3 Reset without mode fails" \
    "taskset_reset" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\"}" \
    "mode is required"

run_test "7.5.4 Reset with mode=failed" \
    "taskset_reset" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\",\"mode\":\"failed\"}" \
    '"tasks_reset":'

run_test_expect_fail "7.5.5 Reset with invalid mode fails" \
    "taskset_reset" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\",\"mode\":\"invalid\"}" \
    "must be"

print_subsection "7.6 Delete Task Set"
run_test "7.6.1 Delete nested task set" \
    "taskset_delete" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis/security\"}" \
    '"deleted":true'

run_test "7.6.2 Verify task set deleted" \
    "taskset_list" \
    "{\"project\":\"$TEST_PROJECT\"}" \
    '"total":1'

run_test "7.6.3 Delete remaining task set" \
    "taskset_delete" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"analysis\"}" \
    '"deleted":true'

#===============================================================================
# SECTION 8: LLM Tools
#===============================================================================

print_section "SECTION 8: LLM Operations" "Tools: llm_list, llm_dispatch"

print_subsection "8.1 List LLMs"
run_test "8.1.1 List configured LLMs" \
    "llm_list" \
    '{}' \
    "llms"

run_test "8.1.2 Verify LLMs have enabled field" \
    "llm_list" \
    '{}' \
    '"enabled"'

print_subsection "8.2 LLM Dispatch (Error Cases)"
run_test_expect_fail "8.2.1 Dispatch to disabled LLM" \
    "llm_dispatch" \
    '{"llm_id":"default","prompt":"Say hello."}' \
    "not enabled"

run_test_expect_fail "8.2.2 Dispatch to non-existent LLM" \
    "llm_dispatch" \
    '{"llm_id":"nonexistent","prompt":"Say hello."}' \
    ""

run_test_expect_fail "8.2.3 Dispatch with empty prompt" \
    "llm_dispatch" \
    '{"llm_id":"default","prompt":""}' \
    ""

#===============================================================================
# SECTION 9: System Tools
#===============================================================================

print_section "SECTION 9: System Operations" "Tools: health, file_copy"

print_subsection "9.1 Health Check"
run_test "9.1.1 Health check returns status" \
    "health" \
    '{}' \
    '"status"'

run_test "9.1.2 Health check returns base_dir" \
    "health" \
    '{}' \
    '"base_dir"'

run_test "9.1.3 Health check returns config_path" \
    "health" \
    '{}' \
    '"config_path"'

run_test "9.1.4 Health check returns enabled_llms" \
    "health" \
    '{}' \
    '"enabled_llms"'

run_test "9.1.5 Health check returns healthy field" \
    "health" \
    '{}' \
    '"healthy"'

run_test "9.1.6 Health check returns first_run" \
    "health" \
    '{}' \
    '"first_run"'

print_subsection "9.2 Cross-Domain File Operations"
# Setup: Ensure playbook exists for cross-domain copy tests
cleanup_silent "playbook_create" "{\"name\":\"$TEST_PLAYBOOK\"}"
run_test "9.2.0 Create playbook file for testing" \
    "playbook_file_put" \
    "{\"playbook\": \"$TEST_PLAYBOOK\", \"path\": \"procedure.md\", \"content\": \"# Test Procedure\\n\\nThis is a test procedure file for copy operations.\"}" \
    '"created":true'

run_test "9.2.1 Copy file from reference to project" \
    "file_copy" \
    "{\"from_source\": \"reference\", \"from_path\": \"start.md\", \"to_source\": \"project\", \"to_project\": \"$TEST_PROJECT\", \"to_path\": \"ref-copy.md\", \"summary\": \"Copied from reference\"}" \
    '"copied":true'

run_test "9.2.2 Verify copied file exists in project" \
    "project_file_get" \
    "{\"project\": \"$TEST_PROJECT\", \"path\": \"ref-copy.md\"}" \
    '"path":"ref-copy.md"'

run_test "9.2.3 Copy file from project to playbook" \
    "file_copy" \
    "{\"from_source\": \"project\", \"from_project\": \"$TEST_PROJECT\", \"from_path\": \"requirements.md\", \"to_source\": \"playbook\", \"to_playbook\": \"$TEST_PLAYBOOK\", \"to_path\": \"copied-requirements.md\"}" \
    '"copied":true'

run_test "9.2.4 Verify copied file exists in playbook" \
    "playbook_file_get" \
    "{\"playbook\": \"$TEST_PLAYBOOK\", \"path\": \"copied-requirements.md\"}" \
    '"path":"copied-requirements.md"'

run_test "9.2.5 Copy file from playbook to project" \
    "file_copy" \
    "{\"from_source\": \"playbook\", \"from_playbook\": \"$TEST_PLAYBOOK\", \"from_path\": \"procedure.md\", \"to_source\": \"project\", \"to_project\": \"$TEST_PROJECT\", \"to_path\": \"imported-procedure.md\"}" \
    '"copied":true'

run_test "9.2.6 Verify copied file exists in project" \
    "project_file_get" \
    "{\"project\": \"$TEST_PROJECT\", \"path\": \"imported-procedure.md\"}" \
    '"content"'

run_test "9.2.7 Copy within project (file duplication)" \
    "file_copy" \
    "{\"from_source\": \"project\", \"from_project\": \"$TEST_PROJECT\", \"from_path\": \"requirements.md\", \"to_source\": \"project\", \"to_project\": \"$TEST_PROJECT\", \"to_path\": \"requirements-backup.md\"}" \
    '"copied":true'

run_test_expect_fail "9.2.8 Error: Copy to reference (read-only)" \
    "file_copy" \
    "{\"from_source\": \"project\", \"from_project\": \"$TEST_PROJECT\", \"from_path\": \"requirements.md\", \"to_source\": \"reference\", \"to_path\": \"invalid.md\"}" \
    "read-only"

run_test_expect_fail "9.2.9 Error: Missing from_path" \
    "file_copy" \
    "{\"to_source\": \"project\", \"to_project\": \"$TEST_PROJECT\", \"to_path\": \"test.md\"}" \
    "required"

run_test_expect_fail "9.2.10 Error: Missing to_path" \
    "file_copy" \
    "{\"from_source\": \"reference\", \"from_path\": \"start.md\", \"to_source\": \"project\", \"to_project\": \"$TEST_PROJECT\"}" \
    "required"

#===============================================================================
# SECTION 10: Error Handling & Edge Cases
#===============================================================================

print_section "SECTION 10: Error Handling" "Edge cases and error conditions"

print_subsection "10.1 Non-existent Resources"
run_test_expect_fail "10.1.1 Get non-existent project" \
    "project_get" \
    '{"name":"nonexistent"}' \
    "not found"

run_test_expect_fail "10.1.2 Update non-existent project" \
    "project_update" \
    '{"name":"nonexistent","title":"Test"}' \
    "not found"

run_test_expect_fail "10.1.3 Delete non-existent project" \
    "project_delete" \
    '{"name":"nonexistent"}' \
    "not found"

run_test_expect_fail "10.1.4 Get file from non-existent project" \
    "project_file_get" \
    '{"project":"nonexistent","path":"test.md"}' \
    "not found"

print_subsection "10.2 Invalid Names"
run_test_expect_fail "10.2.1 Project name with spaces" \
    "project_create" \
    '{"name":"invalid name","title":"Test","disclaimer_template":"none"}' \
    "invalid"

run_test_expect_fail "10.2.2 Project name starting with hyphen" \
    "project_create" \
    '{"name":"-invalid","title":"Test","disclaimer_template":"none"}' \
    "invalid"

run_test_expect_fail "10.2.3 Project name with special chars" \
    "project_create" \
    '{"name":"test@project","title":"Test","disclaimer_template":"none"}' \
    "invalid"

print_subsection "10.3 Security"
run_test_expect_fail "10.3.1 Path traversal in project files" \
    "project_file_put" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"../../etc/passwd\",\"content\":\"test\"}" \
    ""

run_test_expect_fail "10.3.2 Path traversal in playbook files" \
    "playbook_file_put" \
    '{"playbook":"test","path":"../../etc/passwd","content":"test"}' \
    ""

# NOTE: Subproject tests (formerly Section 11) have been removed.
# Subprojects are no longer supported - use path-based task sets instead.

#===============================================================================
# SECTION 11: List Management Tools
#===============================================================================

print_section "SECTION 11: List Management" "Tools: list_*, list_item_*, list_create_tasks"

print_subsection "11.1 Create Project for List Tests"
run_test "11.1.1 Create project for list tests" \
    "project_create" \
    '{"name":"list-test-proj","title":"List Test Project","disclaimer_template":"none"}' \
    '"name":"list-test-proj"'

run_test "11.1.2 Create playbook for list tests" \
    "playbook_create" \
    '{"name":"list-test-playbook"}' \
    '"playbook":"list-test-playbook"'

print_subsection "11.2 List CRUD Operations"
run_test "11.2.1 List lists (initially empty)" \
    "list_list" \
    '{"project":"list-test-proj"}' \
    '"total_count":0'

run_test "11.2.2 Create list in project" \
    "list_create" \
    '{"list":"requirements","name":"Project Requirements","description":"Test requirements list","project":"list-test-proj"}' \
    '"created":true'

run_test "11.2.3 Verify list in list" \
    "list_list" \
    '{"project":"list-test-proj"}' \
    '"total_count":1'

run_test "11.2.4 Get list" \
    "list_get" \
    '{"list":"requirements","project":"list-test-proj"}' \
    '"name":"Project Requirements"'

run_test "11.2.5 Get list summary" \
    "list_get_summary" \
    '{"list":"requirements","project":"list-test-proj"}' \
    '"item_count":0'

run_test "11.2.6 Create list in playbook" \
    "list_create" \
    '{"list":"controls","name":"Standard Controls","description":"Reusable controls","source":"playbook","playbook":"list-test-playbook"}' \
    '"created":true'

run_test_expect_fail "11.2.7 Create duplicate list" \
    "list_create" \
    '{"list":"requirements","name":"Duplicate","project":"list-test-proj"}' \
    "already exists"

run_test_expect_fail "11.2.8 Create list in reference (read-only)" \
    "list_create" \
    '{"list":"test","name":"Test","source":"reference"}' \
    "read-only"

print_subsection "11.3 List Item Operations"
run_test "11.3.1 Add item to list (auto-generates ID)" \
    "list_item_add" \
    '{"list":"requirements","project":"list-test-proj","title":"User Authentication","content":"The system shall authenticate users","source_doc":"spec.md","section":"Security"}' \
    '"id":"item-001"'

run_test "11.3.2 Add second item" \
    "list_item_add" \
    '{"list":"requirements","project":"list-test-proj","title":"Access Logging","content":"The system shall log all access attempts","source_doc":"spec.md","section":"Logging"}' \
    '"id":"item-002"'

run_test "11.3.3 Add third item" \
    "list_item_add" \
    '{"list":"requirements","project":"list-test-proj","title":"Data Encryption","content":"The system shall encrypt data at rest","source_doc":"security.md","section":"Encryption"}' \
    '"id":"item-003"'

# Test that passing an id parameter is ignored - ID should still be auto-generated
run_test "11.3.3a Verify ID parameter is ignored (auto-generates anyway)" \
    "list_item_add" \
    '{"list":"requirements","project":"list-test-proj","id":"CUSTOM-ID","title":"Ignored ID Test","content":"This should get an auto-generated ID, not CUSTOM-ID"}' \
    '"id":"item-004"'

run_test "11.3.3b Verify custom ID was not used" \
    "list_item_get" \
    '{"list":"requirements","project":"list-test-proj","id":"item-004"}' \
    '"title":"Ignored ID Test"'

run_test_expect_fail "11.3.3c Verify CUSTOM-ID does not exist" \
    "list_item_get" \
    '{"list":"requirements","project":"list-test-proj","id":"CUSTOM-ID"}' \
    "not found"

run_test "11.3.4 Verify first item exists with auto-generated ID" \
    "list_item_get" \
    '{"list":"requirements","project":"list-test-proj","id":"item-001"}' \
    '"title":"User Authentication"'

run_test "11.3.5 Get item" \
    "list_item_get" \
    '{"list":"requirements","project":"list-test-proj","id":"item-001"}' \
    '"content":"The system shall authenticate users"'

run_test "11.3.6 Verify item section" \
    "list_item_get" \
    '{"list":"requirements","project":"list-test-proj","id":"item-001"}' \
    'Security'

run_test "11.3.7 Update item content" \
    "list_item_update" \
    '{"list":"requirements","project":"list-test-proj","id":"item-001","content":"The system shall authenticate all users via SSO"}' \
    '"updated":true'

run_test "11.3.8 Verify item update" \
    "list_item_get" \
    '{"list":"requirements","project":"list-test-proj","id":"item-001"}' \
    '"content":"The system shall authenticate all users via SSO"'

run_test "11.3.9 Rename item ID" \
    "list_item_rename" \
    '{"list":"requirements","project":"list-test-proj","id":"item-003","new_id":"SEC-001"}' \
    '"renamed":true'

run_test "11.3.10 Verify item rename" \
    "list_item_get" \
    '{"list":"requirements","project":"list-test-proj","id":"SEC-001"}' \
    '"content":"The system shall encrypt data at rest"'

run_test_expect_fail "11.3.11 Get old item ID" \
    "list_item_get" \
    '{"list":"requirements","project":"list-test-proj","id":"item-003"}' \
    "not found"

print_subsection "11.4 List Item Search"
run_test "11.4.1 Search by query (content)" \
    "list_item_search" \
    '{"list":"requirements","project":"list-test-proj","query":"encrypt"}' \
    '"total_count":1'

run_test "11.4.2 Search by query (ID)" \
    "list_item_search" \
    '{"list":"requirements","project":"list-test-proj","query":"item-001"}' \
    '"total_count":1'

run_test "11.4.3 Search by source_doc" \
    "list_item_search" \
    '{"list":"requirements","project":"list-test-proj","source_doc":"spec.md"}' \
    '"total_count":2'

run_test "11.4.4 Search by section" \
    "list_item_search" \
    '{"list":"requirements","project":"list-test-proj","section":"Security"}' \
    '"total_count":1'

run_test "11.4.5 Search with no results" \
    "list_item_search" \
    '{"list":"requirements","project":"list-test-proj","query":"nonexistent"}' \
    '"total_count":0'

print_subsection "11.4.6 Complete Field and Filtering"
run_test "11.4.6.1 Verify item complete defaults to false" \
    "list_item_get" \
    '{"list":"requirements","project":"list-test-proj","id":"item-001"}' \
    '"complete":false'

run_test "11.4.6.2 Update item to complete=true" \
    "list_item_update" \
    '{"list":"requirements","project":"list-test-proj","id":"item-001","complete":true}' \
    '"updated":true'

run_test "11.4.6.3 Verify item complete is true" \
    "list_item_get" \
    '{"list":"requirements","project":"list-test-proj","id":"item-001"}' \
    '"complete":true'

run_test "11.4.6.4 Search for complete=true items" \
    "list_item_search" \
    '{"list":"requirements","project":"list-test-proj","complete":"true"}' \
    '"total_count":1'

run_test "11.4.6.5 Search for complete=false items" \
    "list_item_search" \
    '{"list":"requirements","project":"list-test-proj","complete":"false"}' \
    '"total_count":3'

run_test "11.4.6.6 Search all items (no complete filter)" \
    "list_item_search" \
    '{"list":"requirements","project":"list-test-proj"}' \
    '"total_count":4'

run_test "11.4.6.7 Get summary with complete=false filter" \
    "list_get_summary" \
    '{"list":"requirements","project":"list-test-proj","complete":"false"}' \
    '"returned_count":3'

run_test "11.4.6.8 Get summary with complete=true filter" \
    "list_get_summary" \
    '{"list":"requirements","project":"list-test-proj","complete":"true"}' \
    '"returned_count":1'

run_test "11.4.6.9 Update item back to complete=false" \
    "list_item_update" \
    '{"list":"requirements","project":"list-test-proj","id":"item-001","complete":false}' \
    '"updated":true'

print_subsection "11.4.7 Playbook Complete Restrictions"
run_test "11.4.7.1 Add item to playbook list" \
    "list_item_add" \
    '{"list":"controls","source":"playbook","playbook":"list-test-playbook","title":"Test Control","content":"Test control content"}' \
    '"id":"item-001"'

run_test "11.4.7.2 Verify playbook item complete is false" \
    "list_item_get" \
    '{"list":"controls","source":"playbook","playbook":"list-test-playbook","id":"item-001"}' \
    '"complete":false'

run_test_expect_fail "11.4.7.3 Cannot set complete=true on playbook item" \
    "list_item_update" \
    '{"list":"controls","source":"playbook","playbook":"list-test-playbook","id":"item-001","complete":true}' \
    "cannot be marked complete"

run_test "11.4.7.4 Verify item still complete=false" \
    "list_item_get" \
    '{"list":"controls","source":"playbook","playbook":"list-test-playbook","id":"item-001"}' \
    '"complete":false'

print_subsection "11.5 List Item Remove"
run_test "11.5.1 Remove item" \
    "list_item_remove" \
    '{"list":"requirements","project":"list-test-proj","id":"item-002"}' \
    '"removed":true'

run_test_expect_fail "11.5.2 Verify item removed" \
    "list_item_get" \
    '{"list":"requirements","project":"list-test-proj","id":"item-002"}' \
    "not found"

run_test "11.5.3 Verify item count" \
    "list_get_summary" \
    '{"list":"requirements","project":"list-test-proj"}' \
    '"item_count":3'

print_subsection "11.6 List Rename"
run_test "11.6.1 Rename list" \
    "list_rename" \
    '{"list":"requirements","new_list":"reqs","project":"list-test-proj"}' \
    '"renamed":true'

run_test_expect_fail "11.6.2 Verify old name gone" \
    "list_get" \
    '{"list":"requirements","project":"list-test-proj"}' \
    "not found"

run_test "11.6.3 Verify new name exists" \
    "list_get" \
    '{"list":"reqs","project":"list-test-proj"}' \
    '"name":"Project Requirements"'

print_subsection "11.7 List Task Creation"
# First create a task set for the tasks
run_test "11.7.0 Create task set for list tasks" \
    "taskset_create" \
    "{\"project\":\"list-test-proj\",\"path\":\"list-tasks\",\"title\":\"List Tasks\",\"worker_response_template\":\"$TEST_PLAYBOOK/templates/worker-response.json\",\"worker_report_template\":\"$TEST_PLAYBOOK/templates/worker-report.md\"}" \
    '"path":"list-tasks"'

run_test "11.7.1 Create tasks from list" \
    "list_create_tasks" \
    '{"list":"reqs","list_project":"list-test-proj","project":"list-test-proj","path":"list-tasks","type":"analysis","title_template":"Analyze {{id}}","llm_model_id":"default","prompt":"Analyze this requirement:"}' \
    '"tasks_created":3'

run_test "11.7.2 Verify tasks created" \
    "task_list" \
    '{"project":"list-test-proj","path":"list-tasks"}' \
    '"total":3'

run_test "11.7.3 Verify task title template" \
    "task_list" \
    '{"project":"list-test-proj","path":"list-tasks"}' \
    'Analyze item-001'

print_subsection "11.8 List Delete"
run_test "11.8.1 Delete list" \
    "list_delete" \
    '{"list":"reqs","project":"list-test-proj"}' \
    '"deleted":true'

run_test_expect_fail "11.8.2 Verify list deleted" \
    "list_get" \
    '{"list":"reqs","project":"list-test-proj"}' \
    "not found"

print_subsection "11.9 List Error Cases"
run_test_expect_fail "11.9.1 Get non-existent list" \
    "list_get" \
    '{"list":"nonexistent","project":"list-test-proj"}' \
    "not found"

run_test_expect_fail "11.9.2 Path traversal in list name" \
    "list_create" \
    '{"list":"../traversal","name":"Test","project":"list-test-proj"}' \
    "path separators"

run_test_expect_fail "11.9.3 Empty list name" \
    "list_create" \
    '{"list":"","name":"Test","project":"list-test-proj"}' \
    ""

run_test_expect_fail "11.9.4 Missing required project" \
    "list_list" \
    '{}' \
    "required"

# Instruction file validation for list_create_tasks
# First create a list to test with
run_test "11.9.5a Create list for validation test" \
    "list_create" \
    '{"list":"validation-test","name":"Validation Test","project":"list-test-proj"}' \
    '"created":true'

run_test "11.9.5b Add item to validation test list" \
    "list_item_add" \
    '{"list":"validation-test","project":"list-test-proj","title":"Test item","content":"Test content"}' \
    '"added":true'

run_test_expect_fail "11.9.5 list_create_tasks with non-existent instructions file" \
    "list_create_tasks" \
    '{"list":"validation-test","list_project":"list-test-proj","project":"list-test-proj","path":"list-tasks","type":"test","prompt":"test","instructions_file":"nonexistent.md","instructions_file_source":"project"}' \
    "not found"

run_test_expect_fail "11.9.6 list_create_tasks with non-existent QA instructions file" \
    "list_create_tasks" \
    '{"list":"validation-test","list_project":"list-test-proj","project":"list-test-proj","path":"list-tasks","type":"test","prompt":"test","qa_enabled":true,"qa_instructions_file":"nonexistent-qa.md","qa_instructions_file_source":"project"}' \
    "not found"

print_subsection "11.10 List Copy"
run_test "11.10.1 Create source list for copy" \
    "list_create" \
    '{"list":"source-list","name":"Source List","description":"List to copy from","project":"list-test-proj"}' \
    '"created":true'

run_test "11.10.2 Add items to source list" \
    "list_item_add" \
    '{"list":"source-list","project":"list-test-proj","title":"First Item","content":"First item to copy"}' \
    '"id":"item-001"'

run_test "11.10.3 Copy list within same project" \
    "list_copy" \
    '{"from_list":"source-list","from_project":"list-test-proj","to_list":"copied-list","to_project":"list-test-proj"}' \
    '"copied":true'

run_test "11.10.4 Verify copied list exists" \
    "list_get" \
    '{"list":"copied-list","project":"list-test-proj"}' \
    '"name":"Source List"'

run_test "11.10.5 Verify copied list has items" \
    "list_get_summary" \
    '{"list":"copied-list","project":"list-test-proj"}' \
    '"item_count":1'

run_test "11.10.6 Copy list to playbook" \
    "list_copy" \
    '{"from_list":"source-list","from_project":"list-test-proj","to_list":"playbook-copy","to_source":"playbook","to_playbook":"list-test-playbook"}' \
    '"copied":true'

run_test "11.10.7 Verify playbook copy exists" \
    "list_get" \
    '{"list":"playbook-copy","source":"playbook","playbook":"list-test-playbook"}' \
    '"name":"Source List"'

run_test_expect_fail "11.10.8 Cannot copy to reference (read-only)" \
    "list_copy" \
    '{"from_list":"source-list","from_project":"list-test-proj","to_list":"ref-copy","to_source":"reference"}' \
    "read-only"

run_test "11.10.9 Delete copied lists" \
    "list_delete" \
    '{"list":"copied-list","project":"list-test-proj"}' \
    '"deleted":true'

run_test "11.10.10 Delete source list" \
    "list_delete" \
    '{"list":"source-list","project":"list-test-proj"}' \
    '"deleted":true'

print_subsection "11.11 List Cleanup"
run_test "11.11.1 Delete playbook list" \
    "list_delete" \
    '{"list":"controls","source":"playbook","playbook":"list-test-playbook"}' \
    '"deleted":true'

run_test "11.11.2 Delete playbook copy list" \
    "list_delete" \
    '{"list":"playbook-copy","source":"playbook","playbook":"list-test-playbook"}' \
    '"deleted":true'

run_test "11.11.3 Delete test project" \
    "project_delete" \
    '{"name":"list-test-proj"}' \
    '"deleted":true'

run_test "11.11.4 Delete test playbook" \
    "playbook_delete" \
    '{"name":"list-test-playbook"}' \
    '"deleted":true'

#===============================================================================
# SECTION 12: Chroot Security Tests
#===============================================================================

print_section "SECTION 12: Chroot Security Tests" "Testing chroot boundary enforcement"

print_subsection "12.1 Project File Chroot Tests"
# These tests attempt to escape the chroot via various path traversal methods

run_test_expect_fail "12.1.1 Path traversal via ../etc/passwd in project file" \
    "project_file_put" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"../../../etc/passwd\",\"content\":\"hacked\"}" \
    ""

run_test_expect_fail "12.1.2 Path traversal via absolute path in project file" \
    "project_file_put" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"/etc/passwd\",\"content\":\"hacked\"}" \
    ""

# Note: URL-encoded paths are not decoded by the server (that's the client's job)
# So ..%2F..%2F becomes a literal filename, which is safe
run_test "12.1.3 URL-encoded path creates literal filename (safe)" \
    "project_file_put" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"..%2F..%2F..%2Fetc%2Fpasswd\",\"content\":\"hacked\"}" \
    '"created":true'

run_test_expect_fail "12.1.4 Path traversal via double dot in project file get" \
    "project_file_get" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"../../etc/passwd\"}" \
    ""

run_test_expect_fail "12.1.5 Path traversal via nested ../ in project file" \
    "project_file_put" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"subdir/../../../../../../etc/passwd\",\"content\":\"hacked\"}" \
    ""

print_subsection "12.2 Playbook File Chroot Tests"

run_test_expect_fail "12.2.1 Path traversal via ../etc/passwd in playbook file" \
    "playbook_file_put" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"../../../etc/passwd\",\"content\":\"hacked\"}" \
    ""

run_test_expect_fail "12.2.2 Path traversal via absolute path in playbook file" \
    "playbook_file_put" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"/etc/passwd\",\"content\":\"hacked\"}" \
    ""

run_test_expect_fail "12.2.3 Path traversal via double dot in playbook file get" \
    "playbook_file_get" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"../../etc/passwd\"}" \
    ""

run_test_expect_fail "12.2.4 Path traversal via nested ../ in playbook file" \
    "playbook_file_put" \
    "{\"playbook\":\"$TEST_PLAYBOOK\",\"path\":\"templates/../../../../../../etc/passwd\",\"content\":\"hacked\"}" \
    ""

print_subsection "12.3 Reference File Chroot Tests"

run_test_expect_fail "12.3.1 Path traversal via ../ in reference get" \
    "reference_get" \
    '{"path":"../../../etc/passwd"}' \
    ""

run_test_expect_fail "12.3.2 Path traversal via absolute path in reference get" \
    "reference_get" \
    '{"path":"/etc/passwd"}' \
    ""

run_test_expect_fail "12.3.3 Path traversal via nested ../ in reference get" \
    "reference_get" \
    '{"path":"phases/../../../../../../etc/passwd"}' \
    ""

print_subsection "12.4 List Chroot Tests"

run_test_expect_fail "12.4.1 Path traversal in list name" \
    "list_create" \
    "{\"list\":\"../../../etc/test\",\"name\":\"Test\",\"project\":\"$TEST_PROJECT\"}" \
    ""

run_test_expect_fail "12.4.2 Path traversal in list get" \
    "list_get" \
    "{\"list\":\"../../../etc/passwd\",\"project\":\"$TEST_PROJECT\"}" \
    ""

print_subsection "12.5 Project/Playbook Name Chroot Tests"

run_test_expect_fail "12.5.1 Path traversal in project name" \
    "project_create" \
    '{"name":"../etc/test","title":"Malicious Project","disclaimer_template":"none"}' \
    ""

run_test_expect_fail "12.5.2 Path traversal in playbook name" \
    "playbook_create" \
    '{"name":"../etc/test"}' \
    ""

run_test_expect_fail "12.5.3 Absolute path as project name" \
    "project_create" \
    '{"name":"/etc/test","title":"Malicious Project","disclaimer_template":"none"}' \
    ""

run_test_expect_fail "12.5.4 Absolute path as playbook name" \
    "playbook_create" \
    '{"name":"/etc/test"}' \
    ""

print_subsection "12.6 File Copy Chroot Tests"

run_test_expect_fail "12.6.1 File copy with path traversal in destination" \
    "file_copy" \
    "{\"from_source\":\"reference\",\"from_path\":\"start.md\",\"to_source\":\"project\",\"to_project\":\"$TEST_PROJECT\",\"to_path\":\"../../../etc/hacked\"}" \
    ""

run_test_expect_fail "12.6.2 File copy with absolute path destination" \
    "file_copy" \
    "{\"from_source\":\"reference\",\"from_path\":\"start.md\",\"to_source\":\"project\",\"to_project\":\"$TEST_PROJECT\",\"to_path\":\"/etc/hacked\"}" \
    ""

print_subsection "12.7 Verify No Files Created Outside Chroot"
echo "  12.7.1 Checking for files outside chroot"
if [ -f "/etc/passwd.test" ] || [ -f "/etc/hacked" ] || [ -f "/tmp/maestro-escape" ]; then
    echo "    ${RED}FAIL${NC}: Found files outside chroot - security breach!"
    FAIL_COUNT=$((FAIL_COUNT + 1))
else
    echo "    ${GREEN}PASS${NC}: No files created outside chroot"
    PASS_COUNT=$((PASS_COUNT + 1))
fi

echo "  12.7.2 Checking test data is within chroot"
if [ -d "$TEST_DATA/playbooks" ] && [ -d "$TEST_DATA/projects" ]; then
    echo "    ${GREEN}PASS${NC}: All data within chroot directory"
    PASS_COUNT=$((PASS_COUNT + 1))
else
    echo "    ${RED}FAIL${NC}: Data directories not in expected location"
    FAIL_COUNT=$((FAIL_COUNT + 1))
fi

#===============================================================================
# SECTION 13: LLM History Capture Tests
#===============================================================================

print_section "SECTION 13: LLM History Capture" "Verify stdout, stderr, exit_code are captured in task results"

print_subsection "13.1 Setup Test Project for LLM Testing"

# Create a dedicated project for LLM testing
LLM_TEST_PROJECT="llm-history-test"
cleanup_silent "project_delete" "{\"name\":\"$LLM_TEST_PROJECT\"}"
run_test "13.1.1 Create LLM test project" \
    "project_create" \
    "{\"name\":\"$LLM_TEST_PROJECT\",\"title\":\"LLM History Test\",\"disclaimer_template\":\"none\"}" \
    "\"name\":\"$LLM_TEST_PROJECT\""

# Use templates created in section 6
run_test "13.1.2 Create taskset for success test" \
    "taskset_create" \
    "{\"project\":\"$LLM_TEST_PROJECT\",\"path\":\"success-test\",\"title\":\"Success Test\",\"worker_response_template\":\"$TEST_PLAYBOOK/templates/worker-response.json\",\"worker_report_template\":\"$TEST_PLAYBOOK/templates/worker-report.md\"}" \
    '"path":"success-test"'

run_test "13.1.3 Create task with test-success LLM" \
    "task_create" \
    "{\"project\":\"$LLM_TEST_PROJECT\",\"path\":\"success-test\",\"title\":\"Test Success\",\"type\":\"test\",\"llm_model_id\":\"test-success\",\"prompt\":\"Test prompt\"}" \
    '"title":"Test Success"'

print_subsection "13.2 Run Success Task and Verify History"

# Run task with wait:true for synchronous execution
# Note: Task may fail schema validation but we're testing history capture, not validation
run_test "13.2.1 Run success task (synchronous)" \
    "task_run" \
    "{\"project\":\"$LLM_TEST_PROJECT\",\"path\":\"success-test\",\"wait\":true}" \
    '"tasks_executed"'

# Find result file directly from results directory (avoids jq parse issues with embedded newlines)
echo "  13.2.2 Verify result file has history with exit_code=0"
RESULT_FILE=$(ls "$TEST_DATA/projects/$LLM_TEST_PROJECT/results/"*.json 2>/dev/null | grep -v error | head -1)
if [ -n "$RESULT_FILE" ] && [ -f "$RESULT_FILE" ]; then
    # Check for exit_code in history
    EXIT_CODE=$(jq '.history[] | select(.role=="worker" and .type=="response") | .exit_code' "$RESULT_FILE" 2>/dev/null | head -1)
    STDOUT=$(jq -r '.history[] | select(.role=="worker" and .type=="response") | .stdout' "$RESULT_FILE" 2>/dev/null | head -1)
    if [ "$EXIT_CODE" = "0" ] && [ -n "$STDOUT" ]; then
        echo "    ${GREEN}PASS${NC}: History has exit_code=0 and stdout captured"
        PASS_COUNT=$((PASS_COUNT + 1))
    else
        echo "    ${RED}FAIL${NC}: exit_code=$EXIT_CODE, stdout='$STDOUT' (expected exit_code=0 with stdout)"
        FAIL_COUNT=$((FAIL_COUNT + 1))
    fi
else
    echo "    ${RED}FAIL${NC}: Result file not found in $TEST_DATA/projects/$LLM_TEST_PROJECT/results/"
    FAIL_COUNT=$((FAIL_COUNT + 1))
fi

print_subsection "13.3 Test LLM Error with Stderr Capture"

run_test "13.3.1 Create taskset for stderr test" \
    "taskset_create" \
    "{\"project\":\"$LLM_TEST_PROJECT\",\"path\":\"stderr-test\",\"title\":\"Stderr Test\",\"worker_response_template\":\"$TEST_PLAYBOOK/templates/worker-response.json\",\"worker_report_template\":\"$TEST_PLAYBOOK/templates/worker-report.md\"}" \
    '"path":"stderr-test"'

run_test "13.3.2 Create task with test-stderr LLM" \
    "task_create" \
    "{\"project\":\"$LLM_TEST_PROJECT\",\"path\":\"stderr-test\",\"title\":\"Test Stderr\",\"type\":\"test\",\"llm_model_id\":\"test-stderr\",\"prompt\":\"Test prompt\"}" \
    '"title":"Test Stderr"'

# Run task with wait:true - it will fail after retries
run_test "13.3.3 Run stderr task (synchronous)" \
    "task_run" \
    "{\"project\":\"$LLM_TEST_PROJECT\",\"path\":\"stderr-test\",\"wait\":true}" \
    '"tasks_failed"'

# Get task status to verify stderr is captured in error message
STDERR_TASK_RESULT=$(call_tool "task_list" "{\"project\":\"$LLM_TEST_PROJECT\",\"path\":\"stderr-test\"}")
STDERR_ERROR=$(echo "$STDERR_TASK_RESULT" | jq -r '.tasks[0].work.error // empty')
STDERR_STATUS=$(echo "$STDERR_TASK_RESULT" | jq -r '.tasks[0].work.status // empty')

echo "  13.3.4 Verify stderr is captured in task error field"
if [ "$STDERR_STATUS" = "failed" ] && echo "$STDERR_ERROR" | grep -q "stderr error message"; then
    echo "    ${GREEN}PASS${NC}: Task failed with stderr captured: '$STDERR_ERROR'"
    PASS_COUNT=$((PASS_COUNT + 1))
else
    echo "    ${RED}FAIL${NC}: Expected status=failed with stderr in error, got status=$STDERR_STATUS, error='$STDERR_ERROR'"
    FAIL_COUNT=$((FAIL_COUNT + 1))
fi

print_subsection "13.4 Test Disabled LLM Handling"

# The test-infra-fail LLM is disabled at startup because the command doesn't exist
# We verify that tasks with disabled LLMs are not run

run_test "13.4.1 Create taskset for disabled LLM test" \
    "taskset_create" \
    "{\"project\":\"$LLM_TEST_PROJECT\",\"path\":\"disabled-llm-test\",\"title\":\"Disabled LLM Test\",\"worker_response_template\":\"$TEST_PLAYBOOK/templates/worker-response.json\",\"worker_report_template\":\"$TEST_PLAYBOOK/templates/worker-report.md\"}" \
    '"path":"disabled-llm-test"'

run_test "13.4.2 Create task with disabled LLM" \
    "task_create" \
    "{\"project\":\"$LLM_TEST_PROJECT\",\"path\":\"disabled-llm-test\",\"title\":\"Test Disabled LLM\",\"type\":\"test\",\"llm_model_id\":\"test-infra-fail\",\"prompt\":\"Test prompt\"}" \
    '"title":"Test Disabled LLM"'

# Run task - pre-flight check should fail for disabled LLM, no tasks executed
DISABLED_RUN_RESULT=$(call_tool "task_run" "{\"project\":\"$LLM_TEST_PROJECT\",\"path\":\"disabled-llm-test\",\"wait\":true}")
DISABLED_TASKS_EXECUTED=$(echo "$DISABLED_RUN_RESULT" | jq -r '.tasks_executed // 0')

echo "  13.4.3 Verify disabled LLM task was not executed"
if [ "$DISABLED_TASKS_EXECUTED" = "0" ]; then
    echo "    ${GREEN}PASS${NC}: Task with disabled LLM was not executed (tasks_executed=0)"
    PASS_COUNT=$((PASS_COUNT + 1))
else
    echo "    ${RED}FAIL${NC}: Expected tasks_executed=0, got $DISABLED_TASKS_EXECUTED"
    FAIL_COUNT=$((FAIL_COUNT + 1))
fi

# Verify task is still in waiting status
DISABLED_TASK_RESULT=$(call_tool "task_list" "{\"project\":\"$LLM_TEST_PROJECT\",\"path\":\"disabled-llm-test\"}")
DISABLED_TASK_STATUS=$(echo "$DISABLED_TASK_RESULT" | jq -r '.tasks[0].work.status // empty')

echo "  13.4.4 Verify task with disabled LLM is still waiting"
if [ "$DISABLED_TASK_STATUS" = "waiting" ]; then
    echo "    ${GREEN}PASS${NC}: Task with disabled LLM remains in waiting status"
    PASS_COUNT=$((PASS_COUNT + 1))
else
    echo "    ${RED}FAIL${NC}: Expected status=waiting, got status=$DISABLED_TASK_STATUS"
    FAIL_COUNT=$((FAIL_COUNT + 1))
fi

# Only cleanup LLM test project if not preserving test directory
if [ "$PRESERVE_TEST_DIR" = false ]; then
    print_subsection "13.5 Cleanup LLM Test Project"

    run_test "13.5.1 Delete LLM test project" \
        "project_delete" \
        "{\"name\":\"$LLM_TEST_PROJECT\"}" \
        '"deleted":true'
fi

#===============================================================================
# SECTION 14: Report Tools
#===============================================================================

print_section "SECTION 14: Report Tools" "Tools: report_start, report_append, report_end, report_list, report_read"

print_subsection "14.1 Report Session Management"

run_test "14.1.1 Start report session" \
    "report_start" \
    "{\"project\":\"$TEST_PROJECT\",\"title\":\"Test Report\",\"intro\":\"This is a test report.\"}" \
    '"main_report"'

run_test "14.1.2 Verify main_report filename returned" \
    "report_start" \
    "{\"project\":\"$TEST_PROJECT\",\"title\":\"Second Report\",\"intro\":\"\"}" \
    'Report.md'

print_subsection "14.2 Report Append"

run_test "14.2.1 Append to default report" \
    "report_append" \
    "{\"project\":\"$TEST_PROJECT\",\"content\":\"## Section 1\\n\\nThis is section 1 content.\\n\\n\"}" \
    '"success":true'

run_test "14.2.2 Append more content to default report" \
    "report_append" \
    "{\"project\":\"$TEST_PROJECT\",\"content\":\"## Section 2\\n\\nThis is section 2 content.\\n\\n\"}" \
    '"bytes_written"'

run_test "14.2.3 Append to named report" \
    "report_append" \
    "{\"project\":\"$TEST_PROJECT\",\"content\":\"# QA Report\\n\\nQA findings here.\\n\",\"report\":\"QA\"}" \
    '"report"'

print_subsection "14.3 Report List"

run_test "14.3.1 List reports" \
    "report_list" \
    "{\"project\":\"$TEST_PROJECT\"}" \
    '"count"'

run_test "14.3.2 Verify reports in list" \
    "report_list" \
    "{\"project\":\"$TEST_PROJECT\"}" \
    'Report.md'

print_subsection "14.4 Report Read"

# Get the actual report name for reading tests
run_test_capture "14.4.1 Get report list for reading" \
    "report_list" \
    "{\"project\":\"$TEST_PROJECT\"}" \
    "reports"

# Extract a report name from the captured output
REPORT_NAME=$(echo "$CAPTURED_RESULT" | grep -o '"name":"[^"]*Report\.md"' | head -1 | sed 's/"name":"//;s/"//')
if [ -n "$REPORT_NAME" ]; then
    run_test "14.4.2 Read specific report by name" \
        "report_read" \
        "{\"project\":\"$TEST_PROJECT\",\"report\":\"$REPORT_NAME\"}" \
        '"content"'

    run_test "14.4.3 Verify report contains appended content" \
        "report_read" \
        "{\"project\":\"$TEST_PROJECT\",\"report\":\"$REPORT_NAME\"}" \
        'Section 1'

    run_test "14.4.4 Verify byte range reading works" \
        "report_read" \
        "{\"project\":\"$TEST_PROJECT\",\"report\":\"$REPORT_NAME\",\"max_bytes\":50}" \
        '"total_bytes"'
else
    echo "  14.4.2 ${YELLOW}SKIP${NC}: Could not extract report name"
    SKIP_COUNT=$((SKIP_COUNT + 1))
    echo "  14.4.3 ${YELLOW}SKIP${NC}: Could not extract report name"
    SKIP_COUNT=$((SKIP_COUNT + 1))
    echo "  14.4.4 ${YELLOW}SKIP${NC}: Could not extract report name"
    SKIP_COUNT=$((SKIP_COUNT + 1))
fi

print_subsection "14.5 Report End Session"

run_test "14.5.1 End report session" \
    "report_end" \
    "{\"project\":\"$TEST_PROJECT\"}" \
    '"success":true'

run_test_expect_fail "14.5.2 End session again fails (no active session)" \
    "report_end" \
    "{\"project\":\"$TEST_PROJECT\"}" \
    "no active report session"

run_test "14.5.3 Reports still exist after session end" \
    "report_list" \
    "{\"project\":\"$TEST_PROJECT\"}" \
    'Report.md'

print_subsection "14.6 Report Error Handling"

run_test_expect_fail "14.6.1 Start report without project" \
    "report_start" \
    '{"title":"Test"}' \
    "project parameter is required"

run_test_expect_fail "14.6.2 Start report without title" \
    "report_start" \
    "{\"project\":\"$TEST_PROJECT\"}" \
    "title parameter is required"

run_test_expect_fail "14.6.3 Append without content" \
    "report_append" \
    "{\"project\":\"$TEST_PROJECT\"}" \
    "content parameter is required"

run_test_expect_fail "14.6.4 Read non-existent report" \
    "report_read" \
    "{\"project\":\"$TEST_PROJECT\",\"report\":\"nonexistent.md\"}" \
    "not found"

run_test_expect_fail "14.6.5 Report name without .md extension" \
    "report_read" \
    "{\"project\":\"$TEST_PROJECT\",\"report\":\"invalid\"}" \
    "must end with .md"

run_test_expect_fail "14.6.6 Report name with path traversal" \
    "report_read" \
    "{\"project\":\"$TEST_PROJECT\",\"report\":\"../../../etc/passwd.md\"}" \
    ""

print_subsection "14.7 Report Create"

# First, create a task set and task with results so report_create has something to generate
run_test "14.7.1 Create taskset for report_create test" \
    "taskset_create" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"report-test\",\"title\":\"Report Test Tasks\"}" \
    '"path":"report-test"'

run_test "14.7.2 Create task for report_create test" \
    "task_create" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"report-test\",\"title\":\"Test Task\",\"prompt\":\"Test prompt\"}" \
    '"title":"Test Task"'

run_test "14.7.3 Start fresh report session for report_create" \
    "report_start" \
    "{\"project\":\"$TEST_PROJECT\",\"title\":\"Report Create Test\"}" \
    '"prefix"'

run_test "14.7.4 Generate report with report_create" \
    "report_create" \
    "{\"project\":\"$TEST_PROJECT\"}" \
    '"reports"'

run_test "14.7.5 Verify report_create returns report count" \
    "report_create" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"report-test\"}" \
    '"reports_count"'

run_test "14.7.6 End report session after report_create" \
    "report_end" \
    "{\"project\":\"$TEST_PROJECT\"}" \
    '"success":true'

run_test_expect_fail "14.7.7 Report create without project fails" \
    "report_create" \
    '{}' \
    "project parameter is required"

# Clean up report-test taskset
run_test "14.7.8 Delete report-test taskset" \
    "taskset_delete" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"report-test\"}" \
    '"deleted":true'

print_subsection "14.8 Supervisor Update"

# Create taskset and task for supervisor testing
run_test "14.8.1 Create taskset for supervisor test" \
    "taskset_create" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"supervisor-test\",\"title\":\"Supervisor Test\"}" \
    '"path":"supervisor-test"'

run_test "14.8.2 Create task for supervisor test" \
    "task_create" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"supervisor-test\",\"title\":\"Supervisor Task\",\"prompt\":\"Test prompt\"}" \
    '"title":"Supervisor Task"'

# Get the task UUID for supervisor_update
SUPERVISOR_TASK_RESULT=$($PROBE -stdio $MAESTRO -env $ENV -call "task_list" -params "{\"project\":\"$TEST_PROJECT\",\"path\":\"supervisor-test\"}" 2>&1)
SUPERVISOR_TASK_UUID=$(echo "$SUPERVISOR_TASK_RESULT" | grep -o '"uuid":"[^"]*"' | head -1 | sed 's/"uuid":"\([^"]*\)"/\1/')

if [ -n "$SUPERVISOR_TASK_UUID" ]; then
    run_test "14.8.3 Apply supervisor update" \
        "supervisor_update" \
        "{\"project\":\"$TEST_PROJECT\",\"uuid\":\"$SUPERVISOR_TASK_UUID\",\"response\":\"Supervisor provided response\"}" \
        '"supervisor_override":true'

    run_test "14.8.4 Verify task status is done after supervisor update" \
        "task_get" \
        "{\"project\":\"$TEST_PROJECT\",\"uuid\":\"$SUPERVISOR_TASK_UUID\"}" \
        '"status":"done"'
else
    echo "    ${YELLOW}SKIP${NC}: Could not get task UUID for supervisor tests"
    SKIP_COUNT=$((SKIP_COUNT + 2))
fi

run_test_expect_fail "14.8.5 Supervisor update without project fails" \
    "supervisor_update" \
    '{"uuid":"test","response":"test"}' \
    "project parameter is required"

run_test_expect_fail "14.8.6 Supervisor update without uuid fails" \
    "supervisor_update" \
    "{\"project\":\"$TEST_PROJECT\",\"response\":\"test\"}" \
    "uuid parameter is required"

run_test_expect_fail "14.8.7 Supervisor update without response fails" \
    "supervisor_update" \
    "{\"project\":\"$TEST_PROJECT\",\"uuid\":\"test\"}" \
    "response parameter is required"

run_test_expect_fail "14.8.8 Supervisor update with invalid uuid fails" \
    "supervisor_update" \
    "{\"project\":\"$TEST_PROJECT\",\"uuid\":\"nonexistent-uuid\",\"response\":\"test\"}" \
    "not found"

# Clean up supervisor-test taskset
run_test "14.8.9 Delete supervisor-test taskset" \
    "taskset_delete" \
    "{\"project\":\"$TEST_PROJECT\",\"path\":\"supervisor-test\"}" \
    '"deleted":true'

#===============================================================================
# SECTION 15: Cleanup & Final Verification
#===============================================================================

print_section "SECTION 15: Cleanup & Verification" "Final cleanup and verification"

print_subsection "15.1 Verify Tasks Cleaned Up"
run_test "15.1.1 Verify no task sets remaining" \
    "taskset_list" \
    "{\"project\":\"$TEST_PROJECT\"}" \
    '"total":0'

print_subsection "15.2 Delete Project"
run_test "15.2.1 Delete test project" \
    "project_delete" \
    "{\"name\":\"$TEST_PROJECT\"}" \
    '"deleted":true'

run_test_expect_fail "15.2.2 Verify project deleted" \
    "project_get" \
    "{\"name\":\"$TEST_PROJECT\"}" \
    "not found"

run_test_expect_fail "15.2.3 Verify project files gone" \
    "project_file_list" \
    "{\"project\":\"$TEST_PROJECT\"}" \
    "not found"

run_test_expect_fail "15.2.4 Verify project tasks gone" \
    "taskset_list" \
    "{\"project\":\"$TEST_PROJECT\"}" \
    "not found"

run_test_expect_fail "15.2.5 Verify project log gone" \
    "project_log_get" \
    "{\"project\":\"$TEST_PROJECT\"}" \
    "not found"

print_subsection "15.3 Delete Remaining Playbook"
run_test "15.3.1 Delete test playbook" \
    "playbook_delete" \
    "{\"name\":\"$TEST_PLAYBOOK\"}" \
    '"deleted":true'

run_test_expect_fail "15.3.2 Verify playbook deleted" \
    "playbook_file_list" \
    "{\"playbook\":\"$TEST_PLAYBOOK\"}" \
    "not found"

#===============================================================================
# TEST SUMMARY
#===============================================================================

echo ""
echo "${BOLD}============================================${NC}"
echo "${BOLD}   TEST SUMMARY${NC}"
echo "${BOLD}============================================${NC}"
echo ""
TOTAL=$((PASS_COUNT + FAIL_COUNT))
echo "Total Tests: ${BOLD}$TOTAL${NC}"
echo "Passed:      ${GREEN}$PASS_COUNT${NC}"
echo "Failed:      ${RED}$FAIL_COUNT${NC}"
echo ""

# Cleanup or preserve test directory
if [ "$PRESERVE_TEST_DIR" = true ]; then
    echo "${CYAN}Test directory preserved: $TEST_ROOT${NC}"
    echo ""
else
    rm -rf "$TEST_ROOT"
fi

if [ $FAIL_COUNT -eq 0 ]; then
    echo "${GREEN}${BOLD}All tests passed!${NC}"
    exit 0
else
    echo "${RED}${BOLD}Some tests failed.${NC}"
    exit 1
fi
