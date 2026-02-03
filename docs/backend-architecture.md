# Backend Architecture

This document describes the architecture of the Contest Maker 150 backend, built using Go with Clean Architecture principles.

## Overview

The backend follows **Clean Architecture** (also known as Hexagonal Architecture), which separates concerns into distinct layers:

```
┌─────────────────────────────────────────────────────────────┐
│                        HTTP Layer                           │
│                   (Handlers + Middleware)                   │
├─────────────────────────────────────────────────────────────┤
│                       Service Layer                         │
│                    (Business Logic)                         │
├─────────────────────────────────────────────────────────────┤
│                      Repository Layer                       │
│                     (Data Access)                           │
├─────────────────────────────────────────────────────────────┤
│                    Infrastructure Layer                     │
│            (Database, Config, Logging, Telemetry)           │
├─────────────────────────────────────────────────────────────┤
│                       Domain Layer                          │
│              (Entities, Interfaces, Errors)                 │
└─────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
backend/
├── cmd/
│   └── api/
│       └── main.go           # Application entry point
├── internal/
│   ├── domain/               # Core business logic
│   │   ├── user.go           # User entity & repository interface
│   │   ├── problem.go        # Problem entity & repository interface
│   │   ├── contest.go        # Contest entity & repository interface
│   │   ├── submission.go     # Submission entity & repository interface
│   │   └── errors.go         # Domain-specific errors
│   ├── handler/              # HTTP request handlers
│   │   ├── auth_handler.go
│   │   ├── user_handler.go
│   │   ├── problem_handler.go
│   │   └── contest_handler.go
│   ├── middleware/           # HTTP middleware
│   │   ├── auth.go           # JWT authentication
│   │   ├── cors.go           # CORS configuration
│   │   ├── logging.go        # Request logging
│   │   ├── metrics.go        # Prometheus metrics
│   │   └── tracing.go        # OpenTelemetry tracing
│   ├── repository/           # Data access implementations
│   │   ├── user_repository.go
│   │   ├── problem_repository.go
│   │   ├── contest_repository.go
│   │   └── submission_repository.go
│   ├── service/              # Business logic
│   │   ├── user_service.go
│   │   ├── problem_service.go
│   │   └── contest_service.go
│   ├── infrastructure/       # External systems
│   │   ├── config.go         # Environment configuration
│   │   ├── database.go       # PostgreSQL connection
│   │   ├── logger.go         # Zap structured logging
│   │   └── telemetry.go      # OpenTelemetry setup
│   └── data/                 # Embedded data
│       ├── neetcode150.json  # Problem set
│       └── seeder.go         # Database seeding
└── Dockerfile
```

## Layer Responsibilities

### Domain Layer (`internal/domain/`)

The domain layer contains the core business entities and interfaces. It has **no dependencies** on other layers.

**Key components:**

- **Entities**: User, Problem, Contest, Submission - represent core business objects
- **Repository Interfaces**: Abstract data access operations
- **Value Objects**: Difficulty, ContestStatus - encapsulate validation
- **Domain Errors**: Type-safe error handling

```go
// Example: Repository interface in domain layer
type UserRepository interface {
    Create(ctx context.Context, user *User) error
    FindByID(ctx context.Context, id uuid.UUID) (*User, error)
    FindByEmail(ctx context.Context, email string) (*User, error)
    Update(ctx context.Context, user *User) error
    Delete(ctx context.Context, id uuid.UUID) error
    GetSolvedProblemIDs(ctx context.Context, id uuid.UUID) ([]uuid.UUID, error)
}
```

### Repository Layer (`internal/repository/`)

Implements domain repository interfaces using GORM for database operations.

**Characteristics:**

- Concrete implementations of domain interfaces
- Uses GORM for PostgreSQL interactions
- Handles database-specific concerns (constraints, transactions)
- Maps domain errors to database errors

```go
// Example: Repository implementation
func (r *userRepository) Create(ctx context.Context, user *domain.User) error {
    if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
        if isUniqueConstraintError(err) {
            return domain.ErrUserAlreadyExists
        }
        return err
    }
    return nil
}
```

### Service Layer (`internal/service/`)

Contains business logic and orchestrates operations across repositories.

**Key services:**

- **UserService**: Registration, authentication, JWT management
- **ProblemService**: Problem retrieval, contest problem selection
- **ContestService**: Contest lifecycle management

**Design principles:**

- Services receive repository interfaces via dependency injection
- Complex operations use Go concurrency patterns
- Context propagation for tracing and cancellation

### Handler Layer (`internal/handler/`)

HTTP request handlers that parse requests and return responses.

**Responsibilities:**

- Request validation and binding
- Error response formatting
- HTTP status code selection
- Context extraction (user ID from JWT)

### Middleware Layer (`internal/middleware/`)

Request/response interceptors for cross-cutting concerns.

| Middleware | Purpose |
|------------|---------|
| Auth | JWT token validation |
| CORS | Cross-origin request handling |
| Logging | Structured request logging |
| Metrics | Prometheus metrics collection |
| Tracing | OpenTelemetry span creation |

### Infrastructure Layer (`internal/infrastructure/`)

External system integrations and configuration.

**Components:**

- **Config**: Environment variable loading with defaults
- **Database**: GORM PostgreSQL setup with connection pooling
- **Logger**: Zap structured logging
- **Telemetry**: OpenTelemetry tracer and Prometheus metrics

## Dependency Flow

```
main.go
    ↓
infrastructure (Config, DB, Logger, Telemetry)
    ↓
repositories (implementing domain interfaces)
    ↓
services (business logic, depends on repositories)
    ↓
handlers (HTTP, depends on services)
    ↓
middleware (wraps handlers)
    ↓
gin.Router
```

## Key Design Decisions

### 1. Interface-Based Repositories

All data access is through interfaces defined in the domain layer, enabling:
- Easy testing with mocks
- Swappable implementations
- Clear contracts

### 2. Embedded Problem Data

NeetCode 150 problems are embedded in the binary:
```go
//go:embed neetcode150.json
var neetcode150Data []byte
```

Benefits:
- No external dependencies at runtime
- Fast startup (no API calls)
- Consistent data across deployments

### 3. JWT with Refresh Tokens

Two-token authentication:
- **Access Token**: Short-lived (15m), used for API calls
- **Refresh Token**: Long-lived (7 days), used to obtain new access tokens

### 4. Context Propagation

All methods accept `context.Context` for:
- Request cancellation
- Timeout handling
- Trace propagation

### 5. GORM for ORM

Chosen for:
- Auto-migration support
- PostgreSQL-specific features (JSONB, arrays)
- Relationship handling
- Query building

## Error Handling

Domain errors are defined as sentinel values:

```go
var (
    ErrUserNotFound      = errors.New("user not found")
    ErrInvalidCredentials = errors.New("invalid credentials")
    ErrContestNotActive  = errors.New("contest not active")
)
```

Handlers map domain errors to HTTP status codes:

```go
switch err {
case domain.ErrUserNotFound:
    c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
case domain.ErrInvalidCredentials:
    c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
default:
    c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal error"})
}
```

## Testing Strategy

| Layer | Testing Approach |
|-------|------------------|
| Domain | Unit tests, no mocks needed |
| Repository | Integration tests with test database |
| Service | Unit tests with mocked repositories |
| Handler | HTTP tests with mocked services |
| E2E | Docker-based full stack tests |

## Extensibility Points

The architecture supports future enhancements:

1. **LeetCode API Integration**: Add a new service for API sync
2. **Real-time Updates**: Add WebSocket handler layer
3. **Caching**: Implement caching repository decorator
4. **Rate Limiting**: Add rate limiting middleware
5. **Multi-tenant**: Add tenant context to domain layer
