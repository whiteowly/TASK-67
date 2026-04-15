# Raise-to-PASS Checklist

- Replace liveness-only matrix checks with contract assertions for high-risk endpoints: for each of `POST /api/v1/checkout`, `POST /api/v1/payments/callback`, `POST /api/v1/admin/restore`, `POST /api/v1/imports/:id/apply`, assert exact status, `success`, error code (when failing), and 2-3 critical response fields in `tests/external_api/coverage_matrix_test.go`.
- Eliminate "not 500" assertions in `tests/integration/api_coverage_test.go`: convert to deterministic expectations (`400/401/403/404/409/422`) based on scenario, and assert response envelope keys (`success`, `error.code`, `error.message`).
- Reduce skip-driven blind spots: replace `t.Skip` precondition branches with deterministic fixture creation helpers (create required user/session/order inside the test) so tests always execute core assertions.
- Add focused unit tests for handler error mapping in `internal/handler/api/**`: verify service-domain errors map to correct HTTP codes/envelopes (especially authz, validation, conflict, not-found).
- Add repo contract tests for `internal/repo/**` critical paths (idempotency, ownership filtering, transition constraints): assert exact DB-side behavior and returned domain errors, not just non-nil/non-error.
