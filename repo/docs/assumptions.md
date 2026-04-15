# CampusRec — Implementation Assumptions

This document captures implementation choices that were made to deliver
the system but were **not** prescribed by the original product prompt.
For each ambiguity in `docs/questions.md` we record here the concrete
mechanism that was actually built. These are the design additions that
were intentionally moved out of `docs/questions.md` (per the merge-gate
requirement that questions.md contain only real prompt ambiguities).

## Assumptions (Not Explicit Prompt Requirements)

### 1. Server-side sessions in Postgres (resolves Q1)

- Sessions live in the `auth_sessions` table.
- 32-byte token via `crypto/rand`; only its SHA-256 is persisted.
- Raw token issued as HttpOnly + SameSite=Strict cookie.
- `middleware/auth.go` does a DB lookup per request; revocation is
  immediate via `revoked_at`.
- Idle timeouts are role-configurable in `system_config` (Member
  default 8 h, Staff/Admin default 30 min).

### 2. `BIGINT` minor units everywhere (resolves Q2)

- Columns: `unit_price_minor`, `total_minor`, `amount_minor`, etc.
- Default currency CNY, so 1 yuan = 100 fen.
- Cart items snapshot price at add-time to avoid race-during-checkout
  drift.
- `payment_service.go` uses exact integer comparison
  (`req.AmountMinor == payment.AmountMinor`) on callback.

### 3. Reset-on-success lockout counter (resolves Q3)

- `users.failed_attempts` increments on each failed login; reset on
  success.
- At 5 failures, `users.locked_until = now + 15min`.
- Login handler short-circuits with `AUTH_LOCKED` before bcrypt to
  avoid timing-side enumeration during the lockout window.

### 4. CHECK constraint + version on `seat_inventory` (resolves Q4)

- DB enforces `CHECK (reserved_seats <= total_seats)` — final safety
  net.
- `available_seats` is a generated column.
- `seat_inventory.version` enables optimistic locking from
  `registration_service.go::Register`.

### 5. Recursive skip of inactive waitlist users (resolves Q5)

- `registration_service.go::PromoteNextWaitlist` skips users where
  `is_active = false` and records the reason on the waitlist row.
- Promotion transitions `waitlisted → registered` and writes
  `registration_status_history`.

### 6. HMAC-SHA256 over pipe-joined fields (resolves Q6)

- Signature payload: `{gatewayTxID}|{merchantRef}|{amountMinor}`.
- Key from `PAYMENT_MERCHANT_KEY` env var; loaded into
  `config.PaymentConfig`.
- Constant-time compare via `hmac.Equal`.
- Idempotency: duplicate `gateway_tx_id` returns the existing payment
  unchanged.

### 7. Scheduled job for payment expiry (resolves Q7)

- `payment_service.go::ExpirePayments` queries `payment_requests`
  where `status='pending' AND expires_at < NOW()` and transitions the
  associated order to `auto_closed`.
- `expires_at = created_at + 15min`.
- QR payload format:
  `campusrec://pay?ref={merchantRef}&amount={amount}&currency={currency}`.

### 8. Per-session `noshow_cancel_minutes` policy (resolves Q8)

- `session_policies` table: `noshow_cancel_minutes` (default 10),
  `checkin_lead_minutes` (30), `leave_max_minutes` (15),
  `unverified_threshold_minutes`.
- `attendance_service.go::DetectNoShows` uses the per-session policy.

### 9. Flag-and-keep on leave overstay (resolves Q9)

- `temporary_leave_events.exceeded` flag set on overstay; registration
  not canceled.
- `DetectStaleOccupancy` raises `occupancy_exceptions` for staff via
  `GET /api/v1/attendance/exceptions`.

### 10. Calendar-hour SLA in facility timezone (resolves Q10)

- `tickets.response_due_at = created_at + 4h`,
  `tickets.resolution_due_at = created_at + 72h`, both absolute
  timestamps.
- `CheckSLABreaches` background job escalates on breach.
- `FACILITY_TIMEZONE` env var (default `Asia/Shanghai`) feeds
  `util/timezone.go`.

### 11. Per-user-global posting rate limit (resolves Q11)

- `moderation_service.go::CreatePost` enforces:
  1. no active row in `account_bans` for the user,
  2. fewer than 5 `posts.created_at` rows in the last hour by the user.
- Window/threshold currently constants; could be moved to
  `system_config` without structural change.

### 12. Deterministic SHA-256 cohort hashing (resolves Q12)

- `feature_flag_service.go::IsEnabledForUser`:
  `SHA256({flagKey}:{userID}:{flagVersion}) mod 100 < cohort_percent`.
- Bumping `flag_version` re-randomizes when expanding rollouts.
- `target_roles` JSON array filters by role first.

### 13. Masked projection rows for archive referential integrity (resolves Q13)

- `archive_runs` row tracks `threshold_date`, `rows_moved`, `status`.
- `archive_lookup_projection` keeps masked references in the main
  schema so audit logs/foreign keys still resolve.
- Triggered via `POST /api/v1/admin/archives`; monitored via
  `GET /api/v1/admin/archives`.

### 14. File-level dedup at upload, row-level at validate (resolves Q14)

- File hash via SHA-256 stored in `file_artifacts`; duplicate uploads
  rejected at `UploadImport`.
- `import_jobs` lifecycle: `uploaded → validated → applied | failed`.
- Per-row failures land in `import_rows` (`raw_data`, `errors`).
- `ApplyImport` requires status `validated`.

### 15. Local filesystem PoD storage with RBAC (resolves Q15)

- `delivery_proofs.signature_path` holds the relative path; root is
  `UPLOAD_DIR` env var (default `./uploads`).
- `POST /api/v1/shipments/:id/pod` is gated by staff+ RBAC.
- `recorded_by` and `recorded_at` captured for audit.

---

## Other implementation choices (no corresponding ambiguity)

These are choices made for which the prompt was silent but where there
was no genuine ambiguity flagged in `docs/questions.md`:

- **Web framework:** Gin (Go).
- **HTML rendering:** Templ (`web/templates/`).
- **Database driver:** `pgx/v5` with `pgxpool.Pool`.
- **Migrations:** in `db/migrations/` applied at boot via
  `cmd/server -migrate up`.
- **Audit-log INET handling:** `host(ip_addr)` cast in
  `internal/repo/audit_repo.go::List` — pgx's default codec does not
  scan PostgreSQL `INET` directly into `*string`, so the SELECT casts
  to text.
- **Test layout:** three suites — `tests/external_api/` (CI primary,
  100 % external endpoint coverage), `tests/blackbox/` (deeper
  behavioral over real HTTP), `tests/integration/` (in-process,
  informational only). Documented in `docs/testing.md`.
- **External API runner:** `./run_external_api_tests.sh` orchestrates
  `docker-compose.external-api.yml` (db + app + tests) and runs
  `go test ./tests/external_api/...` against
  `EXTERNAL_API_BASE_URL=http://app:8080`.
