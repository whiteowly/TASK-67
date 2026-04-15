#!/bin/bash
#
# check_api_coverage_drift.sh — Static guard against documentation drift.
#
# Verifies three sources agree on the API endpoint set:
#   1. internal/router/router.go             (the canonical source)
#   2. docs/api-endpoints-inventory.md       (must list every router endpoint)
#   3. docs/api-coverage-after.md            (must map every router endpoint
#                                             to at least one external test)
#
# Exits non-zero if any source disagrees. Designed to run in CI before the
# test execution stage so a missed endpoint is caught at the docs layer
# rather than as a silent gap in coverage evidence.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

if ! command -v python3 >/dev/null 2>&1; then
    echo "ERROR: python3 not found in PATH (required for endpoint extraction)"
    exit 2
fi

exec python3 "$SCRIPT_DIR/check_api_coverage_drift.py" "$ROOT"
