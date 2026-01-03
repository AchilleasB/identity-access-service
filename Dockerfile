FROM golang:1.24 as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build both binaries
RUN CGO_ENABLED=0 GOOS=linux go build -o identity-api ./cmd/api/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o identity-relay ./cmd/relay/main.go

# Final stage: single image with both binaries
FROM alpine:latest

WORKDIR /app

# Copy both binaries
COPY --from=builder /app/identity-api .
COPY --from=builder /app/identity-relay .

RUN chmod 755 ./identity-api ./identity-relay

EXPOSE 8080

# Default command (overridden by Kubernetes deployment)
CMD ["./identity-api"]

