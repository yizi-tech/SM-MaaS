# Build stage
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/mass-server ./cmd/server \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/seed-demo-users ./cmd/seed-demo-users

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app && adduser -S app -G app

WORKDIR /app

COPY --from=builder /app/mass-server /app/
COPY --from=builder /app/seed-demo-users /app/

# Non-root runtime user: reduce blast radius if an upload/static path is abused.
RUN mkdir -p /app/uploads && chown -R app:app /app

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/health || exit 1

USER app

ENTRYPOINT ["./mass-server"]