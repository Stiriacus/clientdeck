# ── Build Stage ───────────────────────────────────────────────
FROM golang:1.26.5-alpine AS builder

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /build

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build the binary
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o vitrine ./cmd/vitrine

# ── Runtime Stage ─────────────────────────────────────────────
FROM scratch

# Timezone data and CA certs for TLS
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /build/vitrine /vitrine

# Store data in a volume
VOLUME ["/data"]

ENV VITRINE_ADDR=":8080"
ENV VITRINE_DB_PATH="/data/vitrine.db"
ENV VITRINE_THEME="plain"
ENV VITRINE_LOG_LEVEL="info"

EXPOSE 8080

ENTRYPOINT ["/vitrine"]
