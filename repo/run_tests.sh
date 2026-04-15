#!/bin/bash
#
# run_tests.sh — Run the full CampusRec test suite in Docker.
#
# Works from a clean Linux environment with Docker available.
# Always cleans up containers/volumes, preserves real exit code,
# and prints a clear pass/fail summary.

# Do NOT use set -e: we need to capture the test exit code and still run cleanup.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.test.yml"
FALLBACK_DOCKERFILE="$SCRIPT_DIR/.Dockerfile.test.fallback"

if ! command -v docker >/dev/null 2>&1; then
    echo "Error: docker command not found in PATH"
    exit 1
fi

if docker compose version >/dev/null 2>&1; then
    COMPOSE_CMD=(docker compose)
elif command -v docker-compose >/dev/null 2>&1; then
    COMPOSE_CMD=(docker-compose)
else
    echo "Error: neither 'docker compose' nor 'docker-compose' is available"
    exit 1
fi

if ! docker info >/dev/null 2>&1; then
    echo "Error: cannot connect to Docker daemon"
    exit 1
fi

if [ ! -f "$COMPOSE_FILE" ]; then
    echo "Error: compose file not found: $COMPOSE_FILE"
    exit 1
fi

if [ -f "$SCRIPT_DIR/Dockerfile.test" ]; then
    TEST_DOCKERFILE="Dockerfile.test"
elif [ -f "$SCRIPT_DIR/Dockerfile" ]; then
    TEST_DOCKERFILE=".Dockerfile.test.fallback"
    if ! cat > "$FALLBACK_DOCKERFILE" <<'EOF'
FROM golang:1.25-bookworm

RUN go install github.com/a-h/templ/cmd/templ@v0.3.1001

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN templ generate

CMD ["go", "test", "./...", "-v", "-count=1", "-p=1"]
EOF
    then
        echo "Error: failed to write fallback Dockerfile at $FALLBACK_DOCKERFILE"
        exit 1
    fi
    echo "Warning: Dockerfile.test not found, using generated fallback test Dockerfile."
else
    echo "Error: neither Dockerfile.test nor Dockerfile exists in $SCRIPT_DIR"
    exit 1
fi
export TEST_DOCKERFILE

compose() {
    "${COMPOSE_CMD[@]}" --project-directory "$SCRIPT_DIR" -f "$COMPOSE_FILE" "$@"
}

echo "=== CampusRec Test Runner ==="
echo ""

# ---------------------------------------------------------------------------
# Cleanup function — always runs
# ---------------------------------------------------------------------------
cleanup() {
    echo ""
    echo "Cleaning up test containers..."
    compose down -v --remove-orphans 2>/dev/null || true
    rm -f "$FALLBACK_DOCKERFILE"
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# Teardown any leftover state from a previous run
# ---------------------------------------------------------------------------
compose down -v --remove-orphans 2>/dev/null || true

# ---------------------------------------------------------------------------
# Build and run tests
# ---------------------------------------------------------------------------
echo "Building and running tests in Docker..."
echo ""

compose up \
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
