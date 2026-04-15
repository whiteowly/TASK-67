#!/bin/bash
#
# run_all_tests.sh — Run every test suite in sequence with one command.
#
# This is the default release-confidence command. It runs, in order:
#   1. ./run_tests.sh                (Go unit + integration in Docker)
#   2. ./run_external_api_tests.sh   (external HTTP API suite, 79/79 endpoints)
#   3. ./run_e2e.sh                  (Playwright browser E2E)
#
# Behavior:
#   - Fail-fast: aborts on the first non-zero exit.
#   - Never proceeds to the next suite if a prior suite failed.
#   - Prints a final combined summary with per-suite status.
#   - Always exits with the failing suite's exit code (or 0 if all passed).

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Per-suite status tracking
declare -a SUITE_NAMES=()
declare -a SUITE_STATUSES=()
declare -a SUITE_DURATIONS=()

OVERALL_START=$(date +%s)

run_suite() {
    local name="$1"
    local script="$2"

    echo ""
    echo "############################################"
    echo "###  RUN: $name"
    echo "###  cmd: $script"
    echo "############################################"
    echo ""

    local start
    start=$(date +%s)

    "$script"
    local rc=$?

    local end
    end=$(date +%s)
    local dur=$((end - start))

    SUITE_NAMES+=("$name")
    SUITE_DURATIONS+=("$dur")

    if [ "$rc" -ne 0 ]; then
        SUITE_STATUSES+=("FAIL ($rc)")
        print_summary
        echo ""
        echo "ABORTING: $name failed with exit code $rc."
        echo "Subsequent suites were NOT run."
        exit "$rc"
    fi

    SUITE_STATUSES+=("PASS")
}

print_summary() {
    local end
    end=$(date +%s)
    local total=$((end - OVERALL_START))

    echo ""
    echo "============================================"
    echo "  CampusRec — Combined Test Suite Summary"
    echo "============================================"
    local i
    for i in "${!SUITE_NAMES[@]}"; do
        printf "  %-32s %-12s  %4ds\n" \
            "${SUITE_NAMES[$i]}" \
            "${SUITE_STATUSES[$i]}" \
            "${SUITE_DURATIONS[$i]}"
    done
    echo "  --------------------------------------------"
    printf "  %-32s %-12s  %4ds\n" "TOTAL" "" "$total"
    echo "============================================"
}

echo "=== CampusRec — Run All Tests ==="
echo "Order: drift guard → unit/integration → external API → browser E2E"
echo "Fail-fast: enabled"

run_suite "API coverage drift guard"   "$SCRIPT_DIR/scripts/check_api_coverage_drift.sh"
run_suite "Go unit + integration"      "$SCRIPT_DIR/run_tests.sh"
run_suite "External API (79 endpoints)" "$SCRIPT_DIR/run_external_api_tests.sh"
run_suite "Playwright browser E2E"      "$SCRIPT_DIR/run_e2e.sh"

print_summary
echo ""
echo "ALL SUITES PASSED."
exit 0
