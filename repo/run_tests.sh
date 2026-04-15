#!/bin/bash
#
# run_tests.sh — Run the full CampusRec test suite in Docker.
#
# Works from a clean Linux environment with Docker available.
# Always cleans up containers/volumes, preserves real exit code,
# and prints a clear pass/fail summary.

# Do NOT use set -e: we need to capture the test exit code and still run cleanup.
set -uo pipefail

COMPOSE_FILE="docker-compose.test.yml"

echo "=== CampusRec Test Runner ==="
echo ""

# ---------------------------------------------------------------------------
# Cleanup function — always runs
# ---------------------------------------------------------------------------
cleanup() {
    echo ""
    echo "Cleaning up test containers..."
    docker compose -f "$COMPOSE_FILE" down -v --remove-orphans 2>/dev/null || true
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# Teardown any leftover state from a previous run
# ---------------------------------------------------------------------------
docker compose -f "$COMPOSE_FILE" down -v --remove-orphans 2>/dev/null || true

# ---------------------------------------------------------------------------
# Build and run tests
# ---------------------------------------------------------------------------
echo "Building and running tests in Docker..."
echo ""

docker compose -f "$COMPOSE_FILE" up \
    --build \
    --abort-on-container-exit \
    --exit-code-from test-runner

TEST_EXIT=$?

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo "==========================================="
if [ "$TEST_EXIT" -eq 0 ]; then
    echo "  ALL TESTS PASSED"
else
    echo "  TESTS FAILED  (exit code: $TEST_EXIT)"
fi
echo "==========================================="

exit "$TEST_EXIT"
