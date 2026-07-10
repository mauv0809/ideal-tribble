# syntax=docker/dockerfile:1.7
# Cache mounts (Go module + build cache) below need BuildKit + the 1.7 frontend.

# ---- build stage ----
FROM golang:1.24-alpine AS builder
WORKDIR /src

# Cache deps separately for fast rebuilds.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# CGO is disabled: production uses the remote Turso database via the pure-Go
# libsql client, so the mattn/go-sqlite3 (cgo) driver is never needed at
# runtime. This lets us ship a distroless/static image.
# /root/.cache/go-build is Go's incremental compile cache — after the first
# build only changed packages recompile.
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux \
    go build -ldflags="-s -w" -o /out/app .

# ---- runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app

COPY --from=builder /out/app /app/app
# Migrations are read from disk at startup (config.MigrationsDir = "./migrations").
COPY --from=builder /src/migrations /app/migrations

EXPOSE 8080
USER nonroot:nonroot

# The proxy (kamal-proxy) health-checks GET /health over HTTP, so no Docker
# HEALTHCHECK is defined here.
ENTRYPOINT ["/app/app"]
