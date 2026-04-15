.PHONY: build run test lint migrate-up migrate-down migrate-create seed templ-generate clean

# Variables
GO := go
GOBIN := ./bin
APP_NAME := campusrec
MAIN := ./cmd/server
DB_URL ?= postgres://campusrec:campusrec@localhost:5432/campusrec?sslmode=disable

# Build
build: templ-generate
	$(GO) build -o $(GOBIN)/$(APP_NAME) $(MAIN)

run: templ-generate
	$(GO) run $(MAIN)

# Database
migrate-up:
	$(GO) run $(MAIN) -migrate up

migrate-down:
	$(GO) run $(MAIN) -migrate down

migrate-status:
	$(GO) run $(MAIN) -migrate status

seed:
	$(GO) run $(MAIN) -seed

# Templ
templ-generate:
	templ generate

# Test
test:
	$(GO) test ./... -v -count=1

test-unit:
	$(GO) test ./internal/... -v -count=1 -short

test-integration:
	$(GO) test ./tests/... -v -count=1

test-coverage:
	$(GO) test ./... -coverprofile=coverage.out -count=1
	$(GO) tool cover -html=coverage.out -o coverage.html

# Lint
lint:
	golangci-lint run ./...

# Clean
clean:
	rm -rf $(GOBIN)
	rm -f coverage.out coverage.html
