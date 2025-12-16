# Identity Access Service

A microservice for authentication and authorization in the Baby Kliniek system. Built with Go, following hexagonal architecture principles.

## Overview

This service handles:
- **Google OAuth Authentication** - Users authenticate via Google, no passwords stored
- **User Registration** - Admins register Parents and other Admins
- **JWT Token Management** - System JWTs signed with RSA for stateless authorization across microservices
- **Role-Based Access Control** - Admin and Parent roles with route-level enforcement
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
│  1. POST /login                                                         │
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
│     → Issue system JWT if user exists                                   │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

## Project Structure

```
identity-access-service/
├── cmd/
│   └── api/
│       └── main.go              # Application entry point
├── internal/
│   ├── adapters/
│   │   ├── handler/             # HTTP handlers
│   │   │   ├── auth_handler.go
│   │   │   ├── registration_handler.go
│   │   │   └── health_handler.go
│   │   └── repository/          # Database implementation
│   │       └── sql_repository.go
│   ├── core/
│   │   ├── domain/              # Domain models
│   │   │   └── user.go
│   │   ├── ports/               # Interfaces
│   │   │   └── repository.go
│   │   │   └── service.go
│   │   └── services/            # Business logic
│   │       ├── auth_service.go
│   │       └── registration_service.go
│   └── config/
│       └── config.go            # Configuration loading
├── openshift/                   # OKD/OpenShift deployment
│   ├── database.yaml            # PostgreSQL resources
│   └── application.yaml         # Application resources
├── Dockerfile
├── go.mod
├── go.sum
└── .gitignore
```

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/login` | Initiate Google OAuth, returns redirect URL |
| `GET` | `/auth/google/callback` | Handle OAuth callback, returns JWT |
| `POST` | `/register` | Register Admin or Parent |
| `GET` | `/health` | Detailed health status |
| `GET` | `/health/live` | Liveness probe |
| `GET` | `/health/ready` | Readiness probe |

## Security

- **No Password Storage** - Authentication delegated to Google OAuth
- **CSRF Protection** - State parameter with HttpOnly cookies
- **JWT Signing** - RS256 (RSA + SHA256) asymmetric encryption
- **Token Verification** - Google ID tokens verified via JWKS
- **Role-Based Access** - Admin-only registration endpoint
- **HTTPS** - TLS termination at OKD Route level

## License
MIT