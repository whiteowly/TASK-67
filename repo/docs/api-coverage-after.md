# API Coverage — After State

## Result

**External endpoint coverage = 100% (79 / 79).**

Every backend HTTP endpoint listed in `docs/api-endpoints-inventory.md`
is now exercised directly by at least one external HTTP test that runs
against a real Docker-hosted app instance via TCP. No HTTP-layer mocks.
No `r.ServeHTTP`. No in-process recorders.

## How it was achieved

- Existing external tests in `tests/blackbox/blackbox_test.go` already
  covered 67 of the 79 endpoints over real HTTP. Those continue to run.
- A new `tests/external_api/` suite was added with:
  - `coverage_matrix_test.go` — deterministic reachability probe that
    confirms every one of the 79 endpoints accepts a request and
    returns a real HTTP response. This is the merge-gate fence.
  - `missing_endpoints_test.go` — full behavioral assertions (status
    code, response body shape, RBAC, failure-mode) for the 12
    endpoints that previously had no external HTTP coverage.
- One blocking app bug was fixed along the way: `GET /api/v1/admin/audit-logs`
  was returning 500 because pgx could not scan PostgreSQL `INET` into
  `*string`. Cast to text via `host(ip_addr)` in `internal/repo/audit_repo.go`.

## How to run

```bash
./run_external_api_tests.sh
```

Spins up `db` + `app` + `tests` via `docker-compose.external-api.yml`,
runs `go test ./tests/external_api/...` against the live HTTP server
(`EXTERNAL_API_BASE_URL=http://app:8080`), tears down all containers and
volumes on exit.

Suite executes in ~18 s once images are built (~2 min cold).

## Per-endpoint mapping

For each of the 79 endpoints, the column "External test case(s)" lists
direct external HTTP tests. The Coverage Matrix entry alone is
sufficient to prove reachability; the named tests provide deeper
behavioral coverage.

> **Legend**
> - `BB`  = `tests/blackbox/blackbox_test.go::*`
> - `EXT` = `tests/external_api/missing_endpoints_test.go::*`
> - `BHV` = `tests/external_api/behavior_admin_test.go::*` (deeper behavior tests)
> - `MX`  = `tests/external_api/coverage_matrix_test.go::TestExternal_CoverageMatrix/<route>`

| # | Method | Path | External test case(s) |
|---|---|---|---|
| 1 | GET | /health | `EXT::TestExternal_Health`; `MX` |
| 2 | POST | /api/v1/auth/register | `BB::TestBB_Auth_RegisterLoginLogout`; `MX` |
| 3 | POST | /api/v1/auth/login | `BB::TestBB_Auth_RegisterLoginLogout`, `BB::TestBB_Auth_BadCredentials`; `MX` |
| 4 | POST | /api/v1/auth/logout | `BB::TestBB_Auth_RegisterLoginLogout`; `MX` |
| 5 | GET | /api/v1/users/me | `BB::TestBB_Auth_RegisterLoginLogout`, `BB::TestBB_Auth_ProfileUpdate`; `MX` |
| 6 | PATCH | /api/v1/users/me | `BB::TestBB_Auth_ProfileUpdate`; `MX` |
| 7 | GET | /api/v1/catalog/sessions | `BB::TestBB_Catalog`; `MX` |
| 8 | GET | /api/v1/catalog/sessions/:id | `BB::TestBB_Catalog`; `MX` |
| 9 | GET | /api/v1/catalog/products | `BB::TestBB_Catalog`; `MX` |
| 10 | GET | /api/v1/catalog/products/:id | `BB::TestBB_Catalog`; `MX` |
| 11 | GET | /api/v1/addresses | `BB::TestBB_RBAC_Unauthenticated`; `MX` |
| 12 | POST | /api/v1/addresses | `BB::TestBB_Addresses`; `MX` |
| 13 | GET | /api/v1/addresses/:id | `BB::TestBB_Addresses`; `MX` |
| 14 | PATCH | /api/v1/addresses/:id | `BB::TestBB_Addresses`; `MX` |
| 15 | DELETE | /api/v1/addresses/:id | `BB::TestBB_Addresses`; `MX` |
| 16 | GET | /api/v1/admin/config | `BB::TestBB_AdminConfig`, `BB::TestBB_RBAC_Forbidden`, `BHV::TestExternal_AdminConfig_VersionConflict`; `MX` |
| 17 | PATCH | /api/v1/admin/config/:key | `BB::TestBB_AdminConfig`, `BHV::TestExternal_AdminConfig_VersionConflict`; `MX` |
| 18 | GET | /api/v1/admin/feature-flags | `BB::TestBB_FeatureFlags`, `BB::TestBB_RBAC_Forbidden`; `MX` |
| 19 | PATCH | /api/v1/admin/feature-flags/:key | `BB::TestBB_FeatureFlags`; `MX` |
| 20 | GET | /api/v1/admin/audit-logs | `EXT::TestExternal_AdminAuditLogs`, `BHV::TestExternal_AdminAuditLogs_Filtering`; `MX` |
| 21 | POST | /api/v1/admin/backups | `BB::TestBB_AdminOps`, `BHV::TestExternal_AdminBackup_StateTransitions`; `MX` |
| 22 | GET | /api/v1/admin/backups | `BB::TestBB_AdminOps`, `BHV::TestExternal_AdminBackup_StateTransitions`; `MX` |
| 23 | POST | /api/v1/admin/restore | `BB::TestBB_AdminOps`, `BHV::TestExternal_AdminBackup_StateTransitions`; `MX` |
| 24 | GET | /api/v1/admin/archives | `BB::TestBB_AdminOps`; `MX` |
| 25 | POST | /api/v1/admin/archives | `BB::TestBB_AdminOps`; `MX` |
| 26 | POST | /api/v1/admin/refunds/:id/reconcile | `EXT::TestExternal_AdminRefundReconcile`; `MX` |
| 27 | GET | /api/v1/admin/kpis | `BB::TestBB_AdminOps`, `BB::TestBB_RBAC_Forbidden`; `MX` |
| 28 | GET | /api/v1/admin/jobs | `BB::TestBB_AdminOps`; `MX` |
| 29 | POST | /api/v1/admin/registrations/override | `EXT::TestExternal_AdminRegistrationOverride`; `MX` |
| 30 | POST | /api/v1/registrations | `BB::TestBB_Registration_CreateCancelReadback`, `BB::TestBB_Registration_Approve`, `BB::TestBB_Registration_Reject`; `MX` |
| 31 | GET | /api/v1/registrations | `BB::TestBB_Registration_CreateCancelReadback`; `MX` |
| 32 | GET | /api/v1/registrations/:id | `BB::TestBB_Registration_CreateCancelReadback`, `BB::TestBB_Registration_Approve`, `BB::TestBB_Registration_Reject`; `MX` |
| 33 | POST | /api/v1/registrations/:id/cancel | `BB::TestBB_Registration_CreateCancelReadback`; `MX` |
| 34 | POST | /api/v1/registrations/:id/approve | `BB::TestBB_Registration_Approve`; `MX` |
| 35 | POST | /api/v1/registrations/:id/reject | `BB::TestBB_Registration_Reject`; `MX` |
| 36 | POST | /api/v1/attendance/checkin | `BB::TestBB_Attendance`; `MX` |
| 37 | POST | /api/v1/attendance/leave | `BB::TestBB_Attendance`; `MX` |
| 38 | POST | /api/v1/attendance/leave/:id/return | `BB::TestBB_Attendance`; `MX` |
| 39 | GET | /api/v1/attendance/exceptions | `BB::TestBB_Attendance`; `MX` |
| 40 | GET | /api/v1/cart | `BB::TestBB_Commerce_CheckoutAndOrderReadback`, `EXT::TestExternal_CartItemDelete`; `MX` |
| 41 | POST | /api/v1/cart/items | `BB::TestBB_Commerce_CheckoutAndOrderReadback`, `BB::TestBB_Shipments_FullLifecycle`, `EXT::TestExternal_CartItemDelete`, `BHV::TestExternal_Commerce_CheckoutIdempotency`; `MX` |
| 42 | DELETE | /api/v1/cart/items/:id | `EXT::TestExternal_CartItemDelete`; `MX` |
| 43 | POST | /api/v1/checkout | `BB::TestBB_Commerce_CheckoutAndOrderReadback`, `BB::TestBB_Shipments_FullLifecycle`, `BHV::TestExternal_Commerce_CheckoutIdempotency`, `BHV::TestExternal_PaymentCallback_SignatureAndReplay`; `MX` |
| 44 | POST | /api/v1/buy-now | `BB::TestBB_Commerce_BuyNow`; `MX` |
| 45 | GET | /api/v1/orders | `BB::TestBB_Commerce_CheckoutAndOrderReadback`; `MX` |
| 46 | GET | /api/v1/orders/:id | `BB::TestBB_Commerce_CheckoutAndOrderReadback`, `BB::TestBB_Shipments_FullLifecycle`; `MX` |
| 47 | POST | /api/v1/orders/:id/pay | `BB::TestBB_Commerce_CheckoutAndOrderReadback`, `BB::TestBB_Shipments_FullLifecycle`; `MX` |
| 48 | POST | /api/v1/payments/callback | `BB::TestBB_Shipments_FullLifecycle`, `BB::TestBB_PaymentCallback`, `BHV::TestExternal_PaymentCallback_SignatureAndReplay`; `MX` |
| 49 | POST | /api/v1/shipments | `BB::TestBB_Shipments_FullLifecycle`; `MX` |
| 50 | GET | /api/v1/shipments | `BB::TestBB_Shipments_FullLifecycle`, `BB::TestBB_RBAC_Forbidden`; `MX` |
| 51 | PATCH | /api/v1/shipments/:id/status | `BB::TestBB_Shipments_FullLifecycle`; `MX` |
| 52 | POST | /api/v1/shipments/:id/pod | `BB::TestBB_Shipments_FullLifecycle`; `MX` |
| 53 | POST | /api/v1/shipments/:id/exception | `BB::TestBB_Shipments_FullLifecycle`; `MX` |
| 54 | GET | /api/v1/posts | `EXT::TestExternal_ListPosts`; `MX` |
| 55 | GET | /api/v1/posts/:id | `BB::TestBB_Moderation`; `MX` |
| 56 | POST | /api/v1/posts | `BB::TestBB_Moderation`, `EXT::TestExternal_ListPosts`; `MX` |
| 57 | POST | /api/v1/posts/:id/report | `BB::TestBB_Moderation`; `MX` |
| 58 | GET | /api/v1/moderation/reports | `BB::TestBB_Moderation`, `BHV::TestExternal_Moderation_ListContract`; `MX` |
| 59 | GET | /api/v1/moderation/cases | `BB::TestBB_Moderation`, `BB::TestBB_RBAC_Forbidden`, `BHV::TestExternal_Moderation_ListContract`; `MX` |
| 60 | GET | /api/v1/moderation/cases/:id | `EXT::TestExternal_ModerationGetCase`; `MX` |
| 61 | POST | /api/v1/moderation/cases/:id/action | `EXT::TestExternal_ModerationActionCase`; `MX` |
| 62 | POST | /api/v1/moderation/bans | `EXT::TestExternal_ModerationBanAndRevoke`; `MX` |
| 63 | POST | /api/v1/moderation/bans/:id/revoke | `EXT::TestExternal_ModerationBanAndRevoke`; `MX` |
| 64 | POST | /api/v1/tickets | `BB::TestBB_Tickets_Lifecycle`, `BB::TestBB_RBAC_Unauthenticated`, `EXT::TestExternal_TicketAssign`, `BHV::TestExternal_Ticket_LifecycleAndInvalidTransition`; `MX` |
| 65 | GET | /api/v1/tickets | `BB::TestBB_Tickets_Lifecycle`; `MX` |
| 66 | GET | /api/v1/tickets/:id | `BB::TestBB_Tickets_Lifecycle`, `EXT::TestExternal_TicketAssign`; `MX` |
| 67 | PATCH | /api/v1/tickets/:id/status | `BB::TestBB_Tickets_Lifecycle`, `BHV::TestExternal_Ticket_LifecycleAndInvalidTransition`; `MX` |
| 68 | POST | /api/v1/tickets/:id/assign | `EXT::TestExternal_TicketAssign`; `MX` |
| 69 | POST | /api/v1/tickets/:id/comments | `BB::TestBB_Tickets_Lifecycle`; `MX` |
| 70 | POST | /api/v1/tickets/:id/resolve | `BB::TestBB_Tickets_Lifecycle`, `BHV::TestExternal_Ticket_LifecycleAndInvalidTransition`; `MX` |
| 71 | POST | /api/v1/tickets/:id/close | `BB::TestBB_Tickets_Lifecycle`, `BHV::TestExternal_Ticket_LifecycleAndInvalidTransition`; `MX` |
| 72 | POST | /api/v1/imports | `BB::TestBB_ImportExport`, `EXT::TestExternal_ImportValidate`, `BHV::TestExternal_Import_ValidateApplyGate`; `MX` |
| 73 | GET | /api/v1/imports | `BB::TestBB_ImportExport`, `BB::TestBB_RBAC_Forbidden`; `MX` |
| 74 | GET | /api/v1/imports/:id | `BB::TestBB_ImportExport`, `EXT::TestExternal_ImportValidate`; `MX` |
| 75 | POST | /api/v1/imports/:id/validate | `EXT::TestExternal_ImportValidate`, `BHV::TestExternal_Import_ValidateApplyGate`; `MX` |
| 76 | POST | /api/v1/imports/:id/apply | `BB::TestBB_ImportExport`, `BHV::TestExternal_Import_ValidateApplyGate`; `MX` |
| 77 | POST | /api/v1/exports | `BB::TestBB_ImportExport`, `BHV::TestExternal_Export_DownloadPermissionsAndShape`; `MX` |
| 78 | GET | /api/v1/exports | `BB::TestBB_ImportExport`; `MX` |
| 79 | GET | /api/v1/exports/:id/download | `BB::TestBB_ImportExport`, `BHV::TestExternal_Export_DownloadPermissionsAndShape`; `MX` |

## Execution evidence

Captured from `./run_external_api_tests.sh` (Docker Compose, real
PostgreSQL, real app binary, real `http.Client` against TCP).

Final summary line from the test container:

```
tests-1  | PASS
tests-1  | ok  	github.com/campusrec/campusrec/tests/external_api	18.365s
tests-1 exited with code 0
```

Runner final status:

```
===========================================
  EXTERNAL API TESTS PASSED
===========================================
```

### Selected per-endpoint passing entries (truncated for length)

```
tests-1  | --- PASS: TestExternal_CoverageMatrix (3.86s)
tests-1  |     --- PASS: TestExternal_CoverageMatrix/GET_/health (0.00s)
tests-1  |     --- PASS: TestExternal_CoverageMatrix/POST_/api/v1/auth/register (0.60s)
tests-1  |     --- PASS: TestExternal_CoverageMatrix/POST_/api/v1/auth/login (0.62s)
tests-1  |     --- PASS: TestExternal_CoverageMatrix/POST_/api/v1/auth/logout (0.01s)
tests-1  |     --- PASS: TestExternal_CoverageMatrix/GET_/api/v1/admin/audit-logs (0.00s)
tests-1  |     --- PASS: TestExternal_CoverageMatrix/POST_/api/v1/admin/refunds/00000000-0000-0000-0000-000000000000/reconcile (0.00s)
tests-1  |     --- PASS: TestExternal_CoverageMatrix/POST_/api/v1/admin/registrations/override (0.00s)
tests-1  |     --- PASS: TestExternal_CoverageMatrix/DELETE_/api/v1/cart/items/00000000-0000-0000-0000-000000000000 (0.00s)
tests-1  |     --- PASS: TestExternal_CoverageMatrix/GET_/api/v1/posts (0.00s)
tests-1  |     --- PASS: TestExternal_CoverageMatrix/GET_/api/v1/moderation/cases/00000000-0000-0000-0000-000000000000 (0.00s)
tests-1  |     --- PASS: TestExternal_CoverageMatrix/POST_/api/v1/moderation/cases/00000000-0000-0000-0000-000000000000/action (0.01s)
tests-1  |     --- PASS: TestExternal_CoverageMatrix/POST_/api/v1/moderation/bans (0.00s)
tests-1  |     --- PASS: TestExternal_CoverageMatrix/POST_/api/v1/moderation/bans/00000000-0000-0000-0000-000000000000/revoke (0.01s)
tests-1  |     --- PASS: TestExternal_CoverageMatrix/POST_/api/v1/tickets/00000000-0000-0000-0000-000000000000/assign (0.00s)
tests-1  |     --- PASS: TestExternal_CoverageMatrix/POST_/api/v1/imports/00000000-0000-0000-0000-000000000000/validate (0.00s)
tests-1  |     --- PASS: TestExternal_CoverageMatrix/POST_/api/v1/imports (0.01s)
tests-1  | --- PASS: TestExternal_Health (0.00s)
tests-1  | --- PASS: TestExternal_AdminAuditLogs (1.25s)
tests-1  | --- PASS: TestExternal_AdminRefundReconcile (1.17s)
tests-1  | --- PASS: TestExternal_AdminRegistrationOverride (2.43s)
tests-1  | --- PASS: TestExternal_CartItemDelete (0.62s)
tests-1  | --- PASS: TestExternal_ListPosts (0.59s)
tests-1  | --- PASS: TestExternal_ModerationGetCase (1.07s)
tests-1  | --- PASS: TestExternal_ModerationActionCase (1.07s)
tests-1  | --- PASS: TestExternal_ModerationBanAndRevoke (2.39s)
tests-1  | --- PASS: TestExternal_TicketAssign (1.81s)
tests-1  | --- PASS: TestExternal_ImportValidate (1.29s)
```

Real over-the-wire HTTP transactions are visible in the matched
`app-1` container log lines (sample):

```
app-1 | {"action":"GET /health","status":200, ...}
app-1 | {"action":"POST /api/v1/auth/register","status":201, ...}
app-1 | {"action":"POST /api/v1/admin/refunds/:id/reconcile","status":400, ...}
app-1 | {"action":"POST /api/v1/admin/registrations/override","status":201, ...}
app-1 | {"action":"DELETE /api/v1/cart/items/:id","status":200, ...}
app-1 | {"action":"POST /api/v1/moderation/bans","status":201, ...}
app-1 | {"action":"POST /api/v1/tickets/:id/assign","status":200, ...}
app-1 | {"action":"POST /api/v1/imports/:id/validate","status":200, ...}
```

Each of those log lines is the live application processing a real TCP
request from the test container — confirming that no in-process shortcut
was used.

## Final statement

**External endpoint coverage = 100%.** All 79 routes defined in
`internal/router/router.go` are exercised over real HTTP. The merge
gate's "no implicit coverage" rule is satisfied by the explicit
per-endpoint mapping above; the merge gate's "passes in CI-like
environment" rule is satisfied by `./run_external_api_tests.sh`
exiting 0.
