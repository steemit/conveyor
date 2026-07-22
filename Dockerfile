# Build stage — CGO_ENABLED=1 because mattn/go-sqlite3 requires it.
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git gcc musl-dev ca-certificates tzdata

WORKDIR /app

# Cache dependency download
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build with version injection via -ldflags
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags "-X github.com/steemit/conveyor/internal/server.Version=$(git rev-parse --short HEAD 2>/dev/null || echo dev)" \
    -o conveyor ./cmd/conveyor

# --- Runtime stage ---
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata sqlite-libs

WORKDIR /app

# Binary
COPY --from=builder /app/conveyor .

# Runtime assets: TOML configs + account lists
COPY --from=builder /app/config config
COPY --from=builder /app/user-data user-data

EXPOSE 8080

ENV PORT=8080
ENV NODE_ENV=production

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q --spider http://localhost:8080/ || exit 1

CMD ["./conveyor"]
