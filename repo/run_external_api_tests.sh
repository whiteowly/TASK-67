#!/bin/bash
#
# run_external_api_tests.sh — Run the external API test suite end-to-end.
#
# Starts the full app + DB in Docker, then executes
# `go test ./tests/external_api/...` inside a sibling container against
# the live HTTP server via EXTERNAL_API_BASE_URL. Covers all 79 backend
# endpoints at the external HTTP boundary. See docs/api-coverage-after.md.
#
# Deterministic teardown: always `docker compose down -v --remove-orphans`
# on exit so each run starts from a clean DB.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.external-api.yml"
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

CMD ["go", "test", "./tests/external_api/...", "-v", "-count=1", "-p=1"]
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

echo "=== CampusRec External API Test Runner ==="
echo ""
echo "This launches a real Docker-hosted app instance and runs the"
echo "tests/external_api/ suite against it via real HTTP (EXTERNAL_API_BASE_URL)."
echo ""

cleanup() {
    echo ""
    echo "Cleaning up containers and volumes..."
    compose down -v --remove-orphans 2>/dev/null || true
    rm -f "$FALLBACK_DOCKERFILE"
}
trap cleanup EXIT

# Teardown any leftover state from a previous run.
compose down -v --remove-orphans 2>/dev/null || true

echo "Building and starting app + db + tests..."
echo ""

compose up \
    --build \
    --abort-on-container-exit \
    --exit-code-from tests

TEST_EXIT=$?

echo ""
echo "==========================================="
if [ "$TEST_EXIT" -eq 0 ]; then
    echo "  EXTERNAL API TESTS PASSED"
else
    echo "  EXTERNAL API TESTS FAILED  (exit code: $TEST_EXIT)"
fi
echo "==========================================="

exit "$TEST_EXIT"
