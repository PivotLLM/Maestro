#!/bin/sh
################################################################################
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                           #
# Please see the LICENSE file for details                                      #
################################################################################

# Detect script directory
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# PROBE can be overridden via environment variable
: "${PROBE:=probe}"

MAESTRO="$SCRIPT_DIR/maestro"

# Search for probe in PATH if not an absolute path
if [ "${PROBE#/}" = "$PROBE" ]; then
    PROBE_FULL=$(command -v "$PROBE" 2>/dev/null)
    if [ -z "$PROBE_FULL" ]; then
        echo "ERROR: probe binary '$PROBE' not found in PATH" >&2
        echo "MCPProbe can be obtained from: https://github.com/PivotLLM/MCPProbe" >&2
        echo "Please install probe or set PROBE environment variable to the full path" >&2
        exit 1
    fi
    PROBE="$PROBE_FULL"
fi

$PROBE -stdio $MAESTRO -list
