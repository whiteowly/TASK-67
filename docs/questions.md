# CampusRec Seat & Commerce Operations Platform — Clarification Questions



## 1. Session Token Storage: Server-Side vs. JWT on a Private Network

**Question:** The prompt specifies "local username/password authentication" but does not prescribe a session management strategy. On a private network without horizontal scaling concerns, should sessions be managed server-side in the database, or should stateless JWTs be issued? JWTs would eliminate session lookup overhead but make immediate revocation (e.g., mid-shift staff removal) impossible without a blacklist.

**My Understanding:** Since the platform operates on a private network where horizontal scaling is not a concern, server-side sessions stored in PostgreSQL provide the strongest security guarantees. Immediate revocation is critical for a facility where staff roles may need to be suspended mid-shift, and the session lookup overhead is negligible on a local network.

**Solution:** The implementation uses server-side sessions stored in the `auth_sessions` table. A 32-byte cryptographically random token is generated via `crypto/rand`, and only its SHA-256 hash is persisted in the database. The raw token is sent to the client as an HttpOnly + SameSite=Strict cookie. Session validation in `middleware/auth.go` performs a database lookup on each request, enabling immediate revocation via `POST /api/v1/auth/logout` which sets `revoked_at` on the session record. Session idle timeouts are configurable per role via the `system_config` table (Members: 8 hours, Staff/Admin: 30 minutes by default).

---

## 2. Monetary Precision: Floating-Point vs. Integer Storage for Prices

**Question:** The prompt references pricing for sessions and products and WeChat Pay amounts, but does not specify how monetary values should be stored. Floating-point arithmetic is notoriously lossy for currency calculations (e.g., 0.1 + 0.2 ≠ 0.3), but using integer minor units requires all display layers to handle conversion.

**My Understanding:** Financial precision is non-negotiable for a commerce platform. All monetary values should be stored as integers in minor currency units (fen for CNY) to eliminate rounding errors entirely. The display layer handles conversion to major units for rendering.

**Solution:** All monetary columns (`unit_price_minor`, `total_minor`, `amount_minor`, etc.) across `orders`, `order_items`, `payment_requests`, `payments`, and `refunds` are defined as `BIGINT` storing values in minor currency units. The default currency is CNY, so 1 yuan = 100 fen. Cart items snapshot the price at the time of addition to prevent price-change race conditions during checkout. The `payment_service.go` callback handler validates that `req.AmountMinor == payment.AmountMinor` as an exact integer comparison before confirming payment.

---

## 3. Account Lockout: Rolling Window vs. Cumulative Counter

**Question:** The prompt says "lockout after 5 failed attempts for 15 minutes" but does not specify whether the 5-attempt threshold is a cumulative lifetime counter, a rolling window (e.g., 5 attempts within any 15-minute span), or a counter that resets on successful login. The choice significantly affects both security posture and user experience.

**My Understanding:** A cumulative lifetime counter would permanently lock accounts after enough typos, which is unacceptable. A rolling window is the most secure interpretation but adds query complexity. A counter that resets on successful login is the simplest approach that still meets the spirit of the requirement — it prevents brute-force attacks while not punishing users for historical mistakes.

**Solution:** The implementation in `auth_service.go` uses a counter on the `users` table (`failed_attempts` column) that increments on each failed login and resets to zero on successful login. When `failed_attempts` reaches 5, the `locked_until` timestamp is set to 15 minutes from the last failed attempt. The login handler checks `locked_until` before attempting credential validation — if the current time is before `locked_until`, the request is rejected with an `AUTH_LOCKED` error code without even verifying the password, preventing timing-based enumeration during lockout.

---

## 4. Seat Deduction Atomicity: How to Prevent Overselling Under Concurrent Registration

**Question:** The prompt requires "atomic seat deduction with database transactions to prevent overselling," but does not specify the concurrency control mechanism. Simple read-then-write patterns are vulnerable to time-of-check-to-time-of-use (TOCTOU) races when multiple members register for the same session simultaneously.

**My Understanding:** Database-level enforcement is the only reliable approach. Application-level locks (mutexes) would not work across multiple server instances, and even on a single instance they add fragile coordination. The seat count should be enforced with a CHECK constraint at the database level as the final safety net, with the application using transactional updates.

**Solution:** The `seat_inventory` table uses a `CHECK (reserved_seats <= total_seats)` constraint that makes it impossible for the database to store an oversold state. The `registration_service.go` `Register()` method operates within a database transaction: it reads the current inventory, increments `reserved_seats`, and the database rejects the commit if the constraint is violated. Additionally, the `available_seats` column is a generated column (`total_seats - reserved_seats`) providing a consistent view. The `seat_inventory` table also includes a `version` column for optimistic locking, preventing lost-update anomalies when concurrent transactions modify the same row.

---

## 5. Waitlist Promotion Timing and Inactive User Handling

**Question:** The prompt specifies "automatically promote the next user from the waitlist within 30 seconds when a seat is released" but does not clarify what happens if the next waitlisted user's account has been deactivated or banned since they joined the waitlist. Should the system skip inactive users automatically, or should it notify staff and wait?

**My Understanding:** Automatic skipping of inactive users is the only practical approach — waiting for staff intervention would defeat the purpose of automatic promotion and would likely exceed the 30-second window. The system should recursively check the next eligible user until it finds an active one or exhausts the waitlist.

**Solution:** The `registration_service.go` `PromoteNextWaitlist()` method queries the next waitlist entry by position order, then checks whether the user is still active (`is_active = true` on the user record). If the user is inactive, the method records the skip reason on the `waitlist_entries` record (`last_attempt_reason`) and recursively calls itself to try the next entry. This continues until either a valid user is promoted or the waitlist is exhausted. The promotion transitions the registration from `waitlisted` to `registered` and logs the state change in `registration_status_history`.

---

## 6. WeChat Pay Callback Signature Verification Without External Network

**Question:** The prompt requires "WeChat Pay callback signature verification using locally stored merchant keys" but does not describe the signature format or verification algorithm. Standard WeChat Pay uses XML-based callbacks with HMAC-MD5 or HMAC-SHA256, but on a private network the payment terminal may use a custom protocol.

**My Understanding:** Since the system operates on a private network, the payment flow uses a locally agreed-upon signature scheme rather than the standard WeChat public API. The verification must be deterministic, use a strong hash algorithm, and validate the exact payment amount to prevent tampering.

**Solution:** The `payment_service.go` `ProcessCallback()` method implements HMAC-SHA256 signature verification. The signature payload is constructed as `{gatewayTxID}|{merchantRef}|{amount}` — a pipe-delimited concatenation of the transaction ID, merchant reference, and amount. The merchant key is loaded from the `PAYMENT_MERCHANT_KEY` environment variable and stored in the application config. The handler in `payment_handler.go` calls `hmac.Equal()` for constant-time comparison to prevent timing attacks. Payment callbacks are idempotent — duplicate `gateway_tx_id` values are detected and the existing payment is returned without modification.

---

## 7. Payment Expiry and Order Auto-Close: Background Job vs. Request-Time Check

**Question:** The prompt specifies a "15-minute countdown" for payment and that "the order is auto-closed" if not confirmed, but does not prescribe whether this should be enforced by a background job polling for expired requests or lazily evaluated when the order is next accessed. Background jobs add operational complexity; lazy evaluation risks stale UI state.

**My Understanding:** A background job is the correct approach because the order auto-close must trigger regardless of whether the member returns to the UI. Lazy evaluation would leave orders in an inconsistent state if the member never checks back, potentially holding seat reservations or inventory indefinitely.

**Solution:** The `payment_service.go` `ExpirePayments()` method is designed to run as a scheduled job. It queries for all `payment_requests` where `status = 'pending'` and `expires_at < NOW()`, then transitions each associated order to `AutoClosed` status. The `payment_requests` table stores `expires_at` calculated as `created_at + 15 minutes`. The order status transition to `auto_closed` is recorded in the audit log. The QR payload generated by `CreatePaymentRequest()` includes the merchant reference and amount in the format `campusrec://pay?ref={merchantRef}&amount={amount}&currency={currency}`, and the frontend renders a countdown timer from the `expires_at` value.

---

## 8. No-Show Detection Window: Absolute Time vs. Configurable Per-Session Policy

**Question:** The prompt states "no-show is auto-canceled after 10 minutes past start time," but does not specify whether this is a global constant or configurable per session. Some sessions (e.g., a 3-hour workshop) may tolerate late arrivals, while a 30-minute express class cannot.

**My Understanding:** The 10-minute threshold should be the system default but overridable per session. Different session types have fundamentally different tolerance for late arrivals, and a one-size-fits-all policy would either be too strict for long sessions or too lenient for short ones.

**Solution:** The `session_policies` table stores per-session configurable thresholds including `noshow_cancel_minutes` (default: 10), `checkin_lead_minutes` (default: 30), `leave_max_minutes` (default: 15), and `unverified_threshold_minutes`. Each policy row is linked to a specific session and includes a `version` column for historical tracking. The `attendance_service.go` `DetectNoShows()` method queries for registrations in `registered` status where the session start time plus the policy's `noshow_cancel_minutes` has elapsed, then transitions them to `no_show_canceled`. This allows an administrator to set a 20-minute grace period for a workshop while keeping the 10-minute default for express classes.

---

## 9. Temporary Leave Enforcement: What Happens When the Timer Expires

**Question:** The prompt specifies "one 10-minute break per hour, after which the seat is released," but does not define what "released" means operationally. Should the registration be canceled outright? Should it transition to a special state? Should the member be notified? The prompt also does not specify whether leave duration is checked in real-time or by a background job.

**My Understanding:** Exceeding the leave duration should not immediately cancel the registration — that would be too punitive for a minor overstay. Instead, it should trigger an exception that staff can review, while the registration transitions to a state that allows the member to return but flags the overstay.

**Solution:** The `attendance_service.go` `StartLeave()` method creates a `temporary_leave_events` record with the current timestamp and transitions the registration to `temporarily_away`. The `leave_max_minutes` from `session_policies` (default: 15 minutes in the implementation) defines the allowed duration. When `EndLeave()` is called, it calculates whether the leave duration exceeded the maximum and sets `exceeded = true` on the leave event if so. Separately, the `DetectStaleOccupancy()` background job identifies occupancy sessions where the member has been away beyond the unverified threshold and creates `occupancy_exceptions` records with type and description, which appear in the staff-facing exception list at `GET /api/v1/attendance/exceptions`.

---

## 10. Ticket SLA Calculation: Business Hours vs. Calendar Hours

**Question:** The prompt specifies "initial response within 4 business hours, resolution within 3 calendar days" — mixing business-hour and calendar-day calculations. It does not define what constitutes "business hours" for a recreation facility (which may operate on weekends), nor does it specify how timezone affects SLA computation.

**My Understanding:** Since recreation facilities often operate 7 days a week including weekends, "business hours" likely maps to the facility's operating hours rather than a traditional 9-to-5 Monday-Friday schedule. However, implementing full business-hour calculation with dynamic operating schedules adds significant complexity. A practical first approach is to use calendar hours for both SLAs and allow configuration adjustment.

**Solution:** The `ticket_service.go` `Create()` method calculates SLA deadlines at ticket creation time: `response_due_at` is set to `now + 4 hours` and `resolution_due_at` is set to `now + 72 hours` (3 calendar days). These are stored as absolute timestamps on the `tickets` table. The `CheckSLABreaches()` scheduled job queries for tickets where the current time exceeds either SLA deadline and the ticket has not yet reached the corresponding milestone (acknowledged for response, resolved for resolution), then escalates by updating the ticket status. The facility timezone is configured via the `FACILITY_TIMEZONE` environment variable (default: `Asia/Shanghai`) and loaded through the `util/timezone.go` helper to ensure consistent timestamp interpretation.

---

## 11. Moderation Rate Limiting: Per-User Enforcement Across Sessions

**Question:** The prompt specifies "posting frequency limits (e.g., 5 posts per hour)" but does not specify whether this is enforced per-session, per-user globally, or per-IP. A per-session check would allow a banned user to re-login and resume posting; per-IP would break in NAT environments common on private networks.

**My Understanding:** Rate limiting must be per-user globally regardless of session, since the platform uses authenticated posting. IP-based limiting is unreliable on a private network where devices may share NAT addresses. The check should look at actual post creation timestamps over a sliding window.

**Solution:** The `moderation_service.go` `CreatePost()` method enforces two checks before allowing a post: first, it queries `account_bans` for any active ban on the user (where `revoked_at IS NULL` and either `permanent = true` or `expires_at > NOW()`); second, it counts posts created by the user in the last hour from the `posts` table. If the count equals or exceeds 5, the request is rejected. This is a per-user-ID check independent of session, so logging out and back in does not reset the counter. The rate limit window and threshold are not yet externalized to `system_config` but could be made configurable without structural changes.

---

## 12. Canary Release: Deterministic vs. Random Cohort Assignment

**Question:** The prompt specifies "canary release by user cohort percentage (e.g., 10% of staff accounts)" but does not specify whether a user's cohort assignment should be stable across requests or randomly re-evaluated. Random evaluation would cause inconsistent behavior for the same user within a single session; deterministic assignment requires a consistent hashing scheme.

**My Understanding:** Cohort assignment must be deterministic — a user who sees a canary feature on one page load must continue to see it on subsequent loads. Random assignment would cause confusing UX where features appear and disappear unpredictably. The assignment should also be re-randomizable when rolling out to a wider cohort.

**Solution:** The `feature_flag_service.go` `IsEnabledForUser()` method computes a deterministic assignment using SHA-256: the input is `{flagKey}:{userID}:{flagVersion}`, and the resulting hash is taken modulo 100 to produce a value between 0-99. If this value is less than the flag's `cohort_percent`, the user is included. The `flag_version` component allows administrators to re-randomize assignments by incrementing the version when expanding a rollout (e.g., from 10% to 50%), preventing the same users from always being in the canary group. Role targeting is applied as an additional filter — the `target_roles` JSON array on the flag specifies which roles are eligible.

---

## 13. Data Archival: Retaining Referential Integrity After Moving Records

**Question:** The prompt specifies "moving closed orders and tickets older than 24 months into an archive schema while keeping masked lookup fields for reporting." Moving records to a separate schema risks breaking foreign key references from non-archived tables (e.g., audit logs referencing an archived order). How should referential integrity be maintained?

**My Understanding:** The archive process should create masked projection records in the main schema that satisfy foreign key constraints while the full records live in the archive schema. These projections contain only the fields needed for reporting (IDs, dates, anonymized amounts) with sensitive data masked.

**Solution:** The `backup_service.go` `RunArchive()` method processes records older than 24 months in chunks of 1000 rows. It creates an `archive_runs` record tracking the threshold date, rows moved, and processing status. The method is designed to create masked lookup projections in the main schema — records with identifiers and aggregated fields preserved but personal details redacted — while the full records are moved to the archive schema. The `archive_runs` table tracks `rows_moved`, `threshold_date`, and `status` (pending → running → completed/failed) for operational visibility. Administrators trigger archival via `POST /api/v1/admin/archives` and monitor progress via `GET /api/v1/admin/archives`.

---

## 14. Import Duplicate Detection: Checksum-Based vs. Content-Based Deduplication

**Question:** The prompt requires "duplicate detection and file fingerprinting" for imports but does not specify whether duplicates should be detected at the file level (same file uploaded twice) or at the row level (same record appearing in different files). File-level detection prevents re-processing the same upload; row-level detection prevents data conflicts.

**My Understanding:** Both levels are needed but serve different purposes. File-level deduplication is a quick guard against accidental re-uploads (operator clicks "upload" twice). Row-level deduplication requires template-specific logic and should be handled during the validation phase.

**Solution:** The `import_service.go` `UploadImport()` method implements file-level deduplication by computing a checksum of the uploaded file and querying `file_artifacts` for an existing record with the same hash. If found, the upload is rejected with a duplicate error. The `import_jobs` table tracks the job lifecycle through statuses: `uploaded` → `validated` → `applied` (or `failed`). Row-level validation is handled in the `ValidateImport()` method, which is designed to apply template-specific rules and record per-row errors in the `import_rows` table with `raw_data` (original content) and `errors` (validation failures). The `ApplyImport()` method only proceeds if the job is in `validated` status, preventing application of unvalidated data.

---

## 15. Proof-of-Delivery: Signature Image Storage on a Private Network

**Question:** The prompt requires "proof-of-delivery (signature image or typed acknowledgment)" but does not specify how signature images should be stored. On a private network without cloud object storage, the platform needs a local file storage strategy with appropriate retention and access controls.

**My Understanding:** Signature images should be stored as files on the local filesystem with a reference path in the database. The storage directory should be configurable, and files should be organized to prevent directory bloat. Access should be restricted to staff and administrators.

**Solution:** The `shipment_service.go` `RecordPOD()` method creates a `delivery_proofs` record with fields for `signature_path` (filesystem path to the signature image), `acknowledgment_text` (typed alternative), and `receiver_name`. The storage directory is configured via the `UPLOAD_DIR` environment variable (default: `./uploads`), managed through `config/config.go`. The shipment handler at `POST /api/v1/shipments/:id/pod` accepts the proof data and is restricted to staff+ roles via the RBAC middleware. The `delivery_proofs` table also records `recorded_by` (the staff user ID) and `recorded_at` for audit purposes.
