# Audit Report — Fix Check (9 Issues)

Static verification only (no runtime execution, no tests run).

## Issue Status Summary

1. **Leave flow correctness + seat release enforcement** — **Fixed**
   - Active occupancy is required before leave; exceeded leave triggers occupancy end, seat release, exception, and ticket flow.
   - Evidence: `internal/service/attendance_service.go:149`, `internal/service/attendance_service.go:241`, `internal/service/attendance_service.go:249`, `internal/service/attendance_service.go:269`, `internal/service/attendance_service.go:282`

2. **Strict import validation (no metadata-only pass)** — **Fixed**
   - Upload persists file bytes; validation fails if artifact file is missing.
   - Evidence: `internal/handler/api/import_handler.go:51`, `internal/handler/api/import_handler.go:56`, `internal/service/import_service.go:203`, `internal/service/import_service.go:207`

3. **Backup/PITR operational restore path** — **Fixed**
   - Restore now executes real transactional delete/load logic with PITR cutoff filtering and execution stats.
   - Evidence: `internal/service/backup_service.go:572`, `internal/service/backup_service.go:596`, `internal/service/backup_service.go:610`, `internal/service/backup_service.go:653`, `internal/service/backup_service.go:658`

4. **Member web order/payment lifecycle route completeness** — **Fixed**
   - Missing member routes/handlers are wired (`/my/orders/:id`, registration cancel action).
   - Evidence: `internal/router/router.go:257`, `internal/router/router.go:259`, `internal/handler/web/handler.go:208`, `internal/handler/web/handler.go:231`

5. **Canary feature flags enforced at runtime** — **Fixed**
   - Feature flag checks are used in service decision paths with allow/deny audit logs.
   - Evidence: `internal/service/backup_service.go:46`, `internal/service/backup_service.go:51`, `internal/service/ticket_service.go:126`, `internal/service/ticket_service.go:131`

6. **Registration close policy + admin override flow** — **Fixed**
   - Config-based close-hours policy is enforced; admin override endpoint is exposed and admin-gated.
   - Evidence: `internal/service/registration_service.go:45`, `internal/service/registration_service.go:51`, `internal/router/router.go:132`, `internal/handler/api/registration_handler.go:142`

7. **Role literal mismatch (`admin` vs `administrator`)** — **Fixed**
   - Service authorization checks use role constants consistently.
   - Evidence: `internal/service/registration_service.go:360`, `internal/service/ticket_service.go:347`

8. **Churn KPI missing** — **Fixed**
   - `churn_rate` is added and computed in KPI response.
   - Evidence: `internal/service/dashboard_service.go:33`, `internal/service/dashboard_service.go:73`, `internal/service/dashboard_service.go:95`

9. **Logging safety/structure consistency** — **Fixed (targeted paths)**
   - Recovery/scheduler/audit failure paths emit structured logs with safer production behavior.
   - Evidence: `internal/middleware/recovery.go:30`, `internal/middleware/recovery.go:40`, `internal/scheduler/scheduler.go:81`, `internal/service/audit_service.go:25`

## Overall Conclusion

- **All 9 previously tracked issues are fixed by static evidence.**
- **Manual verification still required** for runtime confirmation of behavior under real execution conditions.
