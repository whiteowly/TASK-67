FROM golang:1.25-bookworm AS builder

# Install templ
RUN go install github.com/a-h/templ/cmd/templ@v0.3.1001

WORKDIR /app

# Copy go.mod and go.sum first for cache
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Generate templ files
RUN templ generate

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/campusrec ./cmd/server

# Runtime
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y ca-certificates wget && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/bin/campusrec /app/campusrec
COPY --from=builder /app/web/static /app/web/static

EXPOSE 8080

CMD ["/app/campusrec"]
