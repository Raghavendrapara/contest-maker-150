# Go Backend Deep Dive: Learning Guide

This guide walks you through the Contest Maker 150 Go backend, explaining the patterns, flow, and reasoning behind each design decision. Use this as a reference while exploring the codebase.

---

## Table of Contents

1. [Project Structure Overview](#project-structure-overview)
2. [Application Startup Flow](#application-startup-flow)
3. [Request Lifecycle](#request-lifecycle)
4. [Clean Architecture Explained](#clean-architecture-explained)
5. [Key Go Patterns Used](#key-go-patterns-used)
6. [Dependency Injection Without Frameworks](#dependency-injection-without-frameworks)
7. [Error Handling Patterns](#error-handling-patterns)
8. [Concurrency in Practice](#concurrency-in-practice)
9. [Testing Strategies](#testing-strategies)
10. [Common Gotchas & Best Practices](#common-gotchas--best-practices)

---

## Project Structure Overview

```
backend/
├── cmd/
│   └── api/
│       └── main.go          # Entry point - wires everything together
├── internal/                 # Private packages (can't be imported by other projects)
│   ├── domain/              # Core business entities & interfaces (no dependencies)
│   │   ├── user.go
│   │   ├── contest.go
│   │   ├── problem.go
│   │   ├── submission.go
│   │   └── errors.go
│   ├── repository/          # Data access implementation (depends on domain)
│   │   ├── user_repository.go
│   │   ├── contest_repository.go
│   │   ├── problem_repository.go
│   │   └── submission_repository.go
│   ├── service/             # Business logic (depends on domain interfaces)
│   │   ├── user_service.go
│   │   ├── contest_service.go
│   │   └── problem_service.go
│   ├── handler/             # HTTP handlers (depends on services)
│   │   ├── auth_handler.go
│   │   ├── contest_handler.go
│   │   ├── problem_handler.go
│   │   └── user_handler.go
│   ├── middleware/          # HTTP middleware (cross-cutting concerns)
│   │   ├── auth.go
│   │   ├── cors.go
│   │   ├── logging.go
│   │   ├── metrics.go
│   │   └── tracing.go
│   ├── infrastructure/      # External concerns (config, DB, logging)
│   │   ├── config.go
│   │   ├── database.go
│   │   ├── logger.go
│   │   └── telemetry.go
│   └── data/                # Static data (problems JSON, seeder)
│       ├── neetcode150.json
│       └── seeder.go
└── Dockerfile
```

### Why `internal/`?

Go has a special rule: packages inside `internal/` can only be imported by packages in the same module. This prevents other projects from depending on your internal implementation details.

```go
// ✅ Works (same module)
import "github.com/contest-maker-150/backend/internal/domain"

// ❌ Fails (different module trying to import)
import "github.com/someone-else/backend/internal/domain"
```

---

## Application Startup Flow

The `main.go` file orchestrates the entire application startup. Let's trace through it:

```go
// cmd/api/main.go - Simplified flow

func main() {
    // 1. Load configuration from environment
    cfg := infrastructure.LoadConfig()
    
    // 2. Initialize structured logging
    logger := infrastructure.NewLogger(cfg.Server.Environment)
    
    // 3. Setup observability (tracing + metrics)
    shutdown := infrastructure.SetupTelemetry(ctx, cfg.Telemetry)
    defer shutdown(ctx)  // Clean shutdown on exit
    
    // 4. Connect to database
    db, err := infrastructure.NewDatabase(cfg.Database, logger)
    if err != nil {
        logger.Fatal("Failed to connect", zap.Error(err))
    }
    
    // 5. Create repositories (data access layer)
    userRepo := repository.NewUserRepository(db)
    problemRepo := repository.NewProblemRepository(db)
    contestRepo := repository.NewContestRepository(db)
    submissionRepo := repository.NewSubmissionRepository(db)
    
    // 6. Create services (business logic layer)
    userService := service.NewUserService(userRepo, submissionRepo, cfg.JWT)
    problemService := service.NewProblemService(problemRepo)
    contestService := service.NewContestService(contestRepo, problemService, submissionRepo)
    
    // 7. Create HTTP handlers
    authHandler := handler.NewAuthHandler(userService)
    contestHandler := handler.NewContestHandler(contestService)
    // ...
    
    // 8. Setup router with middleware
    router := gin.New()
    router.Use(middleware.LoggingMiddleware(logger))
    router.Use(middleware.MetricsMiddleware())
    // ... register routes
    
    // 9. Start server with graceful shutdown
    server := &http.Server{Addr: ":8080", Handler: router}
    go server.ListenAndServe()
    
    // 10. Wait for shutdown signal
    <-ctx.Done()
    server.Shutdown(context.Background())
}
```

### Key Insight: Dependency Direction

Notice how dependencies flow **inward**:

```
main.go → handlers → services → repositories → domain
            ↓           ↓            ↓
         (uses)      (uses)       (uses)
```

The `domain` package has **zero dependencies** on other packages. This is intentional - it's the core of your application and should be stable.

---

## Request Lifecycle

Let's trace a complete request: **POST /api/auth/login**

### Step 1: Request Hits Middleware Stack

```go
// Order matters! Each middleware wraps the next
router.Use(middleware.CORSMiddleware())           // 1. Handle CORS
router.Use(middleware.RequestIDMiddleware())      // 2. Generate request ID
router.Use(middleware.TracingMiddleware(tracer))  // 3. Start trace span
router.Use(middleware.LoggingMiddleware(logger))  // 4. Log request/response
router.Use(middleware.MetricsMiddleware())        // 5. Record metrics
```

### Step 2: Route Matched, Handler Called

```go
// handler/auth_handler.go
func (h *AuthHandler) Login(c *gin.Context) {
    // Parse JSON body
    var req domain.LoginRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "Invalid request"})
        return
    }
    
    // Call service layer
    response, err := h.userService.Login(c.Request.Context(), req)
    if err != nil {
        // Map domain errors to HTTP status codes
        handleError(c, err)
        return
    }
    
    c.JSON(200, response)
}
```

### Step 3: Service Executes Business Logic

```go
// service/user_service.go
func (s *UserService) Login(ctx context.Context, req domain.LoginRequest) (*domain.AuthResponse, error) {
    // 1. Find user by email
    user, err := s.userRepo.FindByEmail(ctx, req.Email)
    if err != nil {
        return nil, domain.ErrInvalidCredentials
    }
    
    // 2. Verify password
    if !verifyPassword(req.Password, user.PasswordHash) {
        return nil, domain.ErrInvalidCredentials
    }
    
    // 3. Generate JWT tokens
    tokens, err := s.generateTokens(user)
    if err != nil {
        return nil, err
    }
    
    return &domain.AuthResponse{User: user.ToResponse(), Tokens: tokens}, nil
}
```

### Step 4: Repository Accesses Database

```go
// repository/user_repository.go
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
    var user domain.User
    
    // GORM query with context (for tracing & cancellation)
    result := r.db.WithContext(ctx).Where("email = ?", email).First(&user)
    
    if result.Error != nil {
        if errors.Is(result.Error, gorm.ErrRecordNotFound) {
            return nil, domain.ErrUserNotFound
        }
        return nil, result.Error
    }
    
    return &user, nil
}
```

### Step 5: Response Bubbles Back Up

```
Database → Repository → Service → Handler → Middleware → Client
    ↑                                           ↓
 (user row)                              (JSON response)
```

---

## Clean Architecture Explained

### The Core Principle

> **Inner layers should not know about outer layers.**

```
┌────────────────────────────────────────────┐
│              Infrastructure                │  <- Frameworks, DB, HTTP
│  ┌──────────────────────────────────────┐  │
│  │            Handlers/Adapters         │  │  <- HTTP handlers, CLI
│  │  ┌────────────────────────────────┐  │  │
│  │  │           Services             │  │  │  <- Business logic
│  │  │  ┌──────────────────────────┐  │  │  │
│  │  │  │         Domain           │  │  │  │  <- Entities, interfaces
│  │  │  └──────────────────────────┘  │  │  │
│  │  └────────────────────────────────┘  │  │
│  └──────────────────────────────────────┘  │
└────────────────────────────────────────────┘
```

### How We Achieve This: Interfaces

The domain layer defines **interfaces** that outer layers must implement:

```go
// domain/user.go - Domain defines WHAT it needs
type UserRepository interface {
    Create(ctx context.Context, user *User) error
    FindByID(ctx context.Context, id uuid.UUID) (*User, error)
    FindByEmail(ctx context.Context, email string) (*User, error)
    Update(ctx context.Context, user *User) error
    Delete(ctx context.Context, id uuid.UUID) error
}
```

```go
// repository/user_repository.go - Repository implements HOW
type userRepository struct {
    db *gorm.DB
}

func NewUserRepository(db *gorm.DB) domain.UserRepository {
    return &userRepository{db: db}
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
    // GORM-specific implementation
}
```

### Why This Matters

1. **Testability**: Swap real database with mock in tests
2. **Flexibility**: Switch from PostgreSQL to MongoDB without changing services
3. **Clarity**: Each layer has a single responsibility

---

## Key Go Patterns Used

### 1. Functional Options Pattern

Used for configurable constructors:

```go
// Instead of many constructor parameters:
func NewServer(host string, port int, timeout time.Duration, ...) // ❌

// Use options:
type ServerOption func(*Server)

func WithTimeout(d time.Duration) ServerOption {
    return func(s *Server) {
        s.timeout = d
    }
}

func NewServer(opts ...ServerOption) *Server {
    s := &Server{timeout: 30 * time.Second} // defaults
    for _, opt := range opts {
        opt(s)
    }
    return s
}

// Usage:
server := NewServer(
    WithTimeout(60 * time.Second),
    WithHost("0.0.0.0"),
)
```

### 2. Context Propagation

Context carries request-scoped values and enables cancellation:

```go
func (s *ContestService) Create(ctx context.Context, userID uuid.UUID, req domain.CreateContestRequest) (*domain.Contest, error) {
    // Check if request was cancelled
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }
    
    // Pass context to all downstream calls
    problems, err := s.problemService.SelectForContest(ctx, userID, req.ProblemCount)
    if err != nil {
        return nil, err
    }
    
    return s.contestRepo.Create(ctx, contest)
}
```

### 3. Error Wrapping

Add context to errors without losing the original:

```go
import "fmt"

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
    if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
        // Wrap with context, original error is preserved
        return fmt.Errorf("failed to create user %s: %w", user.Email, err)
    }
    return nil
}

// Later, you can still check the original error:
if errors.Is(err, gorm.ErrDuplicatedKey) {
    // Handle duplicate
}
```

---

## Dependency Injection Without Frameworks

Go doesn't need DI frameworks. We use **constructor injection**:

```go
// Service declares its dependencies in the constructor
type ContestService struct {
    contestRepo   domain.ContestRepository
    problemService *ProblemService
    submissionRepo domain.SubmissionRepository
}

func NewContestService(
    contestRepo domain.ContestRepository,
    problemService *ProblemService,
    submissionRepo domain.SubmissionRepository,
) *ContestService {
    return &ContestService{
        contestRepo:   contestRepo,
        problemService: problemService,
        submissionRepo: submissionRepo,
    }
}

// main.go wires everything together
func main() {
    // Create dependencies bottom-up
    db := connectDB()
    
    contestRepo := repository.NewContestRepository(db)
    problemRepo := repository.NewProblemRepository(db)
    submissionRepo := repository.NewSubmissionRepository(db)
    
    problemService := service.NewProblemService(problemRepo)
    contestService := service.NewContestService(contestRepo, problemService, submissionRepo)
    
    // Dependencies are explicit and traceable
}
```

### Benefits

1. **No magic**: You can trace dependencies by reading the code
2. **Compile-time safety**: Missing dependencies = compile error
3. **Easy testing**: Just pass mock implementations

---

## Error Handling Patterns

### Custom Domain Errors

```go
// domain/errors.go
var (
    ErrUserNotFound       = &DomainError{Code: "USER_NOT_FOUND", Message: "User not found"}
    ErrInvalidCredentials = &DomainError{Code: "INVALID_CREDENTIALS", Message: "Invalid email or password"}
    ErrContestNotFound    = &DomainError{Code: "CONTEST_NOT_FOUND", Message: "Contest not found"}
    ErrContestNotActive   = &DomainError{Code: "CONTEST_NOT_ACTIVE", Message: "Contest is not active"}
)

type DomainError struct {
    Code    string
    Message string
}

func (e *DomainError) Error() string {
    return e.Message
}
```

### Mapping Errors to HTTP Status

```go
// handler/error_handler.go
func handleError(c *gin.Context, err error) {
    var domainErr *domain.DomainError
    
    if errors.As(err, &domainErr) {
        switch domainErr.Code {
        case "USER_NOT_FOUND", "CONTEST_NOT_FOUND":
            c.JSON(http.StatusNotFound, gin.H{"error": domainErr.Message})
        case "INVALID_CREDENTIALS":
            c.JSON(http.StatusUnauthorized, gin.H{"error": domainErr.Message})
        case "VALIDATION_ERROR":
            c.JSON(http.StatusBadRequest, gin.H{"error": domainErr.Message})
        default:
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
        }
        return
    }
    
    // Unknown error - log it, return generic message
    log.Error("Unexpected error", zap.Error(err))
    c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
}
```

---

## Concurrency in Practice

### Worker Pool for Problem Selection

```go
// service/problem_service.go
func (s *ProblemService) SelectForContest(ctx context.Context, userID uuid.UUID, count int) ([]domain.Problem, error) {
    difficulties := []string{"Easy", "Medium", "Hard"}
    
    // Create channels
    resultChan := make(chan []domain.Problem, len(difficulties))
    errChan := make(chan error, len(difficulties))
    
    // Fan-out: Launch goroutine per difficulty
    for _, diff := range difficulties {
        go func(difficulty string) {
            problems, err := s.problemRepo.FindByDifficultyExcludingSolved(ctx, difficulty, userID)
            if err != nil {
                errChan <- err
                return
            }
            resultChan <- problems
        }(diff)  // Pass diff as parameter to avoid closure capture bug
    }
    
    // Fan-in: Collect results
    var allProblems []domain.Problem
    for i := 0; i < len(difficulties); i++ {
        select {
        case problems := <-resultChan:
            allProblems = append(allProblems, problems...)
        case err := <-errChan:
            return nil, err
        case <-ctx.Done():
            return nil, ctx.Err()
        }
    }
    
    return selectRandom(allProblems, count), nil
}
```

### Key Concurrency Lessons

1. **Always pass loop variables to goroutines**:
   ```go
   // ❌ Bug: All goroutines see the last value of `i`
   for i := 0; i < 10; i++ {
       go func() { fmt.Println(i) }()
   }
   
   // ✅ Correct: Pass as parameter
   for i := 0; i < 10; i++ {
       go func(n int) { fmt.Println(n) }(i)
   }
   ```

2. **Use buffered channels to prevent goroutine leaks**:
   ```go
   // Buffered channel: won't block senders
   resultChan := make(chan Result, workerCount)
   ```

3. **Always respect context cancellation**:
   ```go
   select {
   case result := <-resultChan:
       // handle result
   case <-ctx.Done():
       return ctx.Err()  // Request was cancelled
   }
   ```

---

## Testing Strategies

### Unit Testing with Mocks

```go
// service/user_service_test.go
type mockUserRepo struct {
    users map[string]*domain.User
}

func (m *mockUserRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
    if user, ok := m.users[email]; ok {
        return user, nil
    }
    return nil, domain.ErrUserNotFound
}

func TestLogin_Success(t *testing.T) {
    // Arrange
    mockRepo := &mockUserRepo{
        users: map[string]*domain.User{
            "test@example.com": {Email: "test@example.com", PasswordHash: hashPassword("password123")},
        },
    }
    service := NewUserService(mockRepo, nil, testJWTConfig)
    
    // Act
    response, err := service.Login(context.Background(), domain.LoginRequest{
        Email:    "test@example.com",
        Password: "password123",
    })
    
    // Assert
    assert.NoError(t, err)
    assert.NotEmpty(t, response.Tokens.AccessToken)
}
```

### Table-Driven Tests

```go
func TestValidatePassword(t *testing.T) {
    tests := []struct {
        name     string
        password string
        wantErr  bool
    }{
        {"valid password", "SecurePass123!", false},
        {"too short", "abc", true},
        {"no uppercase", "securepass123!", true},
        {"no number", "SecurePassword!", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validatePassword(tt.password)
            if (err != nil) != tt.wantErr {
                t.Errorf("validatePassword() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

---

## Common Gotchas & Best Practices

### 1. Nil Pointer Dereference

```go
// ❌ Dangerous
user, _ := repo.FindByID(ctx, id)
fmt.Println(user.Email)  // Panics if user is nil

// ✅ Safe
user, err := repo.FindByID(ctx, id)
if err != nil {
    return err
}
fmt.Println(user.Email)
```

### 2. Goroutine Leaks

```go
// ❌ Leak: Goroutine blocked forever if no one reads
go func() {
    ch <- result  // Blocks if ch is unbuffered and no reader
}()

// ✅ Use buffered channel or select with timeout
go func() {
    select {
    case ch <- result:
    case <-time.After(5 * time.Second):
        log.Warn("Send timed out")
    }
}()
```

### 3. Struct Field Tags

```go
type User struct {
    ID        uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
    Email     string    `gorm:"uniqueIndex;not null" json:"email"`
    Password  string    `gorm:"-" json:"-"`  // gorm:"-" = don't persist, json:"-" = don't serialize
}
```

### 4. Named Returns for Clarity

```go
// Use named returns for complex functions
func ParseConfig(path string) (config *Config, err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("panic while parsing: %v", r)
        }
    }()
    // ...
}
```

---

## Next Steps for Learning

1. **Add a new feature**: Try adding a "leaderboard" endpoint
   - Create `domain/leaderboard.go` with types
   - Add repository interface and implementation
   - Create service with business logic
   - Add HTTP handler

2. **Write tests**: Add unit tests for the user service

3. **Add middleware**: Create a rate-limiting middleware

4. **Explore the code**: Use your IDE to "Go to Definition" and trace the flow

---

## Quick Reference: File Locations

| Concept | Location |
|---------|----------|
| Entry point | `cmd/api/main.go` |
| Domain entities | `internal/domain/*.go` |
| Repository interfaces | `internal/domain/*.go` (bottom of each file) |
| Repository implementations | `internal/repository/*.go` |
| Business logic | `internal/service/*.go` |
| HTTP handlers | `internal/handler/*.go` |
| Middleware | `internal/middleware/*.go` |
| Configuration | `internal/infrastructure/config.go` |
| Database setup | `internal/infrastructure/database.go` |
| Logging | `internal/infrastructure/logger.go` |
| Tracing/Metrics | `internal/infrastructure/telemetry.go` |
