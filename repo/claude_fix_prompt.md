# Prompt for Claude: Raise Audit Score to >=90/100

You are fixing this repository to raise the combined **Test Coverage + README audit score** to **>=90/100**.

Use the audit report as source of truth:

- `/tmp/test_coverage_and_readme_audit_report.md`
- `/home/nico/.tmp/test_coverage_and_readme_audit_report.md`

## Goal

Ship concrete repo changes that eliminate hard-gate failures and close key testing gaps, with clear evidence in docs.

---

## Hard Gates (must pass)

### README hard gates

1. `README.md` must satisfy backend/fullstack startup instruction requirement with explicit command:
   - include `docker-compose up` (literal token)
   - keep modern `docker compose up --build` if desired, but include the literal required form.

2. Strict environment policy:
   - remove or isolate non-Docker local setup that implies manual DB/bootstrap is part of main workflow.
   - README main path must be fully Docker-contained.

3. Keep access method and verification:
   - URL + port
   - a short deterministic smoke-check section.

4. Keep demo credentials for all roles.

### Test hard gates

1. Preserve full endpoint inventory coverage at API boundary.
2. Do not replace real HTTP tests with mocks.
3. Keep Docker-based test execution as default.

---

## Failed Issues to Fix (from audit)

1. **README Environment Rules FAIL**
   - `README.md` includes local dev/manual DB setup flow (`go run`, manual migrate/seed) as active instructions.
   - Fix: move local dev path to a clearly labeled optional appendix or separate doc not part of required run path.

2. **README Startup command literal gate FAIL**
   - README currently uses `docker compose up --build` but missing literal `docker-compose up` token.
   - Fix: include the exact literal command in startup section (can note equivalent modern variant).

---

## Key Gaps to Close (from audit)

1. **API depth gap**
   - Some endpoints are covered mainly by reachability matrix tests.
   - Add deeper behavior assertions for high-risk endpoints: payload semantics, state transitions, and failure paths.

2. **Unit depth gap**
   - Limited isolated tests for service/repository decision logic.
   - Add focused unit tests for critical business logic branches (orders, registration transitions, backup/restore validation, moderation/ticket transitions as feasible without heavy mocks).

3. **E2E depth gap**
   - Browser E2E is present but shallow for business-critical lifecycle flows.
   - Add robust end-to-end user journeys across FE->BE->DB for at least:
     - cart -> checkout -> payment -> order status confirmation
     - moderation/report action flow
     - staff shipment/ticket critical transition path

4. **Command fragmentation gap**
   - Tests are split across `run_tests.sh`, `run_external_api_tests.sh`, `run_e2e.sh`.
   - Add one top-level orchestrator (e.g., `run_all_tests.sh`) for confidence and CI usage.

---

## Required Deliverables

1. `README.md` updates that satisfy all hard gates.
2. New/updated tests addressing API depth, unit depth, and E2E depth gaps.
3. New `run_all_tests.sh` that runs all critical suites in Docker-oriented flow.
4. A short evidence doc (e.g., `docs/audit_remediation.md`) listing:
   - each issue/gap
   - exact files/functions changed
   - how each fix addresses audit criteria.

---

## Constraints

- Prefer real HTTP boundary tests for API behavior.
- Do not add mock-heavy substitutes for core API paths.
- Keep changes deterministic and CI-friendly.
- Do not weaken existing coverage.

---

## Acceptance Criteria (definition of done)

1. README hard-gate failures are resolved.
2. Endpoint coverage remains complete and behavior depth is improved for previously shallow routes.
3. At least 3 stronger end-to-end flows exist and assert outcomes, not just page presence.
4. Critical service/business logic has additional branch-focused unit tests.
5. `run_all_tests.sh` exists and is documented as the primary validation command.
6. `docs/audit_remediation.md` provides traceable evidence for all fixes.

When done, provide a concise checklist mapping each acceptance criterion to files and test cases.
