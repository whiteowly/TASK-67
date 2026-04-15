# Audit Report 2 - Fix Check

## Final status of the 6 issues

1. **Member commerce Templ flow incomplete** — **Fixed**
   - Added authenticated cart and checkout pages/routes/handlers, including address selection in checkout flow.
   - Evidence: `internal/router/router.go:262`, `internal/handler/web/handler.go:377`, `web/templates/pages/cart.templ:25`, `web/templates/pages/checkout.templ:19`.

2. **Weak critical-path test assertions** — **Fixed**
   - Strengthened tests to assert strict denial and invariant behavior (cross-user order access, archive move semantics, waitlist/seat invariants).
   - Evidence: `tests/integration/round5_fixes_test.go:44`, `tests/integration/round5_fixes_test.go:311`, `tests/integration/round5_fixes_test.go:478`.

3. **Error detail exposure via raw `err.Error()`** — **Fixed (materially)**
   - Service failures now flow through centralized safe mapper with generic fallback (`operation failed`).
   - Evidence: `internal/handler/api/errmap.go:54`, `internal/handler/api/errmap.go:59`, `internal/handler/api/order_handler.go:50`.
   - Note: request binding/validation paths still return validation text, which is typically acceptable.

4. **Job status mismatch (`queued` vs `pending`)** — **Fixed**
   - Dashboard queued metric now queries `pending` queue status.
   - Evidence: `internal/service/dashboard_service.go:111`.

5. **Attendance check-in method constraints too permissive** — **Fixed**
   - Allowed methods restricted to `qr_staff` and `beacon`; beacon-required policy enforcement remains active.
   - Added deterministic policy tests for required-beacon and non-beacon scenarios.
   - Evidence: `internal/service/attendance_service.go:32`, `internal/service/attendance_service.go:81`, `tests/integration/round5_fixes_test.go:769`, `tests/integration/round5_fixes_test.go:854`.

6. **Registration list showing session UUID instead of title** — **Fixed**
   - Registration list now enriches and renders session titles with fallback behavior.
   - Evidence: `internal/handler/web/handler.go:254`, `web/templates/pages/registrations.templ:32`, `web/templates/pages/registrations.templ:50`.

## Overall conclusion

All 6 previously tracked issues are now addressed based on static code/test review.

Boundary note: this check is static-only; no runtime execution or test run was performed in this verification pass.
