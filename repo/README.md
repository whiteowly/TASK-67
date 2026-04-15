# CampusRec

A fullstack private-network operational platform for recreation centers, wellness facilities, and training studios. 

## Architecture & Tech Stack

* **Frontend:** Server-rendered Templ pages (Go) + vanilla CSS/JS (`web/templates`, `web/static`)
* **Backend:** Go 1.25 + Gin (`internal/handler`, `internal/service`, `internal/router`)
* **Database:** PostgreSQL 16 (`db/migrations`, accessed via `pgx/v5`)
* **Auth:** Local username/password with server-side sessions; RBAC across Member / Staff / Moderator / Administrator
* **Containerization:** Docker & Docker Compose (Required)

## Project Structure

```text
.
├── cmd/server/                    # Application entrypoint (Go binary)
├── config/                        # Configuration loader (env vars + defaults)
├── internal/
│   ├── handler/api/               # JSON API handlers
│   ├── handler/web/               # Templ page handlers
│   ├── middleware/                # Auth, RBAC, audit, recovery
│   ├── model/                     # Domain models
│   ├── repo/                      # Data access (pgx)
│   ├── response/                  # API response envelope
│   ├── router/                    # Route wiring (canonical endpoint inventory)
│   ├── service/                   # Business logic
│   ├── util/                      # Password hashing, pagination, timezone
│   └── validator/                 # Input validation
├── web/templates/                 # Templ templates
├── web/static/                    # CSS, JS
├── db/migrations/                 # PostgreSQL migrations
├── tests/
│   ├── external_api/              # External HTTP API tests (real TCP, no mocks)
│   ├── blackbox/                  # Black-box HTTP behavior tests
│   ├── integration/               # In-process integration tests (informational only)
│   └── testutil/                  # Shared test helpers
├── e2e/                           # Playwright browser E2E tests
├── scripts/                       # CI guard scripts (drift check, etc.)
├── docs/                          # Coverage / testing / audit-remediation docs
├── docker-compose.yml             # App + DB orchestration (MANDATORY)
├── docker-compose.test.yml        # Go unit + integration test runner
├── docker-compose.external-api.yml # External API suite runner
├── docker-compose.e2e.yml         # Playwright E2E runner
├── Dockerfile                     # Production image
├── Dockerfile.test                # Test image (Go toolchain + templ)
├── run_tests.sh                   # Standardized test execution script (MANDATORY)
├── run_external_api_tests.sh      # External API suite
├── run_e2e.sh                     # Browser E2E suite
├── run_all_tests.sh               # Top-level orchestrator (primary release-gate command)
└── README.md                      # Project documentation (MANDATORY)
```

> **No `.env.example` is required.** The supported run path injects every needed environment variable directly via `docker-compose.yml` (DB credentials, session secret, payment merchant key, etc.). There are no host-side env vars to set.

## Prerequisites

To ensure a consistent environment, this project is designed to run entirely within containers. You must have the following installed:

* [Docker](https://docs.docker.com/get-docker/)
* [Docker Compose](https://docs.docker.com/compose/install/)

No host-side Go, Node, or PostgreSQL installation is required to run, smoke-test, or test the application.

## Running the Application

1. **Build and Start Containers:**
   Use Docker Compose to build the images and spin up the entire stack.

   ```bash
   docker-compose up
   ```

   Or, equivalently, in detached mode with an explicit rebuild:

   ```bash
   docker-compose up --build -d
   ```

   Both forms launch the same stack defined in `docker-compose.yml` (`db` + `app`). On startup the app container automatically:
   * waits for PostgreSQL to be healthy,
   * runs all schema migrations,
   * seeds demo users / sessions / products,
   * starts the HTTP server on port 8080.

2. **Smoke check (verify the stack is up):**
   Three deterministic checks with exact expected outputs.

   ```bash
   # 1. Health endpoint must return 200 with {"status":"ok"}
   curl -fsS http://localhost:8080/health

   # 2. Public catalog must return at least one seeded session
   curl -fsS http://localhost:8080/api/v1/catalog/sessions | head -c 80
   #    expected: {"success":true,"data":[ ...

   # 3. Demo admin must be able to log in
   curl -fsS -X POST http://localhost:8080/api/v1/auth/login \
     -H 'Content-Type: application/json' \
     -d '{"username":"admin","password":"Seed@Pass1234"}'
   #    expected: HTTP 200 + {"success":true, ...}
   ```

   If any of these three fail, the stack did not come up cleanly — inspect with `docker-compose logs app db`.

3. **Access the App:**
   * Frontend (Templ-rendered web UI): `http://localhost:8080/`
   * Backend API: `http://localhost:8080/api/v1`
   * Health endpoint: `http://localhost:8080/health`
   * Admin dashboard (after admin login): `http://localhost:8080/admin`

   The full per-route list lives in `docs/api-endpoints-inventory.md` (79 endpoints, all externally tested).

4. **Stop the Application:**

   ```bash
   docker-compose down -v
   ```

## Testing

All unit, integration, external-API, and E2E tests are executed via a single, standardized shell script. The script automatically handles container orchestration for the test environment (separate compose files per suite, all in Docker).

Make sure the script is executable, then run it:

```bash
chmod +x run_all_tests.sh
./run_all_tests.sh
```

`run_all_tests.sh` runs, in order, with fail-fast and a combined per-suite summary:

1. `scripts/check_api_coverage_drift.sh` — static guard that confirms the router, the inventory doc, and the after-coverage mapping all describe the same 79 endpoints.
2. `./run_tests.sh` — Go unit + integration tests inside Docker (the standardized per-template runner — also runnable on its own).
3. `./run_external_api_tests.sh` — external HTTP API suite against a Docker-hosted app instance (covers all 79 endpoints over real TCP, no HTTP-layer mocks).
4. `./run_e2e.sh` — Playwright browser E2E (page flows + 4 fullstack journeys).

Each per-suite runner is also available individually for fast iteration.

*Note: `run_all_tests.sh` (and every per-suite runner) outputs a standard exit code (`0` for success, non-zero for failure) to integrate smoothly with CI/CD validators.*

For per-suite descriptions and the per-endpoint test mapping, see `docs/testing.md`, `docs/api-coverage-after.md`, and `docs/audit_remediation.md`.

## Seeded Credentials

The database is pre-seeded with the following test users on startup. Use these credentials to verify authentication and role-based access controls. **Login uses `username`, not email.**

| Role | Username | Password | Notes |
| :--- | :--- | :--- | :--- |
| **Administrator** | `admin` | `Seed@Pass1234` | Full access — system config, feature flags, audit logs, backups, imports/exports, KPIs, refund reconciliation. |
| **Staff** | `staff1` | `Seed@Pass1234` | Attendance check-in, shipment lifecycle, ticket assignment, registration approve/reject. |
| **Moderator** | `mod1` | `Seed@Pass1234` | Moderation reports, cases, actions, bans. |
| **Member** | `member1` | `Seed@Pass1234` | Standard end-user — register for sessions, cart, checkout, orders, posts, tickets. |
| **Member** | `member2` | `Seed@Pass1234` | Second member — useful for cross-user isolation tests. |

---

## Optional: Contributor / non-Docker workflow

The Docker stack above is the **only** supported way to run the application. A non-Docker contributor workflow exists for offline hacking and is documented separately to keep this README focused on the Docker path:

* See [`docs/local_development.md`](docs/local_development.md).

Do not use that path for evaluation, smoke testing, or CI — it is not part of the supported run/verify flow.
