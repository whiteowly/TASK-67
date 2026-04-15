# Local Development (Optional, Non-Docker Path)

> **This is not part of the supported run path.**
> The supported and verified way to run CampusRec is `docker-compose up`
> from the repo root, as documented in `README.md`. Do not use this
> path for evaluation, smoke testing, or CI.

This document exists for contributors who want to iterate on the Go
backend without rebuilding a Docker image on every change. It assumes
you already have:

- Go toolchain installed (matching `go.mod`)
- A local PostgreSQL 16 reachable via a connection string
- `templ` CLI installed (`go install github.com/a-h/templ/cmd/templ@v0.3.1001`)
  if you intend to modify `web/templates/*.templ`

## One-time setup

```bash
# Provide required config via env vars
export DATABASE_URL=postgres://campusrec:campusrec@localhost:5432/campusrec?sslmode=disable
export SESSION_SECRET=local-dev-secret-at-least-32-characters
export PAYMENT_MERCHANT_KEY=your-merchant-key-for-payment-verification

# Bootstrap database
go run ./cmd/server -migrate up
go run ./cmd/server -seed
```

## Run the server

```bash
go run ./cmd/server
```

## Reset / re-seed

```bash
./init_db.sh
```

`init_db.sh` is idempotent and uses `DATABASE_URL` if set, otherwise it
targets the Compose database if running.

## Why this is not the supported path

- It depends on host-side Postgres and env vars — divergent from CI.
- It bypasses the layered Dockerfile that exercises the production
  build and entrypoint.
- The smoke-check section in `README.md` is only validated against the
  Docker stack.

For everything you would otherwise verify locally, prefer:

```bash
docker-compose up
```

and the smoke-check curls in `README.md`.
