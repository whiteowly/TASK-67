#!/bin/bash
#
# init_db.sh — Canonical database bootstrap for CampusRec.
#
# Idempotent: safe to run repeatedly. Creates the database if it does not
# exist, runs all pending migrations, and seeds sample data.
#
# Usage:
#   ./init_db.sh                       # uses Compose DB defaults
#   DATABASE_URL=... ./init_db.sh      # explicit connection string
#
# Prerequisites: docker must be running, or a reachable PostgreSQL instance.

set -euo pipefail

# ---------------------------------------------------------------------------
# Resolve database connection
# ---------------------------------------------------------------------------
# If DATABASE_URL is set, use it directly (for non-Docker local dev).
# Otherwise, target the Compose-managed PostgreSQL service.
if [ -z "${DATABASE_URL:-}" ]; then
    # Ensure the Compose db service is running
    if ! docker compose ps --status running 2>/dev/null | grep -q "db"; then
        echo "Starting database service..."
        docker compose up -d db
        echo "Waiting for database to become healthy..."
        for i in $(seq 1 30); do
            if docker compose exec -T db pg_isready -U campusrec -q 2>/dev/null; then
                break
            fi
            sleep 1
        done
    fi
    DATABASE_URL="postgres://campusrec:campusrec@localhost:5432/campusrec?sslmode=disable"
    USING_COMPOSE_DB=1
else
    USING_COMPOSE_DB=0
fi

export DATABASE_URL
export SESSION_SECRET="${SESSION_SECRET:-init-db-bootstrap-secret-not-for-production}"

echo "=== CampusRec Database Init ==="
echo "Target: ${DATABASE_URL%%@*}@****"

# ---------------------------------------------------------------------------
# If using Compose, run migrations and seed inside the app container
# (avoids requiring Go toolchain on the host).
# ---------------------------------------------------------------------------
if [ "${USING_COMPOSE_DB}" = "1" ]; then
    echo ""
    echo "Building app image (if needed)..."
    docker compose build app --quiet 2>/dev/null || docker compose build app

    echo "Running migrations..."
    docker compose run --rm -e DATABASE_URL -e SESSION_SECRET \
        app sh -c "/app/campusrec -migrate up"

    echo ""
    echo "Seeding data..."
    docker compose run --rm -e DATABASE_URL -e SESSION_SECRET \
        app sh -c "/app/campusrec -seed"
else
    # Non-Docker path: requires Go toolchain
    echo ""
    echo "Running migrations (local Go)..."
    go run ./cmd/server -migrate up

    echo ""
    echo "Seeding data..."
    go run ./cmd/server -seed
fi

echo ""
echo "=== Database initialized successfully ==="
