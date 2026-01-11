# Testing Guide for Identity Access Service

This document explains the testing architecture and how to run tests for the Identity Access Service.

## Table of Contents

1. [Testing Philosophy](#testing-philosophy)
2. [Hexagonal Architecture & Testing](#hexagonal-architecture--testing)
3. [Test Structure](#test-structure)
4. [Running Tests](#running-tests)
5. [Mock Implementations](#mock-implementations)
6. [CI/CD Pipeline](#cicd-pipeline)

---

## Testing Philosophy

The Identity Access Service follows the **Testing Pyramid** approach:

```
                    ▲
                   ╱ ╲
                  ╱ E2E ╲        Few, slow, expensive
                 ╱───────╲
                ╱         ╲
               ╱Integration╲    Medium amount, moderate speed
              ╱─────────────╲
             ╱               ╲
            ╱   Unit Tests    ╲  Many, fast, cheap
           ╱───────────────────╲
```

- **Unit Tests**: Test individual components in isolation using mocks
- **Integration Tests**: Test components working together with real dependencies
- **E2E Tests**: Test the complete system (handled at deployment level)

---

## Hexagonal Architecture & Testing

The service follows **Hexagonal (Ports & Adapters) Architecture**, which makes testing easy:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              APPLICATION                                     │
│                                                                              │
│     DRIVING ADAPTERS           CORE               DRIVEN ADAPTERS           │
│    (HTTP Handlers)           (Domain)            (Infrastructure)           │
│                                                                              │
│    ┌───────────┐          ┌───────────┐          ┌───────────┐             │
│    │   Auth    │          │  Domain   │          │    SQL    │             │
│    │  Handler  │──────────│  Models   │──────────│Repository │             │
│    └───────────┘   PORT   │  (User,   │   PORT   └───────────┘             │
│                    │      │  Parent)  │    │                                │
│    ┌───────────┐   │      └───────────┘    │     ┌───────────┐             │
│    │Registration│  │                       │     │  RabbitMQ │             │
│    │  Handler  │──┼──────┬───────────┬────┼─────│ Publisher │             │
│    └───────────┘  │      │ Services  │    │     └───────────┘             │
│                   │      │  (Auth,   │    │                                │
│    ┌───────────┐  │      │  Regist.) │    │     ┌───────────┐             │
│    │  Health   │  │      └───────────┘    │     │   Redis   │             │
│    │  Handler  │──┼──────────────────────┼─────│   Client  │             │
│    └───────────┘  │                       │     └───────────┘             │
│                   │                       │                                │
│    ────────────────                       ────────────────────             │
│       PORTS                                     PORTS                       │
│    (Interfaces)                              (Interfaces)                   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Why This Matters for Testing

**Ports are interfaces** - they define contracts between components:

**Adapters implement ports** - we can swap implementations:

**Services depend on ports, not adapters:**

## Test Structure

```
identity-access-service/
└── test/
    ├── mocks/                      # Mock implementations
    │   ├── repository_mock.go      # MockUserRepository
    │   ├── publisher_mock.go       # MockBabyEventPublisher
    │   ├── redis_mock.go           # MockRedisClient
    │   └── test_helpers.go         # Helper functions
    │
    ├── api-unit-tests/             # API unit tests
    │   ├── registration_service_test.go
    │   ├── registration_handler_test.go
    │   ├── auth_handler_test.go
    │   └── health_handler_test.go
    │
    ├── api-integration-tests/      # API integration tests
    │   └── api_integration_test.go
    │
    ├── relay-unit-tests/           # Relay unit tests
    │   ├── publisher_test.go
    │   └── relay_test.go
    │
    └── relay-integration-tests/    # Relay integration tests
        └── relay_integration_test.go
```

---

## Mock Implementations

### MockUserRepository

Located in `test/mocks/repository_mock.go`

**Purpose:** Replaces the real PostgreSQL repository for unit testing.

**Features:**
- In-memory storage (maps)
- Call tracking (verify methods were called)
- Error injection (simulate failures)
- Thread-safe operations

### MockBabyEventPublisher

Located in `test/mocks/publisher_mock.go`

**Purpose:** Replaces RabbitMQ for relay testing.

---
