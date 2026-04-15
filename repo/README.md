# CampusRec

A private-network operational platform for recreation centers, wellness facilities, and training studios. Manages session registration, seat utilization, attendance, commerce, moderation, and administration.

## Tech Stack

- **Backend**: Go + Gin
- **Frontend**: Templ-rendered English UI
- **Database**: PostgreSQL 16
- **Auth**: Local username/password with server-side sessions
- **Authorization**: RBAC (Member, Staff, Moderator, Administrator)

## Quick Start

Run the entire application with one command:

```bash
docker compose up --build
```

The app will be available at **http://127.0.0.1:8080**.

This automatically starts PostgreSQL, runs migrations, seeds sample data, and starts the server. No manual env files, exports, or pre-steps required.

### Default Seed Users

| Username | Password | Role |
|----------|----------|------|
| admin | Seed@Pass1234 | Administrator |
| staff1 | Seed@Pass1234 | Staff |
| mod1 | Seed@Pass1234 | Moderator |
| member1 | Seed@Pass1234 | Member |
| member2 | Seed@Pass1234 | Member |

## Running Tests

### Go unit + integration tests

```bash
./run_tests.sh
```

Starts a test database in Docker, runs all Go unit and integration tests, cleans up, and reports pass/fail.

### Browser E2E tests (Playwright)

```bash
./run_e2e.sh
```

Starts the full app stack in Docker, runs Playwright browser tests against real UI pages, produces screenshots in `e2e/screenshots/`.

### Database Init

```bash
./init_db.sh
```

Explicit, idempotent database bootstrap. Runs migrations and seeds data. Uses the Compose database if running, or a `DATABASE_URL` you provide.

## API Endpoints

### Public
- `GET /health` - Health check
- `POST /api/v1/auth/register` - Register new account
- `POST /api/v1/auth/login` - Login
- `GET /api/v1/catalog/sessions` - Browse sessions
- `GET /api/v1/catalog/sessions/:id` - Session detail
- `GET /api/v1/catalog/products` - Browse products
- `GET /api/v1/catalog/products/:id` - Product detail
- `GET /api/v1/posts` - List community posts

### Authenticated (Member+)
- `POST /api/v1/auth/logout` - Logout
- `GET /api/v1/users/me` - Get profile
- `PATCH /api/v1/users/me` - Update profile
- `GET|POST /api/v1/addresses` - List / create addresses
- `GET|PATCH|DELETE /api/v1/addresses/:id` - Address CRUD
- `POST /api/v1/registrations` - Register for session
- `GET /api/v1/registrations` - List own registrations
- `GET /api/v1/registrations/:id` - Get registration detail
- `POST /api/v1/registrations/:id/cancel` - Cancel registration
- `POST /api/v1/registrations/:id/approve` - Approve registration (staff/admin)
- `POST /api/v1/registrations/:id/reject` - Reject registration (staff/admin)
- `GET /api/v1/cart` - Get cart
- `POST /api/v1/cart/items` - Add to cart
- `DELETE /api/v1/cart/items/:id` - Remove from cart
- `POST /api/v1/checkout` - Checkout cart
- `POST /api/v1/buy-now` - Buy Now (ephemeral checkout)
- `GET /api/v1/orders` - List orders
- `GET /api/v1/orders/:id` - Get order detail
- `POST /api/v1/orders/:id/pay` - Create payment request
- `POST /api/v1/posts` - Create post
- `POST /api/v1/posts/:id/report` - Report post
- `POST /api/v1/tickets` - Create ticket
- `GET /api/v1/tickets` - List tickets
- `GET /api/v1/tickets/:id` - Get ticket detail
- `PATCH /api/v1/tickets/:id/status` - Update ticket status
- `POST /api/v1/tickets/:id/comments` - Add ticket comment
- `POST /api/v1/tickets/:id/resolve` - Resolve ticket
- `POST /api/v1/tickets/:id/close` - Close ticket
- `POST /api/v1/attendance/leave` - Start temporary leave
- `POST /api/v1/attendance/leave/:id/return` - End temporary leave

### Staff / Admin
- `POST /api/v1/attendance/checkin` - Staff check-in
- `GET /api/v1/attendance/exceptions` - List occupancy exceptions
- `POST /api/v1/shipments` - Create shipment
- `GET /api/v1/shipments` - List shipments
- `PATCH /api/v1/shipments/:id/status` - Update shipment status
- `POST /api/v1/shipments/:id/pod` - Record proof of delivery
- `POST /api/v1/shipments/:id/exception` - Report shipment exception
- `POST /api/v1/tickets/:id/assign` - Assign ticket (staff/admin)

### Moderator / Admin
- `GET /api/v1/moderation/reports` - List reports
- `GET /api/v1/moderation/cases` - List moderation cases
- `GET /api/v1/moderation/cases/:id` - Get case detail
- `POST /api/v1/moderation/cases/:id/action` - Action moderation case
- `POST /api/v1/moderation/bans` - Apply ban
- `POST /api/v1/moderation/bans/:id/revoke` - Revoke ban

### Administrator Only
- `GET /api/v1/admin/config` - List system configuration
- `PATCH /api/v1/admin/config/:key` - Update config key
- `GET /api/v1/admin/feature-flags` - List feature flags
- `PATCH /api/v1/admin/feature-flags/:key` - Update feature flag
- `GET /api/v1/admin/audit-logs` - Audit logs
- `GET /api/v1/admin/kpis` - KPI dashboard
- `GET /api/v1/admin/jobs` - Job queue status
- `POST /api/v1/admin/backups` - Run backup
- `GET /api/v1/admin/backups` - List backups
- `POST /api/v1/admin/restore` - Restore from backup
- `POST /api/v1/admin/archives` - Run archive
- `GET /api/v1/admin/archives` - List archives
- `POST /api/v1/admin/refunds/:id/reconcile` - Reconcile refund
- `POST /api/v1/admin/registrations/override` - Admin override registration
- `POST /api/v1/imports` - Upload import
- `GET /api/v1/imports` - List imports
- `GET /api/v1/imports/:id` - Get import detail
- `POST /api/v1/imports/:id/validate` - Validate import
- `POST /api/v1/imports/:id/apply` - Apply import
- `POST /api/v1/exports` - Create export
- `GET /api/v1/exports` - List exports
- `GET /api/v1/exports/:id/download` - Download export

### Payment Callback (public, from local payment bridge)
- `POST /api/v1/payments/callback` - Payment gateway callback

### Web Pages
- `/` - Home
- `/login`, `/register` - Auth pages
- `/catalog` - Catalog browse (sessions + products)
- `/catalog/sessions/:id` - Session detail
- `/catalog/products/:id` - Product detail
- `/my/orders` - Order list
- `/my/orders/:id` - Order detail with payment countdown
- `/my/orders/:id/pay` - Create payment request
- `/my/cart/add` - Add to cart
- `/my/buy-now` - Buy now
- `/my/checkout` - Checkout
- `/my/registrations` - Registration list
- `/my/addresses` - Address management
- `/admin` - Admin dashboard, config, feature flags, audit logs

## Project Structure

```
cmd/server/         - Application entrypoint
config/             - Configuration loader (env vars, sensible defaults)
db/migrations/      - PostgreSQL migrations (17 files, all phases)
internal/
  handler/api/      - JSON API handlers
  handler/web/      - Templ page handlers
  middleware/       - Auth, RBAC, audit, recovery middleware
  model/            - Domain models
  repo/             - Data access layer (pgx)
  response/         - API response envelope
  router/           - Route wiring
  service/          - Business logic
  util/             - Password hashing, pagination, timezone
  validator/        - Input validation
web/templates/      - Templ templates
web/static/         - CSS, JS
tests/              - Go integration tests
e2e/                - Playwright browser E2E tests
```

## Local Development (without Docker)

For contributors with Go and PostgreSQL installed locally:

```bash
# Provide required config via env vars
export DATABASE_URL=postgres://campusrec:campusrec@localhost:5432/campusrec?sslmode=disable
export SESSION_SECRET=local-dev-secret-at-least-32-characters
export PAYMENT_MERCHANT_KEY=your-merchant-key-for-payment-verification

# Bootstrap database
go run ./cmd/server -migrate up
go run ./cmd/server -seed

# Run server
go run ./cmd/server
```

This is a secondary path. The primary workflow is `docker compose up --build`.

## Implementation Status

- [x] Phase 1: Auth, RBAC, Catalog, Addresses, Config, Audit
- [x] Phase 2: Registration, Seat Control, Waitlist, Attendance
- [x] Phase 3: Commerce, Payment, Refunds, Logistics
- [x] Phase 4: Moderation, Tickets, SLAs, Import/Export, KPIs
- [x] Phase 5: Backup, Restore, Archive, Canary Release
