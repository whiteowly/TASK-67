# Test Gaps And Fix Plan

## 1) Fragmented confidence path (`run_tests.sh` does not run all critical suites)

### Gap
- `run_tests.sh` runs Go tests in Docker, but does not include:
  - `./run_external_api_tests.sh` (external API boundary suite)
  - `./run_e2e.sh` (browser E2E)
- This weakens one-command release confidence.

### Fix
- Add a top-level orchestrator script (e.g., `run_all_tests.sh`) that runs, in order:
  1. `./run_tests.sh`
  2. `./run_external_api_tests.sh`
  3. `./run_e2e.sh`
- Fail fast on first non-zero exit and print a final combined summary.
- Update `README.md` to make this the default CI/local validation command.

## 2) External API matrix is broad but partially shallow

### Gap
- `tests/external_api/coverage_matrix_test.go` provides strong reachability, but many endpoints are validated minimally there.
- Some endpoints may not have deep behavior assertions (payload invariants, side effects, state transitions).

### Fix
- Keep matrix as a routing guard, but add/expand behavior-focused tests per domain:
  - **Admin**: config version conflict, audit log filtering, backup/restore state transitions
  - **Commerce**: checkout idempotency, stock decrement consistency, payment callback replay handling
  - **Moderation/Tickets**: lifecycle transitions, invalid transition rejection
  - **Imports/Exports**: invalid template handling, apply/rollback semantics, download permissions
- Require each endpoint to have at least one test with:
  - concrete payload assertions,
  - at least one failure-path assertion,
  - RBAC assertion where applicable.

## 3) Browser E2E coverage depth is limited

### Gap
- Playwright tests cover auth/catalog/admin basics, but deeper fullstack user journeys are thin.

### Fix
- Add end-to-end scenarios that span UI + backend + persistence:
  1. Member checkout from cart -> payment -> order detail status update
  2. Staff shipment lifecycle -> proof/exception -> member readback
  3. Moderation report -> case action -> post visibility/result verification
  4. Admin import -> validate -> apply -> data visible in UI/API
- Assert visible UI outcomes and resulting backend state (via API/UI readback), not only page presence.

## 4) Unit-test depth for service/business logic is thin

### Gap
- Unit tests are concentrated in utility/validator packages; core service decision logic appears less isolatedly tested.

### Fix
- Add targeted unit tests for pure business rules and edge-case logic in services/repositories, such as:
  - registration eligibility and waitlist transitions,
  - payment/order state transitions,
  - backup/restore validation gates,
  - RBAC decision helpers.
- Prioritize deterministic, branch-complete tests for high-risk decision code.

## 5) Missing explicit quality gates for coverage evidence drift

### Gap
- Coverage evidence docs exist, but there is no visible static guard to prevent endpoint/test mapping drift.

### Fix
- Add a lightweight check script to verify endpoint inventory vs documented mapping files:
  - `docs/api-endpoints-inventory.md`
  - `docs/api-coverage-after.md`
- Run it in CI before test execution; fail if counts/mappings diverge.

## Recommended implementation order

1. Add `run_all_tests.sh` and wire CI to it.
2. Deepen external API behavior tests for highest-risk domains.
3. Expand Playwright true fullstack flows.
4. Add service-layer unit tests for decision-heavy logic.
5. Add coverage drift guard script for endpoint mapping consistency.
