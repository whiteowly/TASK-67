# Prompt: Enforce 100% External API Endpoint Coverage + Questions Doc Cleanup

You are working in this repository to eliminate API coverage and evidence gaps and to clean requirement clarification docs.

## Hard Requirements

- Backend has **63 API endpoints**.
- External API test coverage must be **100%** (not 90%).
- Coverage evidence must be based on **external black-box HTTP** tests.
- In-process tests (`httptest.NewRecorder`, `r.ServeHTTP`) may remain, but they **do not count** toward required coverage.
- `docs/questions.md` must contain only real prompt ambiguities; no extra implementation scope/design additions.

## Tasks

### 1) Build definitive endpoint inventory

- Extract the canonical list of all 63 endpoints from router definitions.
- Normalize each endpoint as: `METHOD PATH`.
- Save output to `docs/api-endpoints-inventory.md`.

### 2) Build current coverage map

- Map existing tests to endpoints.
- Separate evidence into:
  - **External HTTP tests** (counts toward requirement),
  - **In-process tests** (informational only).
- Save to `docs/api-coverage-before.md` with:
  - covered/uncovered endpoint tables,
  - exact percentage,
  - explicit list of missing endpoints.

### 3) Implement external black-box API tests for all missing endpoints

- Add tests so **all 63 endpoints** are directly covered by external HTTP calls.
- Tests must hit a real running app instance (Docker/compose or equivalent).
- No HTTP-layer mocks.
- Validate at minimum for each endpoint:
  - status code,
  - response body contract/shape,
  - auth/role behavior where applicable,
  - expected failure mode for invalid or unauthorized requests (where relevant).
- Place tests under a clear external suite path (e.g., `tests/external_api/`).

### 4) Make test execution CI-ready

- Add/update a single command to run external API suite end-to-end.
- Ensure deterministic setup/teardown and isolated data.
- Ensure command is documented in `README.md` (or `docs/testing.md`).

### 5) Produce after-state evidence

Create `docs/api-coverage-after.md` containing:

- full endpoint inventory (63/63),
- per-endpoint mapping to external test case(s),
- command(s) used,
- execution evidence excerpts,
- final statement: **External endpoint coverage = 100%**.

### 6) Fix `docs/questions.md`

- Rewrite `docs/questions.md` so every entry is a real ambiguity from original prompt requirements.
- Remove scope/design additions not asked by the prompt.
- Move non-prompt implementation choices to `docs/assumptions.md` under:
  - `## Assumptions (Not Explicit Prompt Requirements)`.

## Merge Gate (must pass)

Do **not** finish until all of these are true:

1. `docs/api-endpoints-inventory.md` lists 63 unique endpoints.
2. `docs/api-coverage-after.md` proves **63/63 externally covered**.
3. Every endpoint has at least one direct external HTTP test.
4. `docs/questions.md` contains only requirement ambiguities.
5. External API test command passes in CI-like environment.
6. No endpoint is “implicitly covered”; each must have explicit test mapping.

## Forbidden shortcuts

- No counting in-process tests as coverage evidence.
- No placeholder tests.
- No “best effort” exemptions.
- No undocumented assumptions in `docs/questions.md`.

## Deliverables summary

- `docs/api-endpoints-inventory.md`
- `docs/api-coverage-before.md`
- external API tests for all endpoints
- updated test runner/docs
- `docs/api-coverage-after.md`
- cleaned `docs/questions.md`
- `docs/assumptions.md` (if needed)
