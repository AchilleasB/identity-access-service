# Identity Access Service

A microservice for authentication and authorization in the Baby Kliniek system. Built with Go, following hexagonal architecture principles.

## Overview

This service handles:
- **User Registration** - Parents register with email and receive an access code
- **Authentication** - Login with email/password, receive JWT token
- **Token Management** - JWT signing with RSA, token blacklisting on logout
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

| Method | Endpoint | Description | Auth Required |
|--------|----------|-------------|---------------|
| `POST` | `/register` | Register a new parent | No |
| `POST` | `/login` | Authenticate and get JWT | No |
| `POST` | `/logout` | Invalidate JWT token | Yes (Bearer) |
| `GET` | `/health` | Detailed health status | No |
| `GET` | `/health/live` | Liveness probe | No |
| `GET` | `/health/ready` | Readiness probe | No |

## Security

- Passwords hashed with **bcrypt**
- JWTs signed with **RS256** (RSA + SHA256)
- Tokens stored as **SHA256 hashes** (not raw)
- HTTPS handled by OKD Route (TLS termination)
- Secrets managed via OKD Secrets

## License

MIT