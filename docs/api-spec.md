# CampusRec Seat & Commerce Operations Platform — API Specification

## Base URL

```
http://<host>:8080/api/v1
```

## Response Envelope

All responses follow a standard envelope:

```json
{
  "success": true,
  "data": { ... },
  "error": null,
  "meta": {
    "request_id": "550e8400-e29b-41d4-a716-446655440000",
    "timestamp": "2026-04-14T10:30:00Z"
  }
}
```

Error responses:

```json
{
  "success": false,
  "data": null,
  "error": {
    "code": "AUTH_LOCKED",
    "message": "Account locked for 15 minutes due to repeated failed attempts"
  },
  "meta": { ... }
}
```

## Pagination

Paginated endpoints accept `page` (default: 1) and `per_page` (default: 20, max: 100) query parameters. Paginated responses include:

```json
{
  "success": true,
  "data": [ ... ],
  "meta": {
    "page": 1,
    "per_page": 20,
    "total": 142,
    "request_id": "...",
    "timestamp": "..."
  }
}
```

## Authentication

Session-based authentication via cookie. Include the `session` cookie on all authenticated requests.

---

## Endpoints

### Authentication

#### POST /auth/register

Create a new member account.

**Auth:** None

**Request Body:**
```json
{
  "username": "janedoe",
  "email": "jane@example.com",
  "password": "SecurePass1234",
  "display_name": "Jane Doe"
}
```

**Validations:**
- `username`: required, alphanumeric + underscores, 3-50 chars
- `email`: required, valid email format
- `password`: required, 12-character minimum
- `display_name`: optional

**Response:** `201 Created`
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "username": "janedoe",
    "display_name": "Jane Doe",
    "email": "jane@example.com",
    "roles": ["member"]
  }
}
```

**Error Codes:** `VALIDATION_ERROR`, `USERNAME_TAKEN`

---

#### POST /auth/login

Authenticate and create a session.

**Auth:** None

**Request Body:**
```json
{
  "username": "janedoe",
  "password": "SecurePass1234"
}
```

**Response:** `200 OK` — Sets `session` HttpOnly cookie
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "username": "janedoe",
    "display_name": "Jane Doe",
    "roles": ["member"]
  }
}
```

**Error Codes:** `AUTH_INVALID_CREDENTIALS`, `AUTH_LOCKED`, `AUTH_ACCOUNT_DISABLED`

---

#### POST /auth/logout

End the current session.

**Auth:** Required

**Response:** `200 OK` — Clears `session` cookie
```json
{
  "success": true,
  "data": null
}
```

---

### User Profile

#### GET /users/me

Get the authenticated user's profile.

**Auth:** Required

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "username": "janedoe",
    "display_name": "Jane Doe",
    "email": "jane@example.com",
    "phone": "+8613800138000",
    "roles": ["member"],
    "created_at": "2026-01-15T08:00:00Z"
  }
}
```

---

#### PATCH /users/me

Update profile fields.

**Auth:** Required

**Request Body:**
```json
{
  "display_name": "Jane D.",
  "email": "newemail@example.com",
  "phone": "+8613900139000"
}
```

**Response:** `200 OK` — Returns updated profile

---

### Catalog

#### GET /catalog/sessions

Browse program sessions.

**Auth:** None (public)

**Query Parameters:**
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `q` | string | — | Full-text search query |
| `status` | string | `published` | Filter by status (admin can use `draft`, `archived`) |
| `category` | string | — | Filter by category |
| `page` | int | 1 | Page number |
| `per_page` | int | 20 | Items per page (max 100) |

**Response:** `200 OK`
```json
{
  "success": true,
  "data": [
    {
      "id": "uuid",
      "title": "Morning Yoga",
      "short_description": "60-minute beginner-friendly yoga",
      "category": "wellness",
      "instructor_name": "Coach Li",
      "start_at": "2026-04-15T07:00:00Z",
      "end_at": "2026-04-15T08:00:00Z",
      "total_seats": 30,
      "available_seats": 12,
      "price_minor": 5000,
      "currency": "CNY",
      "registration_opens_at": "2026-04-01T00:00:00Z",
      "registration_closes_at": "2026-04-15T05:00:00Z",
      "requires_approval": false,
      "waitlist_enabled": true,
      "status": "published"
    }
  ],
  "meta": { "page": 1, "per_page": 20, "total": 45 }
}
```

---

#### GET /catalog/sessions/:id

Get session detail with availability.

**Auth:** None

**Response:** `200 OK` — Single session object with full description and availability counts

---

#### GET /catalog/products

Browse products.

**Auth:** None

**Query Parameters:** Same as sessions (`q`, `status`, `category`, `page`, `per_page`)

**Response:** `200 OK`
```json
{
  "success": true,
  "data": [
    {
      "id": "uuid",
      "name": "CampusRec Water Bottle",
      "short_description": "Insulated 500ml bottle",
      "category": "merchandise",
      "price_minor": 8900,
      "currency": "CNY",
      "is_shippable": true,
      "stock_quantity": 150,
      "status": "published"
    }
  ]
}
```

---

#### GET /catalog/products/:id

Get product detail with stock info.

**Auth:** None

**Response:** `200 OK` — Single product object

---

### Delivery Addresses

#### GET /addresses

List the authenticated user's delivery addresses.

**Auth:** Required

**Response:** `200 OK`
```json
{
  "success": true,
  "data": [
    {
      "id": "uuid",
      "label": "Office",
      "recipient_name": "Jane Doe",
      "phone": "+8613800138000",
      "province": "Beijing",
      "city": "Beijing",
      "district": "Haidian",
      "street_address": "123 University Rd",
      "postal_code": "100084",
      "is_default": true
    }
  ]
}
```

---

#### POST /addresses

Create a new delivery address.

**Auth:** Required

**Request Body:**
```json
{
  "label": "Office",
  "recipient_name": "Jane Doe",
  "phone": "+8613800138000",
  "province": "Beijing",
  "city": "Beijing",
  "district": "Haidian",
  "street_address": "123 University Rd",
  "postal_code": "100084",
  "is_default": true
}
```

**Response:** `201 Created`

**Notes:** Setting `is_default: true` clears the default flag on all other addresses for the user (enforced by partial unique index).

---

#### GET /addresses/:id

**Auth:** Required (ownership enforced)

#### PATCH /addresses/:id

**Auth:** Required (ownership enforced)

#### DELETE /addresses/:id

Soft-deletes the address.

**Auth:** Required (ownership enforced)

**Response:** `200 OK`

---

### Registration

#### POST /registrations

Register for a program session.

**Auth:** Required

**Request Body:**
```json
{
  "session_id": "uuid"
}
```

**Response:** `201 Created`
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "session_id": "uuid",
    "user_id": "uuid",
    "status": "registered",
    "registered_at": "2026-04-14T10:00:00Z"
  }
}
```

**Status Assignment Logic:**
- Seats available + no approval required → `registered`
- Seats available + approval required → `pending_approval`
- No seats + waitlist enabled → `waitlisted`
- No seats + waitlist disabled → error `SEAT_UNAVAILABLE`

**Error Codes:** `SESSION_NOT_FOUND`, `REGISTRATION_CLOSED`, `ALREADY_REGISTERED`, `SEAT_UNAVAILABLE`

---

#### GET /registrations

List the authenticated user's registrations.

**Auth:** Required

**Query Parameters:** `page`, `per_page`

**Response:** `200 OK` — Paginated list of registrations

---

#### GET /registrations/:id

**Auth:** Required

#### POST /registrations/:id/cancel

Cancel a registration. Only allowed from `pending_approval`, `registered`, or `waitlisted` states.

**Auth:** Required (owner)

**Response:** `200 OK`

**Side Effect:** If the canceled registration held a seat, the next waitlisted user is automatically promoted.

**Error Codes:** `REGISTRATION_NOT_CANCELABLE`

---

#### POST /registrations/:id/approve

Approve a pending registration.

**Auth:** Staff, Administrator

**Response:** `200 OK` — Registration transitions to `registered`

---

#### POST /registrations/:id/reject

Reject a pending registration.

**Auth:** Staff, Administrator

**Response:** `200 OK` — Registration transitions to `rejected`; waitlist promoted

---

### Attendance

#### POST /attendance/checkin

Record a member's check-in for a session.

**Auth:** Staff, Administrator

**Request Body:**
```json
{
  "registration_id": "uuid",
  "method": "qr_scan",
  "confirmed": true
}
```

**Validations:**
- Registration must be in `registered` status
- Current time must be within the check-in window (default: 30 minutes before session start to session start time)

**Response:** `201 Created`
```json
{
  "success": true,
  "data": {
    "check_in_event_id": "uuid",
    "occupancy_session_id": "uuid",
    "registration_status": "checked_in"
  }
}
```

**Error Codes:** `REGISTRATION_NOT_FOUND`, `INVALID_CHECKIN_STATE`, `CHECKIN_WINDOW_CLOSED`

---

#### POST /attendance/leave

Record a temporary leave.

**Auth:** Required (member)

**Request Body:**
```json
{
  "registration_id": "uuid"
}
```

**Response:** `201 Created`
```json
{
  "success": true,
  "data": {
    "leave_event_id": "uuid",
    "max_minutes": 15,
    "started_at": "2026-04-15T07:35:00Z"
  }
}
```

**Error Codes:** `NOT_CHECKED_IN`

---

#### POST /attendance/leave/:id/return

End a temporary leave.

**Auth:** Required

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "leave_event_id": "uuid",
    "duration_minutes": 8,
    "exceeded": false
  }
}
```

---

#### GET /attendance/exceptions

List occupancy exceptions.

**Auth:** Staff, Administrator

**Query Parameters:** `page`, `per_page`

**Response:** `200 OK` — Paginated list of occupancy exceptions with type, description, and resolution status

---

### Cart

#### GET /cart

Get the authenticated user's active cart.

**Auth:** Required

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "items": [
      {
        "id": "uuid",
        "item_type": "product",
        "item_id": "uuid",
        "item_name": "CampusRec Water Bottle",
        "quantity": 2,
        "unit_price_minor": 8900,
        "currency": "CNY"
      }
    ]
  }
}
```

---

#### POST /cart/items

Add an item to cart.

**Auth:** Required

**Request Body:**
```json
{
  "item_type": "product",
  "item_id": "uuid",
  "quantity": 1
}
```

`item_type` can be `session` or `product`. Price is snapshotted at the time of addition.

**Response:** `201 Created`

---

#### DELETE /cart/items/:id

Remove an item from the cart.

**Auth:** Required (ownership enforced)

**Response:** `200 OK`

---

### Orders

#### POST /checkout

Create an order from the current cart.

**Auth:** Required

**Request Body:**
```json
{
  "delivery_address_id": "uuid",
  "idempotency_key": "client-generated-uuid"
}
```

`delivery_address_id` is required if the cart contains shippable items.

**Response:** `201 Created`
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "status": "awaiting_payment",
    "total_minor": 17800,
    "currency": "CNY",
    "items": [ ... ],
    "created_at": "2026-04-14T10:30:00Z"
  }
}
```

**Error Codes:** `CART_EMPTY`, `ADDRESS_REQUIRED`, `IDEMPOTENCY_CONFLICT`

---

#### POST /buy-now

Single-item checkout bypassing the cart.

**Auth:** Required

**Request Body:**
```json
{
  "item_type": "product",
  "item_id": "uuid",
  "quantity": 1,
  "delivery_address_id": "uuid"
}
```

**Response:** `201 Created` — Same as checkout response

---

#### GET /orders

List the authenticated user's orders.

**Auth:** Required

**Query Parameters:** `page`, `per_page`

**Response:** `200 OK` — Paginated order list

---

#### GET /orders/:id

Get order detail.

**Auth:** Required (ownership enforced)

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "status": "awaiting_payment",
    "total_minor": 17800,
    "currency": "CNY",
    "items": [
      {
        "id": "uuid",
        "item_type": "product",
        "item_name": "CampusRec Water Bottle",
        "quantity": 2,
        "unit_price_minor": 8900,
        "total_minor": 17800,
        "is_shippable": true
      }
    ],
    "delivery_address_id": "uuid",
    "payment_request": {
      "id": "uuid",
      "qr_payload": "campusrec://pay?ref=MR-20260414-ABC123&amount=17800&currency=CNY",
      "expires_at": "2026-04-14T10:45:00Z",
      "status": "pending"
    },
    "created_at": "2026-04-14T10:30:00Z"
  }
}
```

---

#### POST /orders/:id/pay

Create a payment request for an order (generates QR code data).

**Auth:** Required (ownership enforced)

**Response:** `201 Created`
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "merchant_ref": "MR-20260414-ABC123",
    "qr_payload": "campusrec://pay?ref=MR-20260414-ABC123&amount=17800&currency=CNY",
    "amount_minor": 17800,
    "currency": "CNY",
    "expires_at": "2026-04-14T10:45:00Z",
    "status": "pending"
  }
}
```

**Error Codes:** `ORDER_NOT_PAYABLE`, `PAYMENT_REQUEST_EXISTS`

---

### Payments

#### POST /payments/callback

Receive payment confirmation from the payment terminal (WeChat Pay on the private network).

**Auth:** None (verified via HMAC-SHA256 signature)

**Request Body:**
```json
{
  "gateway_tx_id": "WX20260414103500001",
  "merchant_ref": "MR-20260414-ABC123",
  "amount_minor": 17800,
  "signature": "a1b2c3d4e5..."
}
```

**Signature Verification:**
- Payload: `{gateway_tx_id}|{merchant_ref}|{amount_minor}`
- Algorithm: HMAC-SHA256 with locally stored merchant key
- Comparison: Constant-time via `hmac.Equal()`

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "payment_id": "uuid",
    "order_id": "uuid",
    "status": "confirmed"
  }
}
```

**Idempotency:** Duplicate `gateway_tx_id` returns the existing payment without modification.

**Error Codes:** `SIGNATURE_INVALID`, `PAYMENT_REQUEST_NOT_FOUND`, `AMOUNT_MISMATCH`

---

### Shipments

#### POST /shipments

Create a shipment for a paid order.

**Auth:** Staff, Administrator

**Request Body:**
```json
{
  "order_id": "uuid"
}
```

**Validations:**
- Order must be in `paid` status
- Order must contain shippable items

**Response:** `201 Created`

**Side Effect:** Order transitions to `fulfillment_pending`

---

#### GET /shipments

List shipments.

**Auth:** Staff, Administrator

**Query Parameters:** `status`, `page`, `per_page`

**Response:** `200 OK` — Paginated shipment list

---

#### PATCH /shipments/:id/status

Update shipment status.

**Auth:** Staff, Administrator

**Request Body:**
```json
{
  "status": "shipped"
}
```

**Valid Transitions:**
- `pending_fulfillment` → `packed`
- `packed` → `shipped`
- `shipped` → `delivered`

**Side Effect:** Syncs order status (e.g., shipment `shipped` → order `shipped`)

---

#### POST /shipments/:id/pod

Record proof of delivery.

**Auth:** Staff, Administrator

**Request Body:**
```json
{
  "signature_path": "/uploads/pod/sig-20260414.png",
  "acknowledgment_text": "Received by front desk",
  "receiver_name": "Zhang Wei"
}
```

**Response:** `201 Created`

---

#### POST /shipments/:id/exception

Report a delivery exception.

**Auth:** Staff, Administrator

**Request Body:**
```json
{
  "type": "damaged_in_transit",
  "description": "Package arrived with visible water damage"
}
```

**Response:** `201 Created`

---

### Posts & Moderation

#### GET /posts

List community posts.

**Auth:** None (public)

**Query Parameters:** `page`, `per_page`

**Response:** `200 OK` — Paginated post list

---

#### GET /posts/:id

Get a single post.

**Auth:** None

---

#### POST /posts

Create a new post.

**Auth:** Required

**Request Body:**
```json
{
  "title": "Great yoga class today!",
  "body": "Really enjoyed the morning session with Coach Li."
}
```

**Validations:**
- User must not have an active ban
- User must not have exceeded 5 posts in the last hour

**Response:** `201 Created`

**Error Codes:** `ACCOUNT_BANNED`, `RATE_LIMIT_EXCEEDED`

---

#### POST /posts/:id/report

Report a post.

**Auth:** Required

**Request Body:**
```json
{
  "reason": "spam",
  "details": "This post is advertising an external service"
}
```

**Response:** `201 Created`

**Notes:** Deduplicated per reporter — a user cannot report the same post twice.

---

#### GET /moderation/reports

List post reports for review.

**Auth:** Moderator, Administrator

**Query Parameters:** `page`, `per_page`

---

#### GET /moderation/cases

List moderation cases.

**Auth:** Moderator, Administrator

**Query Parameters:** `page`, `per_page`

---

#### GET /moderation/cases/:id

Get moderation case detail.

**Auth:** Moderator, Administrator

---

#### POST /moderation/cases/:id/action

Take action on a moderation case.

**Auth:** Moderator, Administrator

**Request Body:**
```json
{
  "action_type": "content_removed",
  "notes": "Post contained spam links"
}
```

**Side Effect:** Closes the moderation case.

---

#### POST /moderation/bans

Apply an account ban.

**Auth:** Moderator, Administrator

**Request Body:**
```json
{
  "user_id": "uuid",
  "reason": "Repeated spam posting after warning",
  "permanent": false,
  "duration_days": 30
}
```

**Response:** `201 Created`

---

#### POST /moderation/bans/:id/revoke

Revoke an active ban.

**Auth:** Moderator, Administrator

**Response:** `200 OK`

---

### Tickets

#### POST /tickets

Create a support/exception ticket.

**Auth:** Required

**Request Body:**
```json
{
  "type": "delivery_issue",
  "priority": "high",
  "subject": "Order not delivered",
  "description": "My order OR-20260410-001 was marked shipped but never arrived"
}
```

**Response:** `201 Created`
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "ticket_number": "TK-20260414-0001",
    "status": "open",
    "type": "delivery_issue",
    "priority": "high",
    "response_due_at": "2026-04-14T14:30:00Z",
    "resolution_due_at": "2026-04-17T10:30:00Z",
    "created_at": "2026-04-14T10:30:00Z"
  }
}
```

---

#### GET /tickets

List tickets.

**Auth:** Required (members see own; staff see all)

**Query Parameters:** `status`, `priority`, `page`, `per_page`

---

#### GET /tickets/:id

Get ticket detail.

**Auth:** Required

---

#### PATCH /tickets/:id/status

Update ticket status.

**Auth:** Required

**Request Body:**
```json
{
  "status": "in_progress"
}
```

**Valid Transitions:**
- `open` → `acknowledged`, `in_progress`, `closed`
- `acknowledged` → `in_progress`, `waiting_on_member`, `closed`
- `in_progress` → `waiting_on_member`, `waiting_on_staff`, `resolved`, `escalated`
- `waiting_on_member` → `in_progress`, `closed`
- `waiting_on_staff` → `in_progress`
- `escalated` → `in_progress`, `closed`
- `resolved` → `closed`, `reopened`
- `reopened` → `in_progress`, `closed`

---

#### POST /tickets/:id/assign

Assign a ticket to a staff member.

**Auth:** Staff, Administrator

**Request Body:**
```json
{
  "assignee_id": "uuid"
}
```

---

#### POST /tickets/:id/comments

Add a comment to a ticket.

**Auth:** Required

**Request Body:**
```json
{
  "body": "I've checked the tracking number and it shows delivered to the wrong building.",
  "internal": false
}
```

`internal: true` comments are only visible to staff.

---

#### POST /tickets/:id/resolve

Mark a ticket as resolved.

**Auth:** Required

**Request Body:**
```json
{
  "resolution_code": "redelivered",
  "resolution_summary": "Package was located and redelivered to correct address"
}
```

---

#### POST /tickets/:id/close

Close a ticket.

**Auth:** Required

---

### Administration

#### GET /admin/config/:key

Get a system configuration value.

**Auth:** Administrator

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "key": "session.idle_timeout_minutes",
    "value": "30",
    "version": 3,
    "updated_at": "2026-04-10T08:00:00Z"
  }
}
```

---

#### PATCH /admin/config/:key

Update a system configuration value. Uses optimistic locking.

**Auth:** Administrator

**Request Body:**
```json
{
  "value": "45",
  "version": 3
}
```

**Error Codes:** `VERSION_CONFLICT`

---

#### GET /admin/feature-flags

List all feature flags.

**Auth:** Administrator

**Response:** `200 OK`
```json
{
  "success": true,
  "data": [
    {
      "key": "new_checkout_flow",
      "enabled": true,
      "cohort_percent": 10,
      "target_roles": ["staff"],
      "version": 2,
      "description": "Redesigned checkout page"
    }
  ]
}
```

---

#### PATCH /admin/feature-flags/:key

Update a feature flag. Uses optimistic locking.

**Auth:** Administrator

**Request Body:**
```json
{
  "enabled": true,
  "cohort_percent": 25,
  "target_roles": ["staff", "member"],
  "version": 2
}
```

---

#### GET /admin/audit-logs

Query audit logs.

**Auth:** Administrator

**Query Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| `resource` | string | Filter by resource type (e.g., `order`, `registration`) |
| `action` | string | Filter by action (e.g., `create`, `update`, `delete`) |
| `resource_id` | string | Filter by specific resource ID |
| `actor_id` | string | Filter by actor user ID |
| `page` | int | Page number |
| `per_page` | int | Items per page |

**Response:** `200 OK` — Paginated audit log entries with `old_state`, `new_state`, `metadata` JSON fields

---

#### GET /admin/kpis

Get KPI dashboard data.

**Auth:** Administrator

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "total_users": 1250,
    "active_sessions": 24,
    "total_orders": 3891,
    "pending_tickets": 17,
    "open_moderation_cases": 5
  }
}
```

---

#### GET /admin/jobs

Get job queue status.

**Auth:** Administrator

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "pending": 12,
    "running": 3,
    "completed": 1847,
    "failed": 2
  }
}
```

---

#### POST /admin/backups

Trigger a database backup.

**Auth:** Administrator

**Response:** `201 Created`
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "status": "completed",
    "artifact_path": "./backups/backup-20260414-103000.sql.enc",
    "encrypted": true,
    "expires_at": "2026-05-14T10:30:00Z"
  }
}
```

---

#### GET /admin/backups

List backup runs.

**Auth:** Administrator

**Query Parameters:** `page`, `per_page`

---

#### POST /admin/restore

Initiate a restore from a backup.

**Auth:** Administrator

**Request Body:**
```json
{
  "backup_id": "uuid",
  "reason": "Recovering from accidental bulk deletion",
  "dry_run": true
}
```

**Response:** `201 Created`

---

#### GET /admin/archives

List archive runs.

**Auth:** Administrator

---

#### POST /admin/archives

Trigger data archival (records older than 24 months).

**Auth:** Administrator

**Response:** `201 Created`
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "status": "running",
    "threshold_date": "2024-04-14T00:00:00Z",
    "rows_moved": 0
  }
}
```

---

### Import / Export

#### POST /imports

Upload a file for import.

**Auth:** Administrator

**Request Body:**
```json
{
  "filename": "members-april.csv",
  "checksum": "sha256:abcdef...",
  "template_type": "members",
  "storage_path": "/uploads/imports/members-april.csv"
}
```

**Response:** `201 Created`

**Error Codes:** `DUPLICATE_FILE` (same checksum already imported)

---

#### GET /imports

List import jobs.

**Auth:** Administrator

**Query Parameters:** `page`, `per_page`

---

#### GET /imports/:id

Get import job detail including row-level validation results.

**Auth:** Administrator

---

#### POST /imports/:id/apply

Apply a validated import.

**Auth:** Administrator

**Validations:** Import must be in `validated` status.

**Error Codes:** `IMPORT_NOT_VALIDATED`

---

#### POST /exports

Create an export job.

**Auth:** Administrator

**Request Body:**
```json
{
  "template_type": "orders",
  "format": "csv",
  "filters": {
    "status": "delivered",
    "date_from": "2026-01-01",
    "date_to": "2026-03-31"
  }
}
```

**Response:** `201 Created`

---

#### GET /exports

List export jobs.

**Auth:** Administrator

**Query Parameters:** `page`, `per_page`

---

#### GET /exports/:id/download

Download a completed export file.

**Auth:** Administrator

**Response:** File download (CSV/Excel)

---

### Health

#### GET /health

Health check endpoint.

**Auth:** None

**Response:** `200 OK`
```json
{
  "status": "ok"
}
```
