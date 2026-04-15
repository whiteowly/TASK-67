# Testing Guide

The repo has three distinct Go test suites plus a Playwright E2E suite.
Each one has a dedicated single-command runner.

| Suite | Path | What it proves | Runner | Counts toward API coverage? |
|---|---|---|---|---|
| **External API** | `tests/external_api/` | All 79 endpoints reachable over real HTTP against a Dockerized app, plus deeper behavior tests for high-risk domains | `./run_external_api_tests.sh` | **Yes** |
| Black-box HTTP | `tests/blackbox/` | Business flows exercised over real HTTP via `httptest.NewServer` | `./run_tests.sh` | Yes (behavioral depth) |
| In-process integration | `tests/integration/` | Legacy assertions via `r.ServeHTTP`; kept but informational only | `./run_tests.sh` | No (per task requirement) |
| Service / middleware unit | `internal/service/`, `internal/middleware/`, `internal/util/`, `internal/validator/` | Pure decision logic in isolation (RBAC, payment HMAC, feature-flag cohort, password hashing, validators) | `./run_tests.sh` | No — complements behavioral suites |
| Browser E2E | `e2e/` | Playwright against rendered HTML, including 4 fullstack journeys (member checkout-to-payment, staff shipment, moderation report, admin import) | `./run_e2e.sh` | n/a — covers HTML pages |

## One-command release confidence

```bash
./run_all_tests.sh
```

Runs, fail-fast, in this order:

1. `scripts/check_api_coverage_drift.sh` — static guard that confirms
   `internal/router/router.go`, `docs/api-endpoints-inventory.md`, and
   `docs/api-coverage-after.md` all describe the same endpoint set. CI
   fails before any test runs if a new route is added without
   inventory + after-coverage updates.
2. `./run_tests.sh`
3. `./run_external_api_tests.sh`
4. `./run_e2e.sh`

## External API suite (CI primary)

```bash
./run_external_api_tests.sh
```

This is the command the merge gate runs. It:

1. Boots PostgreSQL 16 in a Docker container (`db` service).
2. Builds and starts the campusrec binary (`app` service) which runs
   migrations + seed and then listens on port 8080. A container-level
   healthcheck gates downstream services on `GET /health`.
3. Starts a `tests` container that waits for `app` to report healthy,
   then runs `go test ./tests/external_api/... -v -count=1 -p=1` with
   `EXTERNAL_API_BASE_URL=http://app:8080` so every request crosses a
   real TCP boundary.
4. Tears down all containers and volumes on exit.

Run time: ~2–3 min cold, ~60 s warm.

## Fast iteration

`go test ./tests/external_api/... -v` will also work against a local
`httptest.Server` when `EXTERNAL_API_BASE_URL` is unset (the suite
auto-detects this via `helpers.go::setupEnv`). This mode requires
`DATABASE_URL` to be set to a Postgres instance.

## What "external" means here

All requests in `tests/external_api/` and `tests/blackbox/` go through
real `http.Client` → TCP → `net/http` server → Gin middleware → handler
→ service → repo → PostgreSQL. There are no HTTP-layer mocks. There is
no `r.ServeHTTP()`. There is no `httptest.NewRecorder()`.

Only the in-process suite under `tests/integration/` uses `ServeHTTP`;
those tests are retained but do **not** count toward the 100 % external
coverage target (per the task requirement).
