# CampusRec Seat & Commerce Operations Platform — Design Document

## 1. Overview

CampusRec is a unified on-site operations platform for recreation centers, clinic-adjacent wellness facilities, and training studios running on a private network. It combines program registration, seat utilization tracking, offline commerce, attendance management, moderation, and administrative dashboards into a single deployable service.

The system operates entirely on a local network with no external internet dependency beyond the internal WeChat Pay callback endpoint.

---

## 2. Architecture

### 2.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Browser (Templ SSR)                   │
│              English UI · Responsive Layout              │
└──────────────────────┬──────────────────────────────────┘
                       │ HTTP
┌──────────────────────▼──────────────────────────────────┐
│                   Gin HTTP Server                        │
│  ┌──────────┐ ┌──────────┐ ┌────────┐ ┌─────────────┐  │
│  │ RequestID│ │AuthSession│ │  RBAC  │ │  AuditLog   │  │
│  └────┬─────┘ └────┬─────┘ └───┬────┘ └──────┬──────┘  │
│       └─────────────┴───────────┴─────────────┘         │
│  ┌────────────────────────────────────────────────────┐  │
│  │              Handler Layer (API + Web)              │  │
│  ├────────────────────────────────────────────────────┤  │
│  │              Service Layer (Business Logic)         │  │
│  ├────────────────────────────────────────────────────┤  │
│  │              Repository Layer (pgx)                 │  │
│  └────────────────────────┬───────────────────────────┘  │
└───────────────────────────┼──────────────────────────────┘
                            │ pgx/pgxpool
┌───────────────────────────▼──────────────────────────────┐
│                   PostgreSQL 16                           │
│  users · sessions · orders · payments · tickets · etc.   │
│  tsvector + GIN indexes · BIGINT monetary · soft delete  │
└──────────────────────────────────────────────────────────┘
```

### 2.2 Tech Stack

| Layer | Technology | Rationale |
|-------|-----------|-----------|
| HTTP framework | Gin | High-performance Go router with middleware ecosystem |
| Template engine | Templ (a-h/templ) | Type-safe, compile-time checked Go templates as specified in PRD |
| Database | PostgreSQL 16 | Native UUID, INET, JSONB, TIMESTAMPTZ types; tsvector full-text search |
| DB driver | pgx v5 + pgxpool | Native PostgreSQL wire protocol, connection pooling, advanced type mapping |
| Migrations | pressly/goose v3 | Embeddable SQL-native migrations, no runtime internet dependency |
| Password hashing | bcrypt (cost 12) | Adaptive hash; cost 12 balances security and performance on target hardware |
| UUID | google/uuid v4 | Standard UUID generation |

### 2.3 Layered Architecture

The codebase follows a strict three-layer separation:

- **Handler** (`internal/handler/api/`, `internal/handler/web/`): HTTP request parsing, input validation, response formatting. No business logic.
- **Service** (`internal/service/`): Business rules, state transitions, cross-entity coordination. No SQL or HTTP concerns.
- **Repository** (`internal/repo/`): Data access via pgx. No business rules.

Dependency injection is manual — `Services` struct aggregates all service instances, each receiving its required repository dependencies at initialization.

---

## 3. Data Model

### 3.1 Core Domain Entities

#### User Management
- **users**: Core identity with username, email, phone, password_hash, lockout tracking (failed_attempts, locked_until). Soft-deletable.
- **auth_sessions**: Server-side session tokens (SHA-256 hashed), with IP/user-agent tracking and revocation support.
- **roles / user_role_assignments**: Four predefined roles (member, staff, moderator, administrator) with time-scoped assignments (effective_from/effective_until).

#### Catalog
- **program_sessions (via sessions table)**: Scheduled classes/activities with title, description, category, instructor, pricing (BIGINT minor units), capacity, registration windows, approval flags, waitlist toggle.
- **products**: Physical merchandise with pricing, stock tracking, is_shippable flag.
- **seat_inventory / product_inventory**: Versioned inventory with optimistic locking for concurrent access.

#### Registration & Attendance
- **session_registrations**: Full lifecycle: pending_approval → registered → checked_in → temporarily_away → completed (plus cancel/reject/no-show/released/expired paths).
- **waitlist_entries**: Position-tracked queue with automatic promotion on seat release.
- **session_policies**: Per-session configurable rules (check-in lead time, no-show threshold, max leave duration, unverified occupancy threshold).
- **check_in_events**: Check-in records with method and confirmation tracking.
- **occupancy_sessions**: Active presence tracking with start/end times.
- **temporary_leave_events**: Leave tracking with duration enforcement.
- **occupancy_exceptions**: Anomaly detection records with resolution workflow.

#### Commerce
- **carts / cart_items**: Shopping cart with price snapshots at time of addition.
- **orders / order_items**: Multi-state order lifecycle (draft → awaiting_payment → paid → fulfillment_pending → shipped → delivered, with auto-close, cancellation, and refund paths).
- **payment_requests**: Time-limited QR payment requests with 15-minute countdown and expiry tracking.
- **payments**: Confirmed payment records with gateway transaction ID and signature verification.
- **refunds**: Refund tracking with amount validation against original payment.

#### Shipping
- **shipments**: Fulfillment lifecycle (pending → packed → shipped → delivered) with exception handling.
- **delivery_proofs**: Proof-of-delivery with signature image path or typed acknowledgment.
- **delivery_exceptions**: Exception reporting and resolution.

#### Moderation
- **posts**: User-generated content with status tracking and rate limiting (5/hour).
- **post_reports**: Content reports with per-user deduplication.
- **moderation_cases / moderation_actions**: Investigation cases with action history.
- **account_bans**: Temporary or permanent bans with revocation support.

#### Support
- **tickets**: Support tickets with SLA tracking (4-hour response, 3-day resolution), status machine, assignment, and escalation.
- **ticket_comments**: Comments with internal/external visibility flag.

#### System
- **system_config**: Key-value configuration with optimistic locking via version column.
- **feature_flags**: Feature toggles with cohort percentage rollout, role targeting, and deterministic user assignment via SHA-256 hash.
- **audit_logs**: Immutable append-only log with old_state/new_state JSON capture and metadata.
- **jobs / scheduled_jobs**: Async job queue with priority, retry logic, lease tokens, and cron scheduling.
- **backup_runs / restore_runs / archive_runs**: Backup/restore/archival tracking with encryption and retention policies.
- **file_artifacts / import_jobs / import_rows / export_jobs**: Offline data import/export with validation, deduplication, and checksum fingerprinting.

### 3.2 Key Data Conventions

| Convention | Implementation |
|-----------|---------------|
| Monetary values | BIGINT in minor units (cents/fen). Default currency: CNY |
| Soft delete | `deleted_at TIMESTAMPTZ` — NULL means active |
| Timestamps | UTC, TIMESTAMPTZ columns, RFC3339 in API responses |
| Primary keys | UUID v4 |
| Case-insensitive uniqueness | `lower(username)` partial index on non-deleted rows |
| Full-text search | PostgreSQL `tsvector` with GIN index over title, description, category, instructor |

---

## 4. Authentication & Authorization

### 4.1 Authentication Flow

1. User submits username + password via `POST /api/v1/auth/login`
2. System validates credentials against bcrypt hash (cost 12)
3. Account lockout check: 5 failed attempts within a rolling window triggers 15-minute lockout
4. On success: 32-byte crypto/rand token generated, SHA-256 hash stored in `auth_sessions` table
5. Session cookie set: HttpOnly + SameSite=Strict (Secure flag configurable for private HTTP networks)
6. Session idle timeout: Members 8 hours, Staff/Admin 30 minutes (configurable via system_config)

### 4.2 RBAC Model

Four roles enforced at the middleware layer via `RequireRole()`:

| Role | Access Scope |
|------|-------------|
| Member | Catalog browsing, registration, cart/checkout, own orders, posts, tickets |
| Staff | Member access + check-in, shipment management, registration approval/rejection, exception viewing |
| Moderator | Member access + post report review, moderation cases, ban management |
| Administrator | Full access including config, feature flags, audit logs, KPIs, backup/restore, import/export, archival |

Role assignments are time-scoped with `effective_from` and `effective_until` columns.

---

## 5. Key Workflows

### 5.1 Session Registration & Seat Control

```
Member registers → Check approval required?
  ├─ No approval + seats available → Status: Registered (atomic seat deduction)
  ├─ Approval required → Status: Pending Approval
  │     ├─ Staff approves → Registered
  │     └─ Staff rejects → Rejected (promote waitlist)
  └─ No seats + waitlist enabled → Status: Waitlisted
        └─ Seat released → Auto-promote next waitlisted user
```

- Seat deduction uses database transactions to prevent overselling.
- Waitlist promotion is automatic and recursive (skips inactive users).
- Registration closes 2 hours before session start (admin-overridable via policy).

### 5.2 Attendance & Occupancy

```
Staff check-in (within 30min window) → Status: Checked In → Occupancy session created
  ├─ Temporary leave → Status: Temporarily Away (max 15min enforced)
  │     └─ Return → Status: Checked In
  ├─ Session ends → Status: Completed
  └─ Unverified occupancy > threshold → Exception ticket created
  
No check-in within 10min of start → Auto no-show cancel
```

### 5.3 Order & Payment Lifecycle

```
Add to Cart → Checkout (or Buy Now) → Order: Awaiting Payment
  → Create Payment Request (QR code, 15-min countdown)
    ├─ WeChat Pay callback received → Verify HMAC-SHA256 signature → Order: Paid
    │     ├─ Shippable items → Create Shipment → Fulfillment Pending → Shipped → Delivered
    │     │     └─ Exception → Delivery Exception (with POD capture)
    │     └─ Refund initiated → Refund Pending → Refunded
    └─ 15 minutes elapsed → Order: Auto-Closed (UI explains next steps)
```

- Payment signature: HMAC-SHA256 of `{gatewayTxID}|{merchantRef}|{amount}` using locally stored merchant key.
- Idempotency enforced via `gateway_tx_id` uniqueness on payments and `idempotency_key` on orders.
- Immutable audit logs record all status transitions.

### 5.4 Moderation

```
Member creates post (rate limit: 5/hour, ban check)
  → Other members report post (deduplicated per reporter)
    → Moderator reviews report → Creates case
      → Action taken (warning / content removal / ban)
        → Ban: temporary (N days) or permanent, revocable
```

### 5.5 Ticket / Exception Workflow

```
Ticket created → SLA clock starts (4hr response, 3-day resolution)
  → Acknowledged → In Progress → Resolved → Closed
  ├─ SLA breach detected by scheduled job → Escalated
  └─ Waiting on member/staff → SLA paused
```

---

## 6. Offline-First Design

The platform is designed for private network operation:

- **No external network calls** required for core operation
- **Server-side sessions** (not JWT) — enables immediate revocation without token blacklists
- **WeChat Pay**: Callback signature verification uses locally stored merchant keys; QR payload format is `campusrec://pay?ref={merchantRef}&amount={amount}&currency={currency}`
- **Import/Export**: Excel/CSV parsing with strict validation, checksum-based duplicate detection, fully offline
- **Backups**: Nightly encrypted backups to local storage with 30-day retention, point-in-time restore with dry-run support
- **Archival**: Records older than 24 months moved to archive schema with masked lookup fields for reporting, processed in 1000-row chunks

---

## 7. Configuration & Feature Flags

### 7.1 System Configuration

Key-value store with optimistic locking (version column). Changes require the client to send the expected version — update fails on mismatch.

Key configurations: facility timezone, session timeouts, SLA durations, backup retention.

### 7.2 Canary Release

Feature flags support percentage-based rollout:

1. Flag defines `cohort_percent` (e.g., 10%) and optional `target_roles`
2. User assignment is **deterministic**: SHA-256 hash of `{flagKey}:{userID}:{flagVersion}` mod 100
3. If hash result < cohort_percent, user is in the cohort
4. Role targeting narrows eligibility further
5. Version bump resets cohort assignment (allows re-randomization)

---

## 8. Scheduled Jobs

Background jobs handle time-sensitive operations:

| Job | Function |
|-----|----------|
| Payment expiry | Finds expired payment requests, auto-closes associated orders |
| No-show detection | Cancels registrations not checked in within 10 minutes of session start |
| Stale occupancy detection | Flags unverified occupancy exceeding threshold, creates exception tickets |
| SLA breach check | Escalates tickets past response/resolution deadlines |
| Attendance completion | Transitions checked-in registrations to completed when session ends |

Jobs use a database-backed queue with priority, retry logic, and lease tokens for distributed safety.

---

## 9. API Design Conventions

| Aspect | Convention |
|--------|-----------|
| Envelope | `{"success": bool, "data": ..., "error": {"code": "...", "message": "..."}, "meta": {"request_id": "...", "timestamp": "..."}}` |
| Pagination | Offset-based: `page` (default 1), `per_page` (default 20, max 100) |
| Error codes | Machine-readable uppercase: `REGISTRATION_CLOSED`, `AUTH_LOCKED`, `SEAT_UNAVAILABLE` |
| Versioning | `/api/v1/` prefix |
| Content type | JSON request/response with snake_case field names |
| Idempotency | Supported on checkout and payment callbacks |

---

## 10. Deployment

### Single-Command Deploy

```bash
docker compose up --build
```

This starts PostgreSQL, runs goose migrations, seeds sample data (5 users across all roles), and starts the Gin server on port 8080.

### Environment Configuration

All configuration via environment variables with sensible defaults for private network operation. Required variables: `DATABASE_URL`, `SESSION_SECRET`. Optional: facility timezone, backup encryption key, payment merchant key, upload limits.

---

## 11. Security Considerations

- **Password policy**: 12-character minimum, bcrypt cost 12
- **Account lockout**: 5 failed attempts → 15-minute lockout
- **Session security**: HttpOnly + SameSite=Strict cookies; Secure flag configurable
- **RBAC enforcement**: Middleware-level role checks on every protected endpoint
- **Audit trail**: All mutating HTTP requests logged; domain events logged in service layer
- **Payment integrity**: HMAC-SHA256 signature verification on callbacks
- **Input validation**: Centralized validator with struct tag-based rules
- **Soft delete**: Data preserved for audit; hard delete only via archival process
- **Encrypted backups**: Backup files encrypted with configurable key
