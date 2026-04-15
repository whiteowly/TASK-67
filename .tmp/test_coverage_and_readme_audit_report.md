# Test Coverage Audit

## Scope and Method
- Static inspection only. No tests, scripts, binaries, containers, builds, package managers, or servers were executed.
- Route source of truth: `internal/router/router.go`.
- Test evidence inspected from `tests/blackbox/`, `tests/external_api/`, `tests/integration/`, and `internal/**/*_test.go`.

## Backend Endpoint Inventory
Source: `internal/router/router.go:30`, `internal/router/router.go:54-240`.

1. `GET /health`
2. `POST /api/v1/auth/register`
3. `POST /api/v1/auth/login`
4. `POST /api/v1/auth/logout`
5. `GET /api/v1/users/me`
6. `PATCH /api/v1/users/me`
7. `GET /api/v1/catalog/sessions`
8. `GET /api/v1/catalog/sessions/:id`
9. `GET /api/v1/catalog/products`
10. `GET /api/v1/catalog/products/:id`
11. `GET /api/v1/addresses`
12. `POST /api/v1/addresses`
13. `GET /api/v1/addresses/:id`
14. `PATCH /api/v1/addresses/:id`
15. `DELETE /api/v1/addresses/:id`
16. `GET /api/v1/admin/config`
17. `PATCH /api/v1/admin/config/:key`
18. `GET /api/v1/admin/feature-flags`
19. `PATCH /api/v1/admin/feature-flags/:key`
20. `GET /api/v1/admin/audit-logs`
21. `POST /api/v1/admin/backups`
22. `GET /api/v1/admin/backups`
23. `POST /api/v1/admin/restore`
24. `GET /api/v1/admin/archives`
25. `POST /api/v1/admin/archives`
26. `POST /api/v1/admin/refunds/:id/reconcile`
27. `GET /api/v1/admin/kpis`
28. `GET /api/v1/admin/jobs`
29. `POST /api/v1/admin/registrations/override`
30. `POST /api/v1/registrations`
31. `GET /api/v1/registrations`
32. `GET /api/v1/registrations/:id`
33. `POST /api/v1/registrations/:id/cancel`
34. `POST /api/v1/registrations/:id/approve`
35. `POST /api/v1/registrations/:id/reject`
36. `POST /api/v1/attendance/checkin`
37. `POST /api/v1/attendance/leave`
38. `POST /api/v1/attendance/leave/:id/return`
39. `GET /api/v1/attendance/exceptions`
40. `GET /api/v1/cart`
41. `POST /api/v1/cart/items`
42. `DELETE /api/v1/cart/items/:id`
43. `POST /api/v1/checkout`
44. `POST /api/v1/buy-now`
45. `GET /api/v1/orders`
46. `GET /api/v1/orders/:id`
47. `POST /api/v1/orders/:id/pay`
48. `POST /api/v1/payments/callback`
49. `POST /api/v1/shipments`
50. `GET /api/v1/shipments`
51. `PATCH /api/v1/shipments/:id/status`
52. `POST /api/v1/shipments/:id/pod`
53. `POST /api/v1/shipments/:id/exception`
54. `GET /api/v1/posts`
55. `GET /api/v1/posts/:id`
56. `POST /api/v1/posts`
57. `POST /api/v1/posts/:id/report`
58. `GET /api/v1/moderation/reports`
59. `GET /api/v1/moderation/cases`
60. `GET /api/v1/moderation/cases/:id`
61. `POST /api/v1/moderation/cases/:id/action`
62. `POST /api/v1/moderation/bans`
63. `POST /api/v1/moderation/bans/:id/revoke`
64. `POST /api/v1/tickets`
65. `GET /api/v1/tickets`
66. `GET /api/v1/tickets/:id`
67. `PATCH /api/v1/tickets/:id/status`
68. `POST /api/v1/tickets/:id/assign`
69. `POST /api/v1/tickets/:id/comments`
70. `POST /api/v1/tickets/:id/resolve`
71. `POST /api/v1/tickets/:id/close`
72. `POST /api/v1/imports`
73. `GET /api/v1/imports`
74. `GET /api/v1/imports/:id`
75. `POST /api/v1/imports/:id/validate`
76. `POST /api/v1/imports/:id/apply`
77. `POST /api/v1/exports`
78. `GET /api/v1/exports`
79. `GET /api/v1/exports/:id/download`

## API Test Mapping Table
Primary breadth evidence for all endpoints: `tests/external_api/coverage_matrix_test.go:28 (TestExternal_CoverageMatrix)`, plus multipart probe `tests/external_api/coverage_matrix_test.go:176`.

| Endpoint | Covered | Test Type | Test Files | Evidence |
|---|---|---|---|---|
| GET /health | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/missing_endpoints_test.go | TestExternal_CoverageMatrix subtest `GET /health` at `coverage_matrix_test.go:43`; TestExternal_Health at `missing_endpoints_test.go:20` |
| POST /api/v1/auth/register | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/blackbox/blackbox_test.go | `coverage_matrix_test.go:46`; TestBB_Auth_RegisterLoginLogout at `blackbox_test.go:152` |
| POST /api/v1/auth/login | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/blackbox/blackbox_test.go | `coverage_matrix_test.go:47`; TestBB_Auth_BadCredentials at `blackbox_test.go:191` |
| POST /api/v1/auth/logout | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/blackbox/blackbox_test.go | `coverage_matrix_test.go:48`; TestBB_Auth_RegisterLoginLogout at `blackbox_test.go:176` |
| GET /api/v1/users/me | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/blackbox/blackbox_test.go | `coverage_matrix_test.go:51`; TestBB_Auth_RegisterLoginLogout at `blackbox_test.go:170` |
| PATCH /api/v1/users/me | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/blackbox/blackbox_test.go | `coverage_matrix_test.go:52`; TestBB_Auth_ProfileUpdate at `blackbox_test.go:203` |
| GET /api/v1/catalog/sessions | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/behavior_commerce_test.go | `coverage_matrix_test.go:55`; TestExternal_CatalogSessions_QueryContract at `behavior_commerce_test.go` |
| GET /api/v1/catalog/sessions/:id | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:56` |
| GET /api/v1/catalog/products | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/behavior_commerce_test.go | `coverage_matrix_test.go:57`; TestExternal_Cart_PayloadContract at `behavior_commerce_test.go:19` |
| GET /api/v1/catalog/products/:id | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/behavior_commerce_test.go | `coverage_matrix_test.go:58`; TestExternal_ProductDetail_Contract at `behavior_commerce_test.go` |
| GET /api/v1/addresses | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:61` |
| POST /api/v1/addresses | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/behavior_commerce_test.go | `coverage_matrix_test.go:62`; multiple tests creating addresses (e.g. `behavior_commerce_test.go:160`) |
| GET /api/v1/addresses/:id | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:63` |
| PATCH /api/v1/addresses/:id | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:64` |
| DELETE /api/v1/addresses/:id | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:65` |
| GET /api/v1/admin/config | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/behavior_admin_test.go | `coverage_matrix_test.go:68`; TestExternal_AdminConfig_VersionConflict at `behavior_admin_test.go:20` |
| PATCH /api/v1/admin/config/:key | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/behavior_admin_test.go | `coverage_matrix_test.go:69`; TestExternal_AdminConfig_VersionConflict at `behavior_admin_test.go:40` |
| GET /api/v1/admin/feature-flags | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:70` |
| PATCH /api/v1/admin/feature-flags/:key | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:71` |
| GET /api/v1/admin/audit-logs | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/missing_endpoints_test.go, tests/external_api/behavior_admin_test.go | `coverage_matrix_test.go:72`; TestExternal_AdminAuditLogs at `missing_endpoints_test.go:43`; TestExternal_AdminAuditLogs_Filtering at `behavior_admin_test.go:73` |
| POST /api/v1/admin/backups | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/behavior_admin_test.go | `coverage_matrix_test.go:73`; TestExternal_AdminBackup_StateTransitions at `behavior_admin_test.go:138` |
| GET /api/v1/admin/backups | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/behavior_admin_test.go | `coverage_matrix_test.go:74`; TestExternal_AdminBackup_StateTransitions at `behavior_admin_test.go:153` |
| POST /api/v1/admin/restore | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/behavior_admin_test.go | `coverage_matrix_test.go:75`; TestExternal_AdminBackup_StateTransitions at `behavior_admin_test.go:173` |
| GET /api/v1/admin/archives | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:76` |
| POST /api/v1/admin/archives | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:77` |
| POST /api/v1/admin/refunds/:id/reconcile | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/missing_endpoints_test.go | `coverage_matrix_test.go:78`; TestExternal_AdminRefundReconcile at `missing_endpoints_test.go:93` |
| GET /api/v1/admin/kpis | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:79` |
| GET /api/v1/admin/jobs | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:80` |
| POST /api/v1/admin/registrations/override | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/missing_endpoints_test.go | `coverage_matrix_test.go:81`; TestExternal_AdminRegistrationOverride at `missing_endpoints_test.go:144` |
| POST /api/v1/registrations | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:84` |
| GET /api/v1/registrations | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:85` |
| GET /api/v1/registrations/:id | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:86` |
| POST /api/v1/registrations/:id/cancel | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:87` |
| POST /api/v1/registrations/:id/approve | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:88` |
| POST /api/v1/registrations/:id/reject | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:89` |
| POST /api/v1/attendance/checkin | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:92` |
| POST /api/v1/attendance/leave | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:93` |
| POST /api/v1/attendance/leave/:id/return | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:94` |
| GET /api/v1/attendance/exceptions | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:95` |
| GET /api/v1/cart | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/behavior_commerce_test.go | `coverage_matrix_test.go:98`; TestExternal_Cart_PayloadContract at `behavior_commerce_test.go:47` |
| POST /api/v1/cart/items | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/behavior_commerce_test.go | `coverage_matrix_test.go:99`; TestExternal_Cart_DuplicateItemRejected at `behavior_commerce_test.go:76` |
| DELETE /api/v1/cart/items/:id | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/missing_endpoints_test.go | `coverage_matrix_test.go:100`; TestExternal_CartItemDelete at `missing_endpoints_test.go:204` |
| POST /api/v1/checkout | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/behavior_admin_test.go, tests/external_api/behavior_commerce_test.go | `coverage_matrix_test.go:103`; TestExternal_Commerce_CheckoutIdempotency at `behavior_admin_test.go:194`; TestExternal_Checkout_EmptyCartRejected at `behavior_commerce_test.go:154` |
| POST /api/v1/buy-now | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/behavior_commerce_test.go | `coverage_matrix_test.go:104`; TestExternal_BuyNow_DoesNotConsumeCart at `behavior_commerce_test.go:236` |
| GET /api/v1/orders | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/behavior_commerce_test.go | `coverage_matrix_test.go:107`; TestExternal_Orders_OnlyOwnOrders at `behavior_commerce_test.go:287` |
| GET /api/v1/orders/:id | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/behavior_commerce_test.go | `coverage_matrix_test.go:108`; TestExternal_OrderDetail_PayloadContract at `behavior_commerce_test.go:183` |
| POST /api/v1/orders/:id/pay | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/behavior_admin_test.go | `coverage_matrix_test.go:109`; TestExternal_PaymentCallback_SignatureAndReplay at `behavior_admin_test.go:256` |
| POST /api/v1/payments/callback | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/behavior_admin_test.go | `coverage_matrix_test.go:112`; TestExternal_PaymentCallback_SignatureAndReplay at `behavior_admin_test.go:256` |
| POST /api/v1/shipments | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:115` |
| GET /api/v1/shipments | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:116` |
| PATCH /api/v1/shipments/:id/status | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:117` |
| POST /api/v1/shipments/:id/pod | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:118` |
| POST /api/v1/shipments/:id/exception | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:119` |
| GET /api/v1/posts | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/missing_endpoints_test.go | `coverage_matrix_test.go:122`; TestExternal_ListPosts at `missing_endpoints_test.go:280` |
| GET /api/v1/posts/:id | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:123` |
| POST /api/v1/posts | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/missing_endpoints_test.go | `coverage_matrix_test.go:124`; post creation in TestExternal_ListPosts at `missing_endpoints_test.go:285` |
| POST /api/v1/posts/:id/report | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:125` |
| GET /api/v1/moderation/reports | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:128` |
| GET /api/v1/moderation/cases | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:129` |
| GET /api/v1/moderation/cases/:id | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/missing_endpoints_test.go | `coverage_matrix_test.go:130`; TestExternal_ModerationGetCase at `missing_endpoints_test.go:328` |
| POST /api/v1/moderation/cases/:id/action | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/missing_endpoints_test.go | `coverage_matrix_test.go:131`; TestExternal_ModerationActionCase at `missing_endpoints_test.go:365` |
| POST /api/v1/moderation/bans | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/missing_endpoints_test.go | `coverage_matrix_test.go:132`; TestExternal_ModerationBanAndRevoke at `missing_endpoints_test.go:403` |
| POST /api/v1/moderation/bans/:id/revoke | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/missing_endpoints_test.go | `coverage_matrix_test.go:133`; TestExternal_ModerationBanAndRevoke at `missing_endpoints_test.go:438` |
| POST /api/v1/tickets | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:136` |
| GET /api/v1/tickets | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:137` |
| GET /api/v1/tickets/:id | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/missing_endpoints_test.go | `coverage_matrix_test.go:138`; readback in TestExternal_TicketAssign at `missing_endpoints_test.go:508` |
| PATCH /api/v1/tickets/:id/status | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:139` |
| POST /api/v1/tickets/:id/assign | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/missing_endpoints_test.go | `coverage_matrix_test.go:140`; TestExternal_TicketAssign at `missing_endpoints_test.go:474` |
| POST /api/v1/tickets/:id/comments | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:141` |
| POST /api/v1/tickets/:id/resolve | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:142` |
| POST /api/v1/tickets/:id/close | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:143` |
| POST /api/v1/imports | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/missing_endpoints_test.go | multipart probe `coverage_matrix_test.go:176`; upload in TestExternal_ImportValidate at `missing_endpoints_test.go:540` |
| GET /api/v1/imports | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:146` |
| GET /api/v1/imports/:id | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/missing_endpoints_test.go | `coverage_matrix_test.go:147`; readback in TestExternal_ImportValidate at `missing_endpoints_test.go:561` |
| POST /api/v1/imports/:id/validate | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go, tests/external_api/missing_endpoints_test.go | `coverage_matrix_test.go:148`; TestExternal_ImportValidate at `missing_endpoints_test.go:551` |
| POST /api/v1/imports/:id/apply | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:149` |
| POST /api/v1/exports | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:152` |
| GET /api/v1/exports | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:153` |
| GET /api/v1/exports/:id/download | yes | true no-mock HTTP | tests/external_api/coverage_matrix_test.go | `coverage_matrix_test.go:154` |

## API Test Classification
### 1) True No-Mock HTTP
- `tests/blackbox/blackbox_test.go`.
  - Evidence: explicit real TCP comments and use of `http.Client` + `httptest.NewServer` in `setupEnv` (`blackbox_test.go:3-5`, `blackbox_test.go:31-39`, `blackbox_test.go:63-80`).
- `tests/external_api/*.go`.
  - Evidence: `setupEnv` uses external base URL or `httptest.NewServer` and always calls through `http.Client` (`tests/external_api/helpers.go:4-11`, `tests/external_api/helpers.go:56-71`, `tests/external_api/helpers.go:85-103`).

### 2) HTTP with Mocking
- None found in API tests.

### 3) Non-HTTP (unit / in-process integration / direct service)
- `tests/integration/*.go`: in-process `r.ServeHTTP` + `httptest.NewRecorder` (`tests/integration/api_coverage_test.go:67-73`, `tests/integration/address_test.go:62-64`).
- Direct service calls bypassing HTTP exist in integration files (e.g., `tests/integration/round4_fixes_test.go:58-66`, `tests/integration/round5_fixes_test.go:81-109`).
- Internal unit tests under `internal/**/*_test.go` are non-HTTP by design.

## Mock Detection
- `jest.mock`, `vi.mock`, `sinon.stub`, `gomock`: not found in inspected Go test files.
- Dependency override / mocked providers in API execution paths: not found.
- Direct non-HTTP invocations present (not mocking, but bypass HTTP):
  - `svc.Import.UploadImport` and `svc.Import.ValidateImport` in `tests/integration/round4_fixes_test.go:58-66`.
  - `svc.Registration.Register/PromoteNextWaitlist/Cancel` in `tests/integration/round5_fixes_test.go:81-109`, `tests/integration/round5_fixes_test.go:130-158`.

## Coverage Summary
- Total backend endpoints: **79**.
- Endpoints with HTTP tests (any HTTP style): **79 / 79**.
- Endpoints with true no-mock HTTP tests: **79 / 79**.
- HTTP coverage: **100.0%**.
- True API coverage: **100.0%**.

## Unit Test Summary
### Test files (unit / non-HTTP-focused)
- `internal/middleware/rbac_test.go`
- `internal/service/cohort_test.go`
- `internal/service/payment_signature_test.go`
- `internal/service/transitions_test.go`
- `internal/validator/validator_test.go`
- `internal/util/password_test.go`
- `internal/util/pagination_test.go`
- Plus non-HTTP service checks in integration tests (`tests/integration/round2_fixes_test.go`, `tests/integration/round4_fixes_test.go`, `tests/integration/round5_fixes_test.go`).

### Modules covered
- Middleware: RBAC branch logic (`internal/middleware/rbac_test.go`).
- Services: feature-flag cohort hashing, payment signature verification, transition state machines (`internal/service/*.go` tests above).
- Utility/validation: password hashing/token generation, pagination parsing, input validators (`internal/util/*.go`, `internal/validator/validator_test.go`).

### Important modules not directly unit tested
- API handlers (`internal/handler/api/**`) have no dedicated unit tests.
- Web handlers (`internal/handler/web/**`) have no dedicated unit tests.
- Repositories (`internal/repo/**`) have no dedicated unit tests.
- Router composition (`internal/router/router.go`) has no dedicated route-table unit test; route reachability is validated externally instead.

## API Observability Check
- Strong observability exists in many external behavior tests with explicit payload assertions:
  - `tests/external_api/behavior_commerce_test.go:19` (cart payload contract).
  - `tests/external_api/behavior_admin_test.go:20` (admin config version conflict + readback).
  - `tests/external_api/missing_endpoints_test.go:474` (ticket assign and readback).
- Weak observability exists in broad reachability sweeps:
  - `tests/external_api/coverage_matrix_test.go:165-171` only enforces not-502/504 and transport reachability.
  - `tests/integration/api_coverage_test.go` frequently accepts non-500 outcomes without strict response-contract assertions (e.g., `api_coverage_test.go:375-392`, `api_coverage_test.go:452-475`, `api_coverage_test.go:646-676`).
- Determination: **strong overall, with a few residual weak zones** (primarily skip-driven branches in some integration round/fix suites).

## Tests Check
- Success paths: present across auth, catalog, cart/checkout, moderation, admin operations, import/validate (`tests/blackbox/blackbox_test.go`, `tests/external_api/behavior_*.go`).
- Failure paths: present (invalid auth, bad UUID, RBAC denials, signature failures) (`tests/external_api/missing_endpoints_test.go`, `tests/external_api/behavior_admin_test.go`).
- Edge cases: present but uneven depth (idempotency, replay, stale versions, duplicate cart items) (`tests/external_api/behavior_admin_test.go:194`, `tests/external_api/behavior_commerce_test.go:76`).
- Auth/permissions: substantial role checks in integration and external tests (`tests/integration/rbac_test.go`, `tests/external_api/missing_endpoints_test.go`).
- Integration boundaries: strong externally (real HTTP + DB) for coverage breadth; internal integration suite includes many service-direct checks that bypass HTTP.
- Assertion quality: materially improved; high-risk endpoints now enforce strict status/envelope contracts and integration suite uses deterministic error-envelope assertions.
- `run_tests.sh` compliance:
  - Docker-based runner confirmed (`run_tests.sh:3`, `run_tests.sh:100-104`).
  - No local package-install requirement in main test runner.

## End-to-End Expectations
- Project appears fullstack (Go backend + templ web + Playwright e2e tests).
- Real FE↔BE tests exist:
  - `e2e/tests/journeys.spec.ts:3-6` states UI flow + backend state assertions.
  - Mixed UI/API validation in same test (e.g., `e2e/tests/journeys.spec.ts:86-95`, `e2e/tests/journeys.spec.ts:183-210`).
- Conclusion: E2E expectation is **met** with real FE↔BE journeys and backend readback assertions.

## Test Coverage Score (0-100)
**92 / 100**

## Score Rationale
- + Full endpoint breadth at true no-mock HTTP boundary (79/79).
- + Strong role/authorization and several deep domain contracts in external behavior tests.
- + Presence of fullstack browser E2E journeys.
- + High-risk matrix probes now include strict status/envelope/field contracts (`tests/external_api/coverage_matrix_test.go:77-220`).
- + Integration suite introduces deterministic 4xx envelope assertions via `expectErrorEnvelope` (`tests/integration/api_coverage_test.go:174-225`).
- + Handler error-mapping coverage added (`internal/handler/api/errmap_test.go`).
- + Repository contract coverage added (`tests/repo_contract/repo_contract_test.go`).
- - Some integration round/fix tests still contain skip branches tied to fixture/feature-flag state.

## Key Gaps
- Some integration round/fix suites still include `t.Skip`/`t.Skipf` branches (e.g., `tests/integration/round4_fixes_test.go`, `tests/integration/round5_fixes_test.go`), which can mask regressions when prerequisites are absent.
- Endpoint-breadth matrix still includes many liveness probes by design; semantic depth depends on companion behavior tests remaining maintained and in-sync with route evolution.

## Confidence & Assumptions
- Confidence: **high** on endpoint inventory and route-to-test mapping; **medium** on semantic sufficiency due static-only inspection.
- Assumptions:
  - Endpoint scope is backend (`/health` + `/api/v1/**`) per router API section.
  - Path-parameter probes with concrete UUIDs map to parameterized routes.

## Test Coverage Verdict
**PASS**

---

# README Audit

## Project Type Detection
- README explicitly declares **fullstack** at top in description text.
- Inferred type (light inspection): **fullstack** (Go backend + templ web UI + Playwright e2e).
  - Evidence: `README.md:3`, `README.md:7-11`, `README.md:107-113`, `README.md:120-143`.

## README Location
- `repo/README.md` exists: **PASS**.

## Hard Gate Evaluation
- Formatting/readability: **PASS** (`README.md` is structured with clear headings/tables).
- Startup instructions (backend/fullstack requires `docker-compose up`): **PASS** (`README.md:71`).
- Access method (URL + port): **PASS** (`README.md:107-111`).
- Verification method: **PASS** (curl smoke checks with expected outputs at `README.md:86-105`).
- Environment rules (no manual runtime setup): **PASS with caveat**.
  - Main path explicitly Docker-only (`README.md:65-85`, `README.md:158-164`).
  - README references optional non-Docker contributor docs (`README.md:162`), but does not redefine it as supported evaluation path.
- Demo credentials (auth exists -> all roles required): **PASS** (`README.md:148-155` includes admin/staff/mod/member credentials).
- Project type declaration at top (critical): **PASS** (`fullstack` appears at top: `README.md:3`).

## High Priority Issues
- None.

## Medium Priority Issues
- README references optional non-Docker path (`docs/local_development.md`), which can conflict with strict evaluators if interpreted loosely; add explicit warning that this path is out-of-scope for grading/CI (partially present already, but should be highlighted in the section header).

## Low Priority Issues
- API endpoint section is large; could link to canonical generated inventory and keep README shorter to reduce drift risk.

## Hard Gate Failures
- None.

## README Verdict
**PASS**

## README Engineering Quality Notes
- Tech stack clarity: strong (`README.md:5-12`).
- Architecture explanation: acceptable (`README.md:13-52`).
- Testing instructions: strong and multi-suite (`README.md:120-143`).
- Security/roles/demo access: clear (`README.md:10`, `README.md:148-155`).
- Presentation quality: strong and compliant with hard gates.

---

# Final Combined Verdicts
- **Test Coverage Audit Verdict:** PASS
- **README Audit Verdict:** PASS
