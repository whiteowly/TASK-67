#!/bin/bash
#
# run_e2e.sh — Run Playwright browser E2E tests in Docker.
#
# Starts the full app stack, then runs Playwright against it.
# Screenshots are written to e2e/screenshots/.

set -uo pipefail

COMPOSE_FILE="docker-compose.e2e.yml"

echo "=== CampusRec E2E Browser Tests ==="
echo ""

cleanup() {
    echo ""
    echo "Cleaning up E2E containers..."
    docker compose -f "$COMPOSE_FILE" down -v --remove-orphans 2>/dev/null || true
}
trap cleanup EXIT

docker compose -f "$COMPOSE_FILE" down -v --remove-orphans 2>/dev/null || true

# Clear previous screenshots
rm -f e2e/screenshots/*.png 2>/dev/null || true

echo "Building and starting app stack..."
docker compose -f "$COMPOSE_FILE" up --build --abort-on-container-exit --exit-code-from e2e

E2E_EXIT=$?

# Defensive check: if the app container died before e2e even ran, the
# --exit-code-from e2e shortcut can report 0 (e2e never started). Detect
# that specifically — but ignore 137/143 (SIGKILL/SIGTERM during normal
# teardown after e2e finished).
APP_EXIT=$(docker compose -f "$COMPOSE_FILE" ps -a --format '{{.Service}} {{.ExitCode}}' 2>/dev/null \
            | awk '$1=="app"{print $2}' | head -1)
case "${APP_EXIT:-0}" in
    0|137|143) ;;  # 0 = clean; 137 = SIGKILL on teardown; 143 = SIGTERM on teardown
    *)
        if [ "$E2E_EXIT" -eq 0 ]; then
            echo "WARNING: app container exited with code $APP_EXIT (not a teardown signal)."
            echo "         Marking the run as failed even though e2e reported 0."
            E2E_EXIT="$APP_EXIT"
        fi
        ;;
esac

echo ""
echo "==========================================="
if [ "$E2E_EXIT" -eq 0 ]; then
    echo "  ALL E2E TESTS PASSED"
    echo ""
    echo "  Screenshots: e2e/screenshots/"
    ls -1 e2e/screenshots/*.png 2>/dev/null | while read f; do echo "    $f"; done
else
    echo "  E2E TESTS FAILED  (exit code: $E2E_EXIT)"
fi
echo "==========================================="

exit "$E2E_EXIT"
