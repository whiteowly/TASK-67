# API Coverage — Before State

**As of:** initial audit for this task (before adding `tests/external_api/`).

## Methodology

- **External HTTP tests (counts):** tests that open a real TCP listener via `httptest.NewServer(r)` and make requests through `http.Client`. File: `tests/blackbox/blackbox_test.go`.
- **In-process tests (informational only):** tests in `tests/integration/*.go` that use `r.ServeHTTP(httptest.NewRecorder(), req)`. These exercise the handler stack in-process without crossing the HTTP boundary, so per the task requirement they **do not count** toward coverage.

## Summary

| Metric | Value |
|---|---|
| Total endpoints (from inventory) | 79 |
| Externally covered (before) | 67 |
| Externally uncovered (before) | 12 |
| **External coverage (before)** | **67 / 79 = 84.8 %** |
| Additionally covered only in-process | 12 (all 12 gaps have in-process tests — but those don't count) |

## Externally covered endpoints (67)

| # | Endpoint | External test case(s) |
|---|---|---|
| 1 | POST /api/v1/auth/register | TestBB_Auth_RegisterLoginLogout |
| 2 | POST /api/v1/auth/login | TestBB_Auth_RegisterLoginLogout, TestBB_Auth_BadCredentials |
| 3 | POST /api/v1/auth/logout | TestBB_Auth_RegisterLoginLogout |
| 4 | GET /api/v1/users/me | TestBB_Auth_RegisterLoginLogout, TestBB_Auth_ProfileUpdate |
| 5 | PATCH /api/v1/users/me | TestBB_Auth_ProfileUpdate |
| 6 | GET /api/v1/catalog/sessions | TestBB_Catalog |
| 7 | GET /api/v1/catalog/sessions/:id | TestBB_Catalog |
| 8 | GET /api/v1/catalog/products | TestBB_Catalog |
| 9 | GET /api/v1/catalog/products/:id | TestBB_Catalog |
| 10 | GET /api/v1/addresses | TestBB_RBAC_Unauthenticated |
| 11 | POST /api/v1/addresses | TestBB_Addresses |
| 12 | GET /api/v1/addresses/:id | TestBB_Addresses |
| 13 | PATCH /api/v1/addresses/:id | TestBB_Addresses |
| 14 | DELETE /api/v1/addresses/:id | TestBB_Addresses |
| 15 | GET /api/v1/admin/config | TestBB_AdminConfig, TestBB_RBAC_Forbidden |
| 16 | PATCH /api/v1/admin/config/:key | TestBB_AdminConfig |
| 17 | GET /api/v1/admin/feature-flags | TestBB_FeatureFlags, TestBB_RBAC_Forbidden |
| 18 | PATCH /api/v1/admin/feature-flags/:key | TestBB_FeatureFlags |
| 19 | POST /api/v1/admin/backups | TestBB_AdminOps |
| 20 | GET /api/v1/admin/backups | TestBB_AdminOps |
| 21 | POST /api/v1/admin/restore | TestBB_AdminOps |
| 22 | GET /api/v1/admin/archives | TestBB_AdminOps |
| 23 | POST /api/v1/admin/archives | TestBB_AdminOps |
| 24 | GET /api/v1/admin/kpis | TestBB_AdminOps, TestBB_RBAC_Forbidden |
| 25 | GET /api/v1/admin/jobs | TestBB_AdminOps |
| 26 | POST /api/v1/registrations | TestBB_Registration_CreateCancelReadback, TestBB_Registration_Approve, TestBB_Registration_Reject |
| 27 | GET /api/v1/registrations | TestBB_Registration_CreateCancelReadback |
| 28 | GET /api/v1/registrations/:id | TestBB_Registration_CreateCancelReadback, TestBB_Registration_Approve, TestBB_Registration_Reject |
| 29 | POST /api/v1/registrations/:id/cancel | TestBB_Registration_CreateCancelReadback |
| 30 | POST /api/v1/registrations/:id/approve | TestBB_Registration_Approve |
| 31 | POST /api/v1/registrations/:id/reject | TestBB_Registration_Reject |
| 32 | POST /api/v1/attendance/checkin | TestBB_Attendance |
| 33 | POST /api/v1/attendance/leave | TestBB_Attendance |
| 34 | POST /api/v1/attendance/leave/:id/return | TestBB_Attendance |
| 35 | GET /api/v1/attendance/exceptions | TestBB_Attendance |
| 36 | GET /api/v1/cart | TestBB_Commerce_CheckoutAndOrderReadback |
| 37 | POST /api/v1/cart/items | TestBB_Commerce_CheckoutAndOrderReadback, TestBB_Shipments_FullLifecycle |
| 38 | POST /api/v1/checkout | TestBB_Commerce_CheckoutAndOrderReadback, TestBB_Shipments_FullLifecycle |
| 39 | POST /api/v1/buy-now | TestBB_Commerce_BuyNow |
| 40 | GET /api/v1/orders | TestBB_Commerce_CheckoutAndOrderReadback |
| 41 | GET /api/v1/orders/:id | TestBB_Commerce_CheckoutAndOrderReadback, TestBB_Shipments_FullLifecycle |
| 42 | POST /api/v1/orders/:id/pay | TestBB_Commerce_CheckoutAndOrderReadback, TestBB_Shipments_FullLifecycle |
| 43 | POST /api/v1/payments/callback | TestBB_Shipments_FullLifecycle, TestBB_PaymentCallback |
| 44 | POST /api/v1/shipments | TestBB_Shipments_FullLifecycle |
| 45 | GET /api/v1/shipments | TestBB_Shipments_FullLifecycle, TestBB_RBAC_Forbidden |
| 46 | PATCH /api/v1/shipments/:id/status | TestBB_Shipments_FullLifecycle |
| 47 | POST /api/v1/shipments/:id/pod | TestBB_Shipments_FullLifecycle |
| 48 | POST /api/v1/shipments/:id/exception | TestBB_Shipments_FullLifecycle |
| 49 | GET /api/v1/posts/:id | TestBB_Moderation |
| 50 | POST /api/v1/posts | TestBB_Moderation |
| 51 | POST /api/v1/posts/:id/report | TestBB_Moderation |
| 52 | GET /api/v1/moderation/reports | TestBB_Moderation |
| 53 | GET /api/v1/moderation/cases | TestBB_Moderation, TestBB_RBAC_Forbidden |
| 54 | POST /api/v1/tickets | TestBB_Tickets_Lifecycle, TestBB_RBAC_Unauthenticated |
| 55 | GET /api/v1/tickets | TestBB_Tickets_Lifecycle |
| 56 | GET /api/v1/tickets/:id | TestBB_Tickets_Lifecycle |
| 57 | PATCH /api/v1/tickets/:id/status | TestBB_Tickets_Lifecycle |
| 58 | POST /api/v1/tickets/:id/comments | TestBB_Tickets_Lifecycle |
| 59 | POST /api/v1/tickets/:id/resolve | TestBB_Tickets_Lifecycle |
| 60 | POST /api/v1/tickets/:id/close | TestBB_Tickets_Lifecycle |
| 61 | POST /api/v1/imports | TestBB_ImportExport |
| 62 | GET /api/v1/imports | TestBB_ImportExport, TestBB_RBAC_Forbidden |
| 63 | GET /api/v1/imports/:id | TestBB_ImportExport |
| 64 | POST /api/v1/imports/:id/apply | TestBB_ImportExport |
| 65 | POST /api/v1/exports | TestBB_ImportExport |
| 66 | GET /api/v1/exports | TestBB_ImportExport |
| 67 | GET /api/v1/exports/:id/download | TestBB_ImportExport |

## Externally uncovered endpoints (12)

| # | Endpoint | In-process coverage? |
|---|---|---|
| 1 | GET /health | no external — no in-process either (trivially used by Docker healthcheck only) |
| 2 | GET /api/v1/admin/audit-logs | yes — `audit_fixes_test.go` (in-process, doesn't count) |
| 3 | POST /api/v1/admin/refunds/:id/reconcile | yes — `round*_fixes_test.go` (in-process) |
| 4 | POST /api/v1/admin/registrations/override | yes — `admin_ops_test.go` (in-process) |
| 5 | DELETE /api/v1/cart/items/:id | yes — `order_test.go` (in-process) |
| 6 | GET /api/v1/posts | yes — `moderation_test.go` (in-process) |
| 7 | GET /api/v1/moderation/cases/:id | yes — `moderation_test.go` (in-process) |
| 8 | POST /api/v1/moderation/cases/:id/action | yes — `moderation_test.go` (in-process) |
| 9 | POST /api/v1/moderation/bans | yes — `round*_fixes_test.go` (in-process) |
| 10 | POST /api/v1/moderation/bans/:id/revoke | yes — `round*_fixes_test.go` (in-process) |
| 11 | POST /api/v1/tickets/:id/assign | yes — `round*_fixes_test.go` (in-process) |
| 12 | POST /api/v1/imports/:id/validate | yes — `round*_fixes_test.go` (in-process) |

## Informational — In-process test files (do not count)

All files under `tests/integration/` use `r.ServeHTTP(httptest.NewRecorder(), req)`:

- `address_test.go`, `admin_ops_test.go`, `api_coverage_test.go`, `audit_fixes_test.go`,
  `auth_test.go`, `catalog_test.go`, `moderation_test.go`, `order_test.go`, `rbac_test.go`,
  `registration_test.go`, `round2_fixes_test.go`, `round3_fixes_test.go`, `round4_fixes_test.go`,
  `round5_fixes_test.go`.

These test files were retained (as permitted) but **not counted** toward the 100 % target.

## Gap to close

**12 endpoints** need direct external HTTP coverage. See `docs/api-coverage-after.md` for the delivered state.
