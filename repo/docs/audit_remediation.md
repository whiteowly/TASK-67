# Audit Remediation — Test Coverage + README

This document maps every issue and gap raised in
`/tmp/test_coverage_and_readme_audit_report.md` (initial score
**83 / 100**) to the concrete repository changes that resolve it. Each row
points at the exact files and tests that satisfy the audit criterion.

---

## 1. README hard-gate failures

### 1a. Hard gate FAIL — environment policy (manual local DB setup in main flow)

**Audit evidence:** `README.md:202–218` exposed `go run ./cmd/server -migrate up`,
`go run ./cmd/server -seed`, and host-side env-var exports as part of the
documented run flow.

**Fix:** removed entirely from `README.md`. The local-dev path now lives in
`docs/local_development.md` with an explicit "**This is not part of the
supported run path.**" warning at the top.

| Change | Path | Detail |
|---|---|---|
| Local-dev section deleted from README | `README.md` | Old lines 202–230 are gone. |
| New optional doc | `docs/local_development.md` | Marked clearly as non-supported; explains why and points back at `docker-compose up`. |
| README links to it | `README.md` (final section "Optional: Contributor / non-Docker workflow") | Single link, no commands inline. |

**How this satisfies the gate:** the README main path is now
fully Docker-contained. There are no `go run`, `go install`, `psql`, or
`export DATABASE_URL` instructions on the supported path.

### 1b. Hard gate FAIL — missing literal `docker-compose up` token

**Audit evidence:** `README.md:18` used `docker compose up --build` (modern
form) but not the literal hyphenated token the gate requires.

**Fix:** the Quick Start now contains the literal command and explains the
modern equivalent.

| Change | Path | Line |
|---|---|---|
| Literal `docker-compose up` in Quick Start | `README.md` | line 19 |
| Modern equivalent noted | `README.md` | line 22: `docker compose up --build` |

**How this satisfies the gate:** the literal token `docker-compose up`
appears in a fenced bash block under "Quick Start" and is referenced again
in the smoke-check section heading.

### 1c. Medium-priority — implicit verification, no smoke checklist

**Audit evidence:** README listed endpoints/pages but had no "run these 2–3
checks and expected results" block.

**Fix:** new "Smoke check" section with three deterministic curls and
exact expected outputs.

| Check | Asserts |
|---|---|
| `curl /health` → `{"status":"ok"}` | Server is alive, port reachable. |
| `curl /api/v1/catalog/sessions` → `{"success":true,"data":[…` | Public route + seed data present. |
| `POST /api/v1/auth/login` (admin) → `{"success":true,…}` | Auth path + seeded admin works. |

Path: `README.md` (section "Smoke check (run after `docker-compose up`)").

### 1d. Medium-priority — test-command fragmentation, no release-gate command

**Audit evidence:** `run_external_api_tests.sh`, `run_tests.sh`, `run_e2e.sh`
existed but no single "release confidence" entry point.

**Fix:** `run_all_tests.sh` orchestrates all suites (drift guard →
unit/integration → external API → browser E2E) with fail-fast and a
combined per-suite summary. README documents it as the **primary
release-gate command**.

| Path | Role |
|---|---|
| `run_all_tests.sh` | Top-level orchestrator. |
| `README.md` "Run every suite in one command" | Documents it as primary. |
| `docs/testing.md` | Per-suite descriptions and what each proves. |

---

## 2. Key gap: API depth (matrix-led endpoints lacked behavior assertions)

**Audit evidence:**
- "Some endpoints are covered mainly by reachability matrix tests."
- `tests/external_api/coverage_matrix_test.go:165–171` — matrix asserts only
  non-502/504, not payload semantics.

**Fix:** the matrix is preserved as a routing fence, but explicit
behavior tests cover state transitions, payload contracts, idempotency,
RBAC, and failure modes for the highest-risk surfaces.

| Domain | New behavior tests | File |
|---|---|---|
| Admin config | `TestExternal_AdminConfig_VersionConflict` (optimistic-version 409 + readback) | `tests/external_api/behavior_admin_test.go` |
| Audit logs | `TestExternal_AdminAuditLogs_Filtering` (filter narrows result, no-match → 0) | `tests/external_api/behavior_admin_test.go` |
| Backup / Restore | `TestExternal_AdminBackup_StateTransitions` (run → list contains → restore validation) | `tests/external_api/behavior_admin_test.go` |
| Commerce — checkout idempotency | `TestExternal_Commerce_CheckoutIdempotency` (same key → same order id; different key → different order id) | `tests/external_api/behavior_admin_test.go` |
| Commerce — payment callback HMAC + replay | `TestExternal_PaymentCallback_SignatureAndReplay` (bad sig rejected, good sig confirms, replay returns same payment_id) | `tests/external_api/behavior_admin_test.go` |
| Tickets lifecycle | `TestExternal_Ticket_LifecycleAndInvalidTransition` (open → ack → resolved → closed; reject post-close resolve) | `tests/external_api/behavior_admin_test.go` |
| Imports | `TestExternal_Import_ValidateApplyGate` (apply-before-validate rejected with `validat`-message; validate advances status) | `tests/external_api/behavior_admin_test.go` |
| Exports | `TestExternal_Export_DownloadPermissionsAndShape` (admin gets CSV with `Content-Disposition: attachment`; staff → 403) | `tests/external_api/behavior_admin_test.go` |
| Moderation lists | `TestExternal_Moderation_ListContract` (envelope shape + RBAC for cases/reports) | `tests/external_api/behavior_admin_test.go` |
| Cart payload contract | `TestExternal_Cart_PayloadContract` (snapshot price = catalog price; required keys present) | `tests/external_api/behavior_commerce_test.go` |
| Cart dedup | `TestExternal_Cart_DuplicateItemRejected` | `tests/external_api/behavior_commerce_test.go` |
| Cart cross-user isolation | `TestExternal_Cart_CrossUserDeleteIsIsolated` | `tests/external_api/behavior_commerce_test.go` |
| Empty-cart checkout | `TestExternal_Checkout_EmptyCartRejected` | `tests/external_api/behavior_commerce_test.go` |
| Order detail contract | `TestExternal_OrderDetail_PayloadContract` (id, order_number, status, subtotal>0, currency, valid status) | `tests/external_api/behavior_commerce_test.go` |
| Buy-Now isolation | `TestExternal_BuyNow_DoesNotConsumeCart` | `tests/external_api/behavior_commerce_test.go` |
| Order ownership | `TestExternal_Orders_OnlyOwnOrders` (cross-user list + detail isolated) | `tests/external_api/behavior_commerce_test.go` |
| Catalog pagination + search | `TestExternal_Catalog_PaginationAndSearch` (`per_page=1` caps; no-match → empty + meta.total=0) | `tests/external_api/behavior_commerce_test.go` |
| Auth register validation | `TestExternal_Auth_RegisterValidation` (4 sub-tests: missing username/password/weak/empty body) | `tests/external_api/behavior_commerce_test.go` |
| Catalog product detail shape | `TestExternal_CatalogProduct_Detail_Shape` | `tests/external_api/behavior_commerce_test.go` |

All tests run over real TCP via `EXTERNAL_API_BASE_URL=http://app:8080`
inside `docker-compose.external-api.yml`. Run with
`./run_external_api_tests.sh`. No HTTP-layer mocks are introduced.

---

## 3. Key gap: Unit depth (services / decision logic lacked isolated tests)

**Audit evidence:**
- "Limited isolated unit tests for service/repository decision logic."
- "Most `internal/service/*` business flows … weakly unit-tested."

**Fix:** added pure unit tests for the highest-leverage decision functions.
These are pure (no DB / network / time), so they execute in milliseconds
and gate every release.

| Function under test | New test file | What it proves |
|---|---|---|
| `service.isValidTicketTransition` | `internal/service/transitions_test.go::TestIsValidTicketTransition_*` | All 21 allowed edges accepted; 9 disallowed edges (sinks, illegal jumps, self-loops) rejected; unknown source state rejected; every non-sink status has at least one outbound edge (regression net for new statuses). |
| `service.isValidShipmentTransition` | `internal/service/transitions_test.go::TestIsValidShipmentTransition_*` | 8 allowed edges accepted; 10 disallowed edges (cannot ship before pack, cannot deliver before ship, sinks, post-ship cancel) rejected; unknown source rejected. |
| `service.cohortBucket` (extracted) | `internal/service/cohort_test.go::TestCohortBucket_*` | Determinism, in-range [0,100), version-bump reshuffles, flag-key affects bucket, ~uniform distribution, selection inequality. |
| `service.PaymentService.verifySignature` | `internal/service/payment_signature_test.go::TestVerifySignature_*` | Empty key always rejects, happy path, 3 tampering paths (gateway tx, ref, amount), wrong key, empty/non-hex/wrong-length/bit-flipped signatures, negative amount edge. |
| `middleware.RequireAuth` | `internal/middleware/rbac_test.go::TestRequireAuth_*` | Both branches: unauthenticated → 401, authenticated → next runs. |
| `middleware.RequireRole` | `internal/middleware/rbac_test.go::TestRequireRole_*` | All branches: unauthenticated → 401, role mismatch → 403, role match → next; multi-role match; empty roles → 403. |

To keep the cohort unit testable without requiring repo mocks, the
deterministic hashing was extracted into a pure helper:

| Refactor | File | Detail |
|---|---|---|
| `cohortBucket(flagKey, userID, version) int` | `internal/service/feature_flag_service.go` | Pure helper called by `IsEnabledForUser`. No behavior change. |

---

## 4. Key gap: E2E depth (browser tests too shallow for business journeys)

**Audit evidence:** "Browser E2E scenarios are present but shallow for
complex lifecycle flows (checkout/payment/shipment/moderation end-to-end
UI chains)."

**Fix:** new `e2e/tests/journeys.spec.ts` with four fullstack journeys.
Each journey drives the browser through a real flow and asserts the
backend state via the JSON API — outcome verification, not page presence.

| Journey | What it asserts |
|---|---|
| Member checkout → payment → order detail | UI: login → product → add to cart → cart → checkout (with address selection) → orders. Backend: `GET /api/v1/orders` returns the new order with valid `id`, `order_number`, `status`. UI cross-surface: detail page shows the order_number. |
| Staff shipment full lifecycle | API drives: cart → checkout → pay → HMAC-signed payment callback (real HMAC computed in Node `crypto`) → order paid. Staff API: create shipment → status → pod. Cross-surface readback: shipment list contains the new shipment with `status=packed`. |
| Moderation report → cases | Member creates a post, reports it. Moderator API: `GET /moderation/reports` includes the new report (matched by `post_id`). |
| Admin import lifecycle | Multipart upload → `GET /imports/:id` shows `uploaded` → `POST /:id/validate` → status advances → list contains the import. |

Two pre-existing E2E gaps were also fixed along the way:

| Gap | Fix |
|---|---|
| `docker-compose.e2e.yml` was missing `PAYMENT_MERCHANT_KEY`, so the app crashed on startup. | Added the env var. |
| `run_e2e.sh` would falsely report PASS if the app crashed before tests ran (because `--exit-code-from e2e` returned 0 for a never-started container). | Defensive check: inspect app container exit code; treat any non-teardown failure (i.e. not 137 / 143) as a run failure. |

---

## 5. Coverage drift guard (preventive)

To keep the inventory + after-coverage docs from drifting from the
router on future PRs, a static guard runs as the **first step** of
`run_all_tests.sh`.

| Path | Role |
|---|---|
| `scripts/check_api_coverage_drift.sh` | Shell entry point. |
| `scripts/check_api_coverage_drift.py` | Parses `internal/router/router.go`, `docs/api-endpoints-inventory.md`, `docs/api-coverage-after.md`. Fails if any pair disagrees. |

Currently outputs:

```
=== API Coverage Drift Check ===
  router.go              : 79 endpoints
  inventory doc          : 79 endpoints
  after-coverage mapping : 79 endpoints

OK: router.go == inventory doc
OK: router.go == after-coverage mapping
OK: inventory == after-coverage mapping

  NO COVERAGE DRIFT — all sources agree.
```

---

## 6. Constraints honored

| Constraint | Honored? | How |
|---|---|---|
| Prefer real HTTP boundary tests | Yes | Every new external test in §2 uses real `http.Client` over TCP via `EXTERNAL_API_BASE_URL`. No `httptest.NewRecorder`. No HTTP-layer mocks. |
| Do not add mock-heavy substitutes for core API paths | Yes | Static grep confirms zero gomock / testify-mock / sinon usage in test code. |
| Keep changes deterministic and CI-friendly | Yes | New unit tests use seeded UUIDs and fixed inputs. New E2E journeys assert via API readback rather than fragile UI text. |
| Do not weaken existing coverage | Yes | All 79 endpoints still mapped in `docs/api-coverage-after.md`. Drift guard enforces this. |

---

## 7. Acceptance-criteria checklist

| # | Criterion | Where it is satisfied |
|---|---|---|
| 1 | README hard-gate failures resolved | `README.md` (literal `docker-compose up` on line 19; no local-dev path); `docs/local_development.md` (off-path warning). |
| 2 | Endpoint coverage remains complete and depth improved for previously shallow routes | `docs/api-coverage-after.md` (79/79) + `tests/external_api/behavior_admin_test.go` + `tests/external_api/behavior_commerce_test.go` (28 new behavior tests); drift guard enforces. |
| 3 | At least 3 stronger end-to-end flows asserting outcomes | `e2e/tests/journeys.spec.ts` (4 journeys: checkout-to-payment, shipment lifecycle, moderation report, admin import) — each asserts both UI and backend state. |
| 4 | Critical service/business logic has additional branch-focused unit tests | `internal/service/transitions_test.go` (ticket + shipment state machines, branch-complete); `internal/service/cohort_test.go`; `internal/service/payment_signature_test.go`; `internal/middleware/rbac_test.go`. |
| 5 | `run_all_tests.sh` exists and is documented as primary validation | `run_all_tests.sh` (orchestrates 4 suites with fail-fast); documented in `README.md` "Run every suite in one command (primary release-gate command)" and `docs/testing.md`. |
| 6 | `docs/audit_remediation.md` provides traceable evidence | This file. Each issue/gap → exact file + test → criterion satisfied. |
