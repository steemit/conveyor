# Build stage — CGO_ENABLED=1 because mattn/go-sqlite3 requires it.
# golang:1.26-alpine tracks the latest 1.26.x patch, which carries the
# standard-library security fixes (audit 2026-08-18 T-005 requires >=1.26.6).
FROM golang:1.26-alpine AS builder

# Version is passed via build-arg (avoids needing .git in the build context,
# which .dockerignore excludes). Set with: docker build --build-arg VERSION=$(git rev-parse --short HEAD)
ARG VERSION=dev

RUN apk add --no-cache gcc musl-dev ca-certificates tzdata

WORKDIR /app

# Cache dependency download
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build with version injection via -ldflags
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags "-X github.com/steemit/conveyor/internal/server.Version=${VERSION}" \
    -o conveyor ./cmd/conveyor

# --- Runtime stage ---
FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata sqlite-libs

# Run as a non-root user (audit 2026-08-18 C-2). /app is owned by app so an
# eventual sqlite dialect override can still create its database file.
RUN addgroup -S app && adduser -S app -G app

WORKDIR /app

# Binary
COPY --from=builder /app/conveyor .

# Runtime assets: TOML configs + account lists
COPY --from=builder /app/config config
COPY --from=builder /app/user-data user-data

# RDS CA bundle (public AWS artifact, us-east-1 roots). Only consulted when
# DATABASE_SSL_ROOT_CERT points here; plain self-hosted deployments ignore it.
COPY --from=builder /app/certs certs

RUN chown -R app:app /app
USER app

EXPOSE 8080

ENV PORT=8080
ENV NODE_ENV=production

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q --spider http://localhost:8080/ || exit 1

CMD ["./conveyor"]
