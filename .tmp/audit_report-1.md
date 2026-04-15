# Static Audit Report - Delivery Acceptance & Project Architecture (CampusRec Prompt)

## 1. Verdict
- **Overall conclusion: Partial Pass**
- The repository is materially aligned with the CampusRec domain and has substantial implementation coverage, but several prompt-critical requirements remain incomplete or statically contradicted (notably async/offline import rigor, leave/seat lifecycle correctness, nightly encrypted backup/PITR depth, canary rollout enforcement, and member web payment lifecycle completeness).

## 2. Scope and Static Verification Boundary
- **Reviewed:** README/docs, routing and entrypoint wiring, auth/RBAC middleware, core services/repos/models/migrations, API/web handlers, and test suites under `tests/`.
- **Not reviewed:** runtime behavior in a live environment, browser rendering behavior, actual scheduler timing under load, cryptographic restore correctness, and database performance/SLO behavior.
- **Intentionally not executed:** app startup, Docker, tests, migrations, external services.
- **Manual verification required:** p95 latency target, real QR/payment callback operational flow, true backup/restore artifact integrity, and time-based scheduler SLA behavior.

## 3. Repository / Requirement Mapping Summary
- **Prompt core goal:** private-network CampusRec platform for session registration + seat control + offline commerce lifecycle with WeChat callback verification, ticket/SLA operations, moderation, import/export, canary config, and archival/backup controls.
- **Mapped implementation areas:** Gin REST APIs + Templ web UI, PostgreSQL schema/migrations for registrations/attendance/commerce/logistics/moderation/tickets/import-export/backup, role-based auth (member/staff/moderator/administrator), scheduler jobs, and integration tests.
- **High-level fit:** strong domain overlap and broad module presence, with critical execution gaps in several required workflows.

## 4. Section-by-section Review

### 1. Hard Gates

#### 1.1 Documentation and static verifiability
- **Conclusion: Pass**
- **Rationale:** clear startup, test, endpoint, and structure documentation with statically consistent entrypoints/config.
- **Evidence:** `README.md:13`, `README.md:35`, `README.md:61`, `README.md:116`, `cmd/server/main.go:23`, `config/config.go:59`

#### 1.2 Material deviation from Prompt
- **Conclusion: Partial Pass**
- **Rationale:** overall product direction matches the prompt (CampusRec seat + commerce + moderation + admin), but multiple required behaviors are weakened or missing in implementation details.
- **Evidence:** `README.md:3`, `internal/router/router.go:132`, `internal/router/router.go:152`, `internal/router/router.go:178`, `internal/router/router.go:221`, `internal/router/router.go:240`

### 2. Delivery Completeness

#### 2.1 Coverage of explicit core requirements
- **Conclusion: Partial Pass**
- **Rationale:** many required modules exist (auth/RBAC, seat inventory, waitlist, payment callback signature, logistics POD/exception, moderation, tickets, imports/exports, feature flags), but critical required semantics are incomplete.
- **Evidence:** `internal/service/auth_service.go:16`, `internal/repo/registration_repo.go:25`, `internal/service/payment_service.go:53`, `internal/service/shipment_service.go:126`, `internal/service/moderation_service.go:35`, `internal/service/ticket_service.go:268`, `internal/service/import_service.go:169`, `internal/service/feature_flag_service.go:57`
- **Manual verification note:** timing guarantees (e.g., 30s waitlist promotion, 15m expiry execution) are runtime-dependent.

#### 2.2 End-to-end 0-to-1 deliverable vs partial/demo
- **Conclusion: Partial Pass**
- **Rationale:** repository is product-shaped and not a toy sample, but user-facing order/payment and registration web lifecycle is statically incomplete.
- **Evidence:** `README.md:116`, `internal/service/services.go:9`, `web/templates/pages/orders.templ:42`, `web/templates/pages/registrations.templ:48`, `internal/router/router.go:256`, `internal/router/router.go:257`

### 3. Engineering and Architecture Quality

#### 3.1 Structure and module decomposition
- **Conclusion: Pass**
- **Rationale:** clean layered decomposition (handler/service/repo/model/middleware/migrations) with domain separation.
- **Evidence:** `README.md:118`, `internal/service/services.go:9`, `internal/router/router.go:34`, `db/migrations/00012_create_commerce.sql:31`, `db/migrations/00010_create_attendance.sql:4`

#### 3.2 Maintainability and extensibility
- **Conclusion: Partial Pass**
- **Rationale:** architecture is maintainable, but key policy/config hooks are seeded but not enforced in core flows, reducing practical extensibility.
- **Evidence:** `db/migrations/00007_create_config.sql:19`, `internal/service/registration_service.go:33`, `internal/service/registration_service.go:36`, `internal/service/feature_flag_service.go:59`, `internal/service/feature_flag_service.go:89`

### 4. Engineering Details and Professionalism

#### 4.1 Error handling, logging, validation, API design
- **Conclusion: Partial Pass**
- **Rationale:** response envelope and validation patterns are consistent, but there are notable correctness defects and logging risks.
- **Evidence:** `internal/response/envelope.go:12`, `internal/response/envelope.go:107`, `internal/service/import_service.go:195`, `internal/service/import_service.go:197`, `internal/service/attendance_service.go:168`, `db/migrations/00010_create_attendance.sql:38`, `internal/middleware/recovery.go:17`

#### 4.2 Product/service realism vs demo level
- **Conclusion: Partial Pass**
- **Rationale:** substantial real service scaffolding exists, but some required flows still behave as placeholders or metadata-only lifecycle records.
- **Evidence:** `internal/service/backup_service.go:55`, `internal/service/backup_service.go:58`, `internal/service/backup_service.go:85`, `internal/service/import_service.go:370`

### 5. Prompt Understanding and Requirement Fit

#### 5.1 Business goal and implicit constraint fit
- **Conclusion: Partial Pass**
- **Rationale:** prompt intent is understood, but important semantics are under-delivered: strict import validation path, leave-seat release rule, canary rollout usage, nightly encrypted backup/PITR, and KPI churn dimension.
- **Evidence:** `internal/service/import_service.go:194`, `internal/service/import_service.go:197`, `internal/service/attendance_service.go:229`, `internal/repo/attendance_repo.go:80`, `internal/service/feature_flag_service.go:59`, `internal/service/dashboard_service.go:19`, `cmd/server/main.go:67`

### 6. Aesthetics (frontend/full-stack)
- **Conclusion: Partial Pass**
- **Rationale:** UI has responsive catalog/cards/badges and clear registration status labeling, but key UX requirement (15-minute payment countdown + complete order-detail flow) is incomplete.
- **Evidence:** `web/templates/pages/catalog.templ:75`, `web/templates/pages/catalog.templ:132`, `web/templates/pages/registrations.templ:10`, `web/templates/pages/orders.templ:88`, `internal/router/router.go:256`, `web/static/css/main.css:140`
- **Manual verification note:** final rendering quality and mobile behavior require manual browser validation.

## 5. Issues / Suggestions (Severity-Rated)

### Blocker / High

1) **Severity:** High  
**Title:** Temporary leave flow is internally inconsistent with DB contract and required seat-release behavior  
**Conclusion:** Fail  
**Evidence:** `internal/service/attendance_service.go:168`, `internal/repo/attendance_repo.go:65`, `db/migrations/00010_create_attendance.sql:38`, `internal/service/attendance_service.go:229`, `internal/repo/attendance_repo.go:80`  
**Impact:** leave creation can fail due invalid `occupancy_id`; leave-return path never computes overstay/seat release as required.  
**Minimum actionable fix:** resolve active occupancy ID before leave insert; compute exceeded duration on return; on breach, release seat + update registration/occupancy + create exception ticket.

2) **Severity:** High  
**Title:** Import validation can be bypassed when file is unavailable on disk  
**Conclusion:** Fail  
**Evidence:** `internal/handler/api/import_handler.go:36`, `internal/handler/api/import_handler.go:48`, `internal/service/import_service.go:194`, `internal/service/import_service.go:197`  
**Impact:** strict offline CSV/XLSX validation promise is broken; jobs can be marked validated without parsing uploaded content.  
**Minimum actionable fix:** persist uploaded file artifact before creating job, fail validation if artifact is missing/unreadable, and require real parsed-row validation before apply.

3) **Severity:** High  
**Title:** Nightly encrypted backup + PITR requirements are not fully implemented  
**Conclusion:** Partial Fail  
**Evidence:** `internal/service/backup_service.go:55`, `internal/service/backup_service.go:58`, `config/config.go:44`, `config/config.go:100`, `internal/service/services.go:48`, `internal/service/backup_service.go:85`, `cmd/server/main.go:67`, `db/migrations/00011_create_job_queue.sql:65`  
**Impact:** backup/restore controls are largely metadata lifecycle without verifiable encryption/PITR behavior; nightly scheduling defined in DB is not wired in runtime scheduler.  
**Minimum actionable fix:** implement real artifact creation/encryption using configured key, operational restore/PITR steps, and schedule nightly backup/archive jobs in runtime.

4) **Severity:** High  
**Title:** Member web order/payment lifecycle is incomplete (detail route/cancel flow mismatch)  
**Conclusion:** Fail  
**Evidence:** `web/templates/pages/orders.templ:42`, `web/templates/pages/orders.templ:85`, `web/templates/pages/registrations.templ:48`, `internal/router/router.go:256`, `internal/router/router.go:257`, `internal/router/router.go:258`, `internal/handler/web/handler.go:198`  
**Impact:** required member UX for paying awaiting orders and registration lifecycle actions is partially unreachable via documented web routes.  
**Minimum actionable fix:** add routed handlers for `/my/orders/:id` and registration cancel/operations endpoints used by templates; include countdown/next-step UX on expiry.

5) **Severity:** High  
**Title:** Canary release exists in storage/API but is not used to gate any runtime behavior  
**Conclusion:** Partial Fail  
**Evidence:** `internal/service/feature_flag_service.go:59`, `internal/service/feature_flag_service.go:89`, `internal/router/router.go:95`, `internal/router/router.go:271`  
**Impact:** canary by user cohort percentage is not operationally effective; feature flags are only managed/listed.  
**Minimum actionable fix:** integrate flag evaluation at target feature decision points (especially staff-facing flows) and add audit/tests for cohort rollout behavior.

6) **Severity:** High  
**Title:** Registration close-policy requirement (2h-before-start default with admin override) is not enforced from config/policy center  
**Conclusion:** Partial Fail  
**Evidence:** `db/migrations/00007_create_config.sql:19`, `internal/service/registration_service.go:33`, `internal/service/registration_service.go:36`, `internal/service/config_service.go:28`  
**Impact:** central policy exists but is not used in registration decisioning; behavior depends only on per-session timestamps.  
**Minimum actionable fix:** enforce default close-hours policy in registration service and support explicit admin override path with audit trail.

### Medium

7) **Severity:** Medium  
**Title:** Role literal mismatches can break admin/staff authorization in service-level checks  
**Conclusion:** Partial Fail  
**Evidence:** `internal/model/role.go:13`, `internal/service/registration_service.go:284`, `internal/service/ticket_service.go:321`  
**Impact:** users with `administrator` role may be denied checks that look for `admin`.  
**Minimum actionable fix:** replace string literals with role constants everywhere.

8) **Severity:** Medium  
**Title:** KPI set does not include explicit churn metric required by prompt  
**Conclusion:** Partial Fail  
**Evidence:** `internal/service/dashboard_service.go:19`, `internal/service/dashboard_service.go:26`, `internal/service/dashboard_service.go:27`  
**Impact:** admin dashboard does not fully satisfy required operations-review metric set.  
**Minimum actionable fix:** add churn KPI computation and include it in API and dashboard rendering/tests.

9) **Severity:** Medium  
**Title:** Logging strategy is mostly unstructured and can expose internals in panic paths  
**Conclusion:** Suspected Risk  
**Evidence:** `internal/middleware/recovery.go:17`, `cmd/server/main.go:30`, `internal/scheduler/scheduler.go:62`  
**Impact:** operational troubleshooting and sensitive-data controls may be weaker than required observability expectations.  
**Minimum actionable fix:** migrate to structured logging with redaction policy; limit stack traces in production responses/log streams.

## 6. Security Review Summary

- **Authentication entry points:** **Pass**  
  Evidence: `internal/router/router.go:57`, `internal/handler/api/auth_handler.go:56`, `internal/service/auth_service.go:129`  
  Reasoning: local username/password login, lockout, session creation, and cookie auth middleware are present.

- **Route-level authorization:** **Pass**  
  Evidence: `internal/router/router.go:65`, `internal/router/router.go:91`, `internal/router/router.go:198`, `internal/middleware/rbac.go:23`  
  Reasoning: privileged route groups consistently guarded by auth/role middleware.

- **Object-level authorization:** **Partial Pass**  
  Evidence: `internal/repo/address_repo.go:54`, `internal/service/order_service.go:342`, `internal/service/registration_service.go:138`, `internal/service/ticket_service.go:316`  
  Reasoning: ownership checks exist across major entities, but role-literal bugs weaken some elevated paths.

- **Function-level authorization:** **Partial Pass**  
  Evidence: `internal/router/router.go:139`, `internal/router/router.go:146`, `internal/router/router.go:215`, `internal/service/ticket_service.go:321`  
  Reasoning: function access generally constrained by roles, but inconsistent role naming introduces defects.

- **Tenant / user data isolation:** **Cannot Confirm Statistically**  
  Evidence: `db/migrations/00001_create_users.sql:4`, `internal/repo/order_repo.go:421`, `internal/repo/address_repo.go:28`  
  Reasoning: user-level isolation exists; multi-tenant boundary model is not explicitly defined.

- **Admin / internal / debug protection:** **Pass**  
  Evidence: `internal/router/router.go:91`, `internal/router/router.go:267`  
  Reasoning: admin APIs/pages are role-gated; no open debug endpoints observed in router.

## 7. Tests and Logging Review

- **Unit tests:** **Partial Pass**  
  Evidence: `internal/validator/validator_test.go:7`, `internal/util/password_test.go:8`  
  Reasoning: core utility validation tested; domain-heavy business rules less covered at unit granularity.

- **API / integration tests:** **Partial Pass**  
  Evidence: `tests/integration/auth_test.go:22`, `tests/integration/rbac_test.go:12`, `tests/integration/audit_fixes_test.go:17`, `tests/integration/round3_fixes_test.go:18`  
  Reasoning: broad endpoint coverage exists, but several prompt-critical failure modes remain untested or only weakly asserted.

- **Logging categories / observability:** **Partial Pass**  
  Evidence: `internal/middleware/audit.go:11`, `internal/service/audit_service.go:20`, `internal/scheduler/scheduler.go:37`  
  Reasoning: audit and scheduler logs exist, but logs are largely plain text and not fully structured KPI/ops telemetry.

- **Sensitive-data leakage risk in logs / responses:** **Partial Pass (Suspected Risk)**  
  Evidence: `internal/middleware/recovery.go:17`, `internal/handler/api/order_handler.go:50`, `tests/integration/round3_fixes_test.go:233`  
  Reasoning: tests check some leakage paths, but panic stack logging and direct error passthrough patterns still present risk.

## 8. Test Coverage Assessment (Static Audit)

### 8.1 Test Overview
- **Unit tests exist:** yes.
- **API/integration tests exist:** yes (integration + blackbox + e2e folder present).
- **Frameworks:** Go `testing` + `httptest`; Playwright for e2e scripts.
- **Test entry points:** `tests/testutil.SetupTestRouter` for integration; shell scripts and e2e package docs in repo.
- **Documentation provides test commands:** yes.
- **Evidence:** `tests/testutil/helpers.go:153`, `tests/integration/auth_test.go:22`, `tests/integration/api_coverage_test.go:66`, `tests/blackbox/blackbox_test.go:1`, `README.md:35`, `README.md:45`, `run_tests.sh:38`

### 8.2 Coverage Mapping Table

| Requirement / Risk Point | Mapped Test Case(s) | Key Assertion / Fixture / Mock | Coverage Assessment | Gap | Minimum Test Addition |
|---|---|---|---|---|---|
| Auth policy (12+ chars, lockout) | `tests/integration/auth_test.go:96`, `tests/integration/auth_test.go:132` | weak password rejected; lockout on repeated failures | basically covered | lockout expiry window not asserted | add time-window test for unlock after 15 min |
| 401/403 route controls | `tests/integration/rbac_test.go:12`, `tests/integration/rbac_test.go:36` | admin endpoints return 403/401 by role/auth state | sufficient | limited to selected endpoints | expand to shipment/moderation/ticket privileged mutations |
| Object-level ownership | `tests/integration/audit_fixes_test.go:17`, `tests/integration/audit_fixes_test.go:77` | cross-user order/registration access must be 403 | sufficient | no coverage for admin-role edge cases (`administrator` literal bug) | add tests with seeded admin account for service-level checks |
| Seat oversell/atomic reservation | no direct concurrent test found | n/a | insufficient | transaction logic exists but concurrency not tested | add parallel registration race test for single-seat session |
| Waitlist promotion timing | `tests/integration/round3_fixes_test.go:175` | only method-call/no-panic | insufficient | no assertion of 30s promotion SLA or actual promotion | add integration test that frees seat and verifies promotion within window |
| Temporary leave rules + seat release on breach | `tests/integration/round2_fixes_test.go:85` | only invalid-registration failure path | missing | no test for occupancy linkage, exceed handling, or seat release | add full check-in -> leave -> overstay -> seat release/exception tests |
| Payment callback signature/idempotency | `tests/integration/api_coverage_test.go:680`, `tests/integration/moderation_test.go:90` | mostly "not 500" and safe error checks | insufficient | no strong positive callback flow with valid signature + duplicate tx replay | add deterministic signed callback tests including replay idempotency |
| Import strict validation/fingerprinting | `tests/integration/round3_fixes_test.go:18`, `tests/integration/round3_fixes_test.go:108` | status checks + duplicate checksum rejection | insufficient | no test for missing-on-disk artifact bypass path; no XLSX path | add test asserting validation fails when storage file absent and add XLSX parser tests |
| Backup/archive/PITR depth | `tests/integration/round2_fixes_test.go:155`, `tests/integration/round2_fixes_test.go:189` | metadata/status checks | insufficient | no cryptographic artifact/restore/PITR validation; no nightly scheduling assertion | add backup artifact integrity + restore drill + scheduler wiring tests |
| KPI completeness (incl churn) | `tests/integration/round3_fixes_test.go:131` | asserts several KPI fields | insufficient | churn metric absent from API and tests | add churn KPI field and tests for month-over-month calculation |
| Member web order/payment lifecycle | `tests/integration/round3_fixes_test.go:185`, `tests/integration/round3_fixes_test.go:196` | route-level 200 checks on list pages | insufficient | no tests for `/my/orders/:id`, payment countdown view, registration cancel web action | add web route tests for detail + cancel form action wiring |

### 8.3 Security Coverage Audit
- **Authentication:** basically covered (register/login/logout/lockout tests exist). Evidence: `tests/integration/auth_test.go:22`, `tests/integration/auth_test.go:176`.
- **Route authorization:** basically covered for key admin/moderation paths. Evidence: `tests/integration/rbac_test.go:12`, `tests/integration/moderation_test.go:34`.
- **Object-level authorization:** moderately covered for orders/registrations/tickets. Evidence: `tests/integration/audit_fixes_test.go:17`, `tests/integration/audit_fixes_test.go:116`.
- **Tenant/data isolation:** insufficient; no explicit multi-tenant scenarios or cross-tenant fixtures.
- **Admin/internal protection:** partially covered at endpoint level, but service-level role-literal inconsistencies remain weakly tested.

### 8.4 Final Coverage Judgment
- **Fail**
- Tests cover many happy-path and basic authz cases, but major prompt-critical risks remain under-tested (concurrency/seat guarantees, leave-breach behavior, strict offline import integrity, backup/PITR realism, and full member payment web lifecycle). Severe defects can still remain undetected while tests pass.

## 9. Final Notes
- This report is static-only; no runtime success was inferred.
- Main weaknesses are concentrated in execution integrity of time-based/offline workflows rather than pure endpoint availability.
- Prioritize remediation in this order: leave/seat correctness, import strictness, backup/PITR/nightly ops, canary enforcement wiring, and member web payment lifecycle completion.
