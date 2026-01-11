# =============================================================================
# Dockerfile for Identity Access Service
# =============================================================================
# Builds a single image containing both API and Relay binaries.
# The Kubernetes deployment specifies which binary to run via command override.
# =============================================================================

ARG APP_VERSION=unknown

# Initial stage: build both binaries
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG APP_VERSION
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-X main.Version=${APP_VERSION}" \
    -o identity-api ./cmd/api/main.go

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-X main.Version=${APP_VERSION}" \
    -o identity-relay ./cmd/relay/main.go

# Final stage: minimal runtime image
FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

# Security: Run as non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

COPY --from=builder /app/identity-api .
COPY --from=builder /app/identity-relay .
RUN chmod 755 ./identity-api ./identity-relay

USER appuser

EXPOSE 8080 8090

# Default command (overridden by Kubernetes deployment)
CMD ["./identity-api"]

