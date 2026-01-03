# Identity Access Service

A microservice for authentication and authorization in the Baby Kliniek system. Built with Go, following hexagonal architecture principles.

## Overview

This service handles:
- **Google OAuth Authentication** - Users authenticate via Google, no passwords stored
- **User Registration** - Admins register Parents and other Admins
- **JWT Token Management** - System JWTs signed with RSA for stateless authorization across microservices
- **Token Lifecycle Management** - Redis-backed session tracking, logout, and token revocation
- **Role-Based Access Control** - Admin and Parent roles with route-level enforcement
- **Event-Driven Architecture** - Transactional outbox pattern with PostgreSQL NOTIFY for reliable event publishing
- **Health Checks** - Liveness and readiness probes for container orchestration

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Hexagonal Architecture                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   ┌─────────────┐     ┌─────────────────┐     ┌─────────────────┐       │
│   │  Handlers   │───▶│    Services     │────▶│   Repository    │       │
│   │  (HTTP)     │     │  (Business)     │     │  (PostgreSQL)   │       │
│   └─────────────┘     └─────────────────┘     └─────────────────┘       │
│        Adapters              Core                   Adapters            │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

## Authentication Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Google OAuth Flow                               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  1. GET /login                                                          │
│     → Generate cryptographic state                                      │
│     → Store state in HttpOnly cookie                                    │
│     → Return Google OAuth redirect URL                                  │
│                                                                         │
│  2. User authenticates with Google                                      │
│     → Google verifies credentials                                       │
│     → Google redirects to callback with code + state                    │
│                                                                         │
│  3. GET /auth/google/callback                                           │
│     → Verify state matches cookie (CSRF protection)                     │
│     → Exchange authorization code for tokens                            │
│     → Verify Google ID token signature via JWKS                         │
│     → Extract email from verified token                                 │
│     → Lookup user in database by email                                  │
│     → Store active session in Redis                                     │
│     → Issue system JWT if user exists                                   │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

## Token Lifecycle Management

The service uses **Redis** as a distributed cache for token lifecycle management, providing:

### Active Session Tracking
When a user authenticates, their session metadata (JTI and expiration) is stored in Redis
### Token Blacklisting
Revoked tokens are stored in Redis to prevent reuse:

### Caching Strategy
- **Warm Requests**: Subsequent authorization checks benefit from Redis's in-memory performance, avoiding database lookups for token validation
- **Cold Requests**: Initial requests require full JWT signature verification and Redis blacklist check
- **TTL-Based Cleanup**: Redis automatically evicts expired entries, ensuring efficient memory usage

## Logout & Discharge Endpoints

### POST /logout
Allows authenticated users (Admin or Parent) to invalidate their current JWT:
- Extracts the JWT from the request context
- Adds the token's JTI to the Redis blacklist
- Token remains blacklisted until its natural expiration

### POST /discharge
Admin-only endpoint to discharge a parent from the system:
- Revokes the parent's active session token (if any)
- Updates the parent's status to `Discharged` in the database
- Discharged parents cannot log in again

## Outbox Relay Pattern

The service implements the **Transactional Outbox Pattern** to ensure reliable event publishing to RabbitMQ without distributed transactions.

### How It Works
```
┌─────────────────────────────────────────────────────────────────────────┐
│                    Transactional Outbox Pattern                         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  1. Business Transaction                                                │
│     ┌──────────────────────────────────────────────┐                    │
│     │  BEGIN TRANSACTION                           │                    │
│     │    INSERT INTO parents (...)                 │                    │
│     │    INSERT INTO outbox_events (...)           │                    │
│     │  COMMIT                                      │                    │
│     └──────────────────────────────────────────────┘                    │
│                         │                                               │
│                         ▼                                               │
│  2. PostgreSQL Trigger fires pg_notify('outbox_channel', event_id)      │
│                         │                                               │
│                         ▼                                               │
│  3. Relay Service (separate process)                                    │
│     ┌──────────────────────────────────────────────┐                    │
│     │  LISTEN outbox_channel                       │                    │
│     │    → Receives notification                   │                    │
│     │    → Fetches event from outbox_events        │                    │
│     │    → Publishes to RabbitMQ                   │                    │
│     │    → Marks event as processed                │                    │
│     └──────────────────────────────────────────────┘                    │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```
### Benefits
- **Atomicity**: Event creation is part of the same transaction as business data
- **Reliability**: Events are never lost, even if RabbitMQ is temporarily unavailable
- **Exactly-once delivery**: Row locking prevents duplicate processing
- **Low latency**: PostgreSQL NOTIFY provides near real-time event propagation

## Project Structure
```
identity-access-service/
├── cmd/
│   ├── api/
│   │   └── main.go              # API server entry point
│   └── relay/
│       └── main.go              # Outbox relay service entry point
├── internal/
│   ├── adapters/
│   │   ├── handler/             # HTTP handlers
│   │   │   ├── auth_handler.go
│   │   │   ├── registration_handler.go
│   │   │   └── health_handler.go
│   │   ├── middleware/          # Middleware implementation
│   │   │   └── auth_middleware.go
│   │   ├── messaging/           # Message broker adapters
│   │   │   ├── rabbitmq.go
│   │   │   └── baby_publisher.go
│   │   ├── outbox/              # Outbox relay implementation
│   │   │   └── relay.go
│   │   └── repository/          # Database implementation
│   │       └── sql_repository.go
│   ├── core/
│   │   ├── domain/              # Domain models
│   │   │   └── user.go
│   │   ├── ports/               # Interfaces
│   │   │   ├── repository.go
│   │   │   ├── service.go
│   │   │   └── event.go
│   │   └── services/            # Business logic
│   │       ├── auth_service.go
│   │       └── registration_service.go
│   └── config/
│       ├── config.go            # API configuration
│       └── relay_config.go      # Relay configuration
├── openshift/                   # OKD/OpenShift deployment
│   ├── application.yaml         # API deployment
│   ├── relay.yaml               # Relay deployment
│   ├── database.yaml            # PostgreSQL resources
│   └── redis.yaml               # Redis resources
├── Dockerfile
├── go.mod
├── go.sum
└── .gitignore
```

## API Endpoints

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| `GET` | `/login` | None | Initiate Google OAuth, returns redirect URL |
| `GET` | `/auth/google/callback` | None | Handle OAuth callback, returns JWT |
| `POST` | `/register` | Admin | Register Admin or Parent (triggers outbox event for parents) |
| `POST` | `/logout` | Admin, Parent | Invalidate current JWT token |
| `POST` | `/discharge` | Admin | Discharge a parent and revoke their session |
| `GET` | `/health` | None | Detailed health status |
| `GET` | `/health/live` | None | Liveness probe |
| `GET` | `/health/ready` | None | Readiness probe |

## Security

- **No Password Storage** - Authentication delegated to Google OAuth
- **CSRF Protection** - State parameter with HttpOnly cookies
- **JWT Signing** - RS256 (RSA + SHA256) asymmetric encryption
- **Token Verification** - Google ID tokens verified via JWKS
- **Token Revocation** - Redis-backed blacklist for logout/discharge
- **Role-Based Access** - Admin-only registration and discharge endpoints
- **HTTPS** - TLS termination at OKD Route level

## License
MIT