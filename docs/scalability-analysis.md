# Go Scalability Guide: Bottlenecks, Race Conditions & Optimizations

A deep analysis of the Contest Maker 150 backend for billion-scale readiness, covering performance bottlenecks, race conditions, connection pooling, OS optimizations, and Go-specific code smells.

---

## Table of Contents

1. [Critical Bottlenecks for Billion-Scale](#critical-bottlenecks-for-billion-scale)
2. [Race Conditions & Concurrency Issues](#race-conditions--concurrency-issues)
3. [Connection Pooling Deep Dive](#connection-pooling-deep-dive)
4. [Memory & Allocation Optimization](#memory--allocation-optimization)
5. [OS-Level Optimizations](#os-level-optimizations)
6. [Code Smells in Go](#code-smells-in-go)
7. [Bad vs Good Examples from This Project](#bad-vs-good-examples-from-this-project)
8. [HTTP Server Tuning](#http-server-tuning)
9. [Database Query Optimization](#database-query-optimization)
10. [Observability Overhead](#observability-overhead)

---

## Critical Bottlenecks for Billion-Scale

### 1. Database Connection Pool Limits

**Current Code** ([database.go:52-54](file:///c:/Users/rsrsr/Programs/contest-maker-150/backend/internal/infrastructure/database.go#L52-L54)):
```go
sqlDB.SetMaxOpenConns(config.MaxOpenConns)  // Default: 25
sqlDB.SetMaxIdleConns(config.MaxIdleConns)  // Default: 5
sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)
```

**Problem**: At 25 max connections with 1B requests/day (~12k QPS), you'll hit connection exhaustion. Each request waiting for a connection blocks a goroutine.

**Billion-Scale Fix**:
```go
// Scale based on available cores and expected parallelism
cores := runtime.NumCPU()
sqlDB.SetMaxOpenConns(cores * 4)        // Tune based on Postgres max_connections
sqlDB.SetMaxIdleConns(cores * 2)        // Keep warm connections ready
sqlDB.SetConnMaxLifetime(5 * time.Minute)  // Refresh before Postgres timeout
sqlDB.SetConnMaxIdleTime(2 * time.Minute)  // Release idle connections faster

// Add connection pool metrics
go func() {
    ticker := time.NewTicker(10 * time.Second)
    for range ticker.C {
        stats := sqlDB.Stats()
        metrics.DBConnectionsOpen.Set(float64(stats.OpenConnections))
        metrics.DBConnectionsInUse.Set(float64(stats.InUse))
        metrics.DBConnectionsWaiting.Set(float64(stats.WaitCount))
    }
}()
```

---

### 2. N+1 Query Problem in Preloading

**Current Code** ([contest_repository.go:62-74](file:///c:/Users/rsrsr/Programs/contest-maker-150/backend/internal/repository/contest_repository.go#L62-L74)):
```go
func (r *contestRepository) FindByUserID(userID uuid.UUID) ([]domain.Contest, error) {
    result := r.db.
        Preload("ContestProblems", func(db *gorm.DB) *gorm.DB {
            return db.Order("contest_problems.order ASC")
        }).
        Preload("ContestProblems.Problem").  // ❌ N+1: One query per contest!
        Where("user_id = ?", userID).
        Find(&contests)
}
```

**Why It's Bad**: If user has 100 contests, this fires:
- 1 query for contests
- 100 queries for contest_problems
- 100+ queries for problems

**Fix: Use JOINs instead**:
```go
func (r *contestRepository) FindByUserID(userID uuid.UUID) ([]domain.Contest, error) {
    var contests []domain.Contest
    
    // Single query with JOIN
    result := r.db.
        Joins("LEFT JOIN contest_problems ON contest_problems.contest_id = contests.id").
        Joins("LEFT JOIN problems ON problems.id = contest_problems.problem_id").
        Where("contests.user_id = ?", userID).
        Order("contests.created_at DESC, contest_problems.order ASC").
        Find(&contests)
    
    return contests, result.Error
}
```

---

### 3. No Pagination

**Current Code**: `FindAll()`, `FindByUserID()` return unbounded results.

**Problem**: User with 10,000 contests = 10,000 rows loaded into memory.

**Fix**:
```go
type PaginationRequest struct {
    Page     int `form:"page" binding:"min=1"`
    PageSize int `form:"page_size" binding:"min=1,max=100"`
}

func (r *contestRepository) FindByUserIDPaginated(
    userID uuid.UUID, 
    page, pageSize int,
) ([]domain.Contest, int64, error) {
    var contests []domain.Contest
    var total int64
    
    r.db.Model(&domain.Contest{}).Where("user_id = ?", userID).Count(&total)
    
    offset := (page - 1) * pageSize
    result := r.db.
        Where("user_id = ?", userID).
        Order("created_at DESC").
        Offset(offset).
        Limit(pageSize).
        Find(&contests)
    
    return contests, total, result.Error
}
```

---

### 4. Synchronous Problem Selection

**Current Code** ([problem_service.go:130-134](file:///c:/Users/rsrsr/Programs/contest-maker-150/backend/internal/service/problem_service.go#L130-L134)):
```go
for _, diff := range difficulties {
    wg.Add(1)
    go fetchProblems(diff)  // ✅ Good: Parallel fetch
}
```

**What's Good**: Parallel fetching is already implemented.

**What's Missing**: No timeout or context cancellation respect.

**Improved Version**:
```go
func (s *ProblemService) SelectProblemsForContest(ctx context.Context, userID uuid.UUID, count int) ([]domain.Problem, error) {
    ctx, cancel := context.WithTimeout(ctx, 3*time.Second)  // Limit total time
    defer cancel()
    
    g, ctx := errgroup.WithContext(ctx)  // Use errgroup for better error handling
    
    var mu sync.Mutex
    problemsByDifficulty := make(map[domain.Difficulty][]domain.Problem)
    
    for _, diff := range difficulties {
        diff := diff  // Capture loop variable
        g.Go(func() error {
            problems, err := s.problemRepo.FindUnsolvedByUserAndDifficulty(ctx, userID, diff)
            if err != nil {
                return err
            }
            mu.Lock()
            problemsByDifficulty[diff] = problems
            mu.Unlock()
            return nil
        })
    }
    
    if err := g.Wait(); err != nil {
        return nil, fmt.Errorf("failed to fetch problems: %w", err)
    }
    
    // ... rest of selection logic
}
```

---

## Race Conditions & Concurrency Issues

### Issue 1: Shared RNG Without Proper Isolation

**Current Code** ([problem_service.go:24-26](file:///c:/Users/rsrsr/Programs/contest-maker-150/backend/internal/service/problem_service.go#L24-L26)):
```go
type ProblemService struct {
    rng   *rand.Rand
    rngMu sync.Mutex  // ✅ Good: Mutex protects shared RNG
}
```

**What's Good**: The mutex is present.

**Better Pattern**: Use per-request RNG to avoid contention:
```go
func (s *ProblemService) randomSelect(problems []domain.Problem, n int) []domain.Problem {
    // No mutex needed - each request gets its own RNG
    rng := rand.New(rand.NewSource(time.Now().UnixNano()))
    
    shuffled := make([]domain.Problem, len(problems))
    copy(shuffled, problems)
    
    rng.Shuffle(len(shuffled), func(i, j int) {
        shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
    })
    
    return shuffled[:n]
}
```

---

### Issue 2: Race in Active Contest Check

**Current Code** ([contest_service.go:52-70](file:///c:/Users/rsrsr/Programs/contest-maker-150/backend/internal/service/contest_service.go#L52-L70)):
```go
// Check if user already has an active contest
activeContest, err := s.contestRepo.FindActiveByUserID(userID)
if activeContest == nil {
    // Another request could create a contest here! ⚠️
    // ... create new contest
}
```

**Problem**: Two concurrent requests can both pass the check and create duplicate active contests.

**Fix: Use Database-Level Locking**:
```go
func (s *ContestService) CreateContest(ctx context.Context, userID uuid.UUID, req *domain.CreateContestRequest) (*domain.Contest, error) {
    var contest *domain.Contest
    
    err := s.db.Transaction(func(tx *gorm.DB) error {
        // Lock the user row to prevent concurrent contest creation
        var user domain.User
        if err := tx.Set("gorm:query_option", "FOR UPDATE").
            Where("id = ?", userID).First(&user).Error; err != nil {
            return err
        }
        
        // Now safe to check for active contest
        var existing int64
        tx.Model(&domain.Contest{}).
            Where("user_id = ? AND status = ?", userID, domain.ContestStatusActive).
            Count(&existing)
        
        if existing > 0 {
            return domain.ErrActiveContestExists
        }
        
        // Create contest within transaction
        contest = &domain.Contest{...}
        return tx.Create(contest).Error
    })
    
    return contest, err
}
```

---

### Issue 3: Expired Contest Auto-Update Race

**Current Code** ([contest_service.go:59-66](file:///c:/Users/rsrsr/Programs/contest-maker-150/backend/internal/service/contest_service.go#L59-L66)):
```go
if activeContest.IsExpired() {
    now := time.Now()
    activeContest.Status = domain.ContestStatusCompleted
    // Two requests could both update this ⚠️
    if err := s.contestRepo.Update(activeContest); err != nil {
        s.logger.Error("Failed to complete expired contest", zap.Error(err))
    }
}
```

**Problem**: Multiple requests trigger redundant updates.

**Fix: Conditional Update**:
```go
func (s *ContestService) autoCompleteExpiredContest(ctx context.Context, contestID uuid.UUID) error {
    // Atomic conditional update - only updates if still active
    result := s.db.Model(&domain.Contest{}).
        Where("id = ? AND status = ? AND started_at + (duration_minutes * interval '1 minute') < NOW()", 
            contestID, domain.ContestStatusActive).
        Updates(map[string]interface{}{
            "status":   domain.ContestStatusCompleted,
            "ended_at": time.Now(),
        })
    
    if result.RowsAffected == 0 {
        return nil  // Already updated by another request
    }
    return result.Error
}
```

---

## Connection Pooling Deep Dive

### PostgreSQL Connection Pool Tuning

```go
// config.go - Production-ready defaults
Database: DatabaseConfig{
    MaxOpenConns:    50,   // Match Postgres max_connections / num_app_instances
    MaxIdleConns:    10,   // Keep 20% of max as idle
    ConnMaxLifetime: 5 * time.Minute,  // Rotate before Postgres timeout
    ConnMaxIdleTime: 2 * time.Minute,  // Release unused connections
}
```

### Connection Pool Monitoring

```go
func (d *Database) startPoolMetrics(ctx context.Context) {
    go func() {
        ticker := time.NewTicker(15 * time.Second)
        defer ticker.Stop()
        
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                stats := d.sqlDB.Stats()
                
                // Prometheus metrics
                dbPoolOpen.Set(float64(stats.OpenConnections))
                dbPoolInUse.Set(float64(stats.InUse))
                dbPoolIdle.Set(float64(stats.Idle))
                dbPoolWaitCount.Add(float64(stats.WaitCount))
                dbPoolWaitDuration.Observe(stats.WaitDuration.Seconds())
                
                // Alert if pool is exhausted
                utilization := float64(stats.InUse) / float64(stats.MaxOpenConnections)
                if utilization > 0.9 {
                    d.logger.Warn("Database pool near exhaustion",
                        zap.Float64("utilization", utilization),
                        zap.Int64("waiting", stats.WaitCount),
                    )
                }
            }
        }
    }()
}
```

### Connection Pool Exhaustion Symptoms

1. **Slow response times**: Requests waiting for connections
2. **Timeouts**: Context deadline exceeded
3. **High goroutine count**: Blocked goroutines waiting

```go
// Add connection acquire timeout
ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
defer cancel()

result := db.WithContext(ctx).Where("id = ?", id).First(&user)
if errors.Is(result.Error, context.DeadlineExceeded) {
    // Connection pool exhausted - shed load
    return nil, ErrServiceOverloaded
}
```

---

## Memory & Allocation Optimization

### Issue 1: Slice Pre-allocation Missing

**Bad** (from [contest_service.go:91](file:///c:/Users/rsrsr/Programs/contest-maker-150/backend/internal/service/contest_service.go#L91)):
```go
contestProblems := make([]domain.ContestProblem, len(problems))  // ✅ Good - pre-allocated
```

**What's Good**: This is already pre-allocated.

**Common Mistake to Avoid**:
```go
// ❌ BAD: Grows dynamically, causes multiple allocations
var results []Problem
for _, p := range problems {
    results = append(results, p)  // Re-allocates on growth
}

// ✅ GOOD: Pre-allocate when size is known
results := make([]Problem, 0, len(problems))
for _, p := range problems {
    results = append(results, p)  // No re-allocation
}
```

---

### Issue 2: Large JSON Payloads

**Problem**: Serializing large responses allocates significant memory.

**Fix: Stream JSON for large responses**:
```go
func (h *ContestHandler) GetContests(c *gin.Context) {
    contests, err := h.service.GetContests(ctx, userID)
    if err != nil {
        handleError(c, err)
        return
    }
    
    // For small responses, normal JSON is fine
    if len(contests) < 100 {
        c.JSON(http.StatusOK, gin.H{"contests": contests})
        return
    }
    
    // For large responses, stream JSON
    c.Header("Content-Type", "application/json")
    c.Writer.WriteHeader(http.StatusOK)
    
    enc := json.NewEncoder(c.Writer)
    enc.Encode(map[string]interface{}{
        "contests": contests,
    })
}
```

---

### Issue 3: String Concatenation in Hot Paths

**Bad Pattern** (hypothetical):
```go
// ❌ BAD: Creates new string for each concatenation
dsn := "host=" + host + " port=" + port + " user=" + user
```

**Good Pattern** (from [config.go:122-129](file:///c:/Users/rsrsr/Programs/contest-maker-150/backend/internal/infrastructure/config.go#L122-L129)):
```go
// ✅ GOOD: Use strings.Builder for multiple concatenations
func (c *DatabaseConfig) DSN() string {
    var b strings.Builder
    b.Grow(100)  // Pre-allocate estimated size
    b.WriteString("host=")
    b.WriteString(c.Host)
    b.WriteString(" port=")
    b.WriteString(strconv.Itoa(c.Port))
    // ...
    return b.String()
}
```

---

## OS-Level Optimizations

### 1. File Descriptor Limits

```bash
# Check current limits
ulimit -n

# Set for production (in /etc/security/limits.conf)
* soft nofile 65535
* hard nofile 65535
```

### 2. TCP Tuning for High Connections

```bash
# /etc/sysctl.conf for Linux servers

# Allow more connections
net.core.somaxconn = 65535
net.ipv4.tcp_max_syn_backlog = 65535

# Faster connection reuse
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_fin_timeout = 15

# More local ports for outbound connections
net.ipv4.ip_local_port_range = 1024 65535
```

### 3. GOGC Tuning

```go
// For memory-constrained environments
import "runtime/debug"

func init() {
    // Default is 100 (GC when heap doubles)
    // For high-throughput, low-latency, reduce GC frequency
    debug.SetGCPercent(200)  // GC when heap triples
    
    // Or set via environment
    // GOGC=200 ./server
}
```

### 4. GOMAXPROCS

```go
import "runtime"

func main() {
    // Usually auto-detected, but verify for containers
    fmt.Printf("GOMAXPROCS: %d\n", runtime.GOMAXPROCS(0))
    
    // In Docker, may need explicit setting
    // Read from cgroup limits in Kubernetes
}
```

---

## Code Smells in Go

### 1. Ignoring Errors

**Bad**:
```go
_ = file.Close()  // ❌ Error ignored
json.Unmarshal(data, &obj)  // ❌ Error not checked
```

**Good**:
```go
if err := file.Close(); err != nil {
    log.Error("failed to close file", zap.Error(err))
}
```

---

### 2. Naked Returns in Complex Functions

**Bad**:
```go
func calculate(x int) (result int, err error) {
    if x < 0 {
        err = errors.New("negative")
        return  // ❌ What does this return?
    }
    result = x * 2
    return  // ❌ Confusing
}
```

**Good**:
```go
func calculate(x int) (int, error) {
    if x < 0 {
        return 0, errors.New("negative")  // ✅ Explicit
    }
    return x * 2, nil  // ✅ Clear
}
```

---

### 3. Interface Pollution

**Bad**:
```go
// ❌ Huge interface - hard to mock, hard to understand
type UserRepository interface {
    Create(...)
    Update(...)
    Delete(...)
    FindByID(...)
    FindByEmail(...)
    FindByUsername(...)
    FindAll(...)
    FindActive(...)
    Search(...)
    Count(...)
}
```

**Good**:
```go
// ✅ Small, focused interfaces
type UserReader interface {
    FindByID(ctx context.Context, id uuid.UUID) (*User, error)
    FindByEmail(ctx context.Context, email string) (*User, error)
}

type UserWriter interface {
    Create(ctx context.Context, user *User) error
    Update(ctx context.Context, user *User) error
}

// Compose when needed
type UserRepository interface {
    UserReader
    UserWriter
}
```

---

### 4. Context as Struct Field

**Bad**:
```go
type Service struct {
    ctx context.Context  // ❌ Context stored in struct
}
```

**Good**:
```go
// ✅ Context passed as first parameter
func (s *Service) DoWork(ctx context.Context, ...) error {
    // Use ctx directly
}
```

---

### 5. Mutex Copying

**Bad**:
```go
type Counter struct {
    mu    sync.Mutex
    count int
}

func copyCounter(c Counter) Counter {  // ❌ Copies mutex!
    return c
}
```

**Good**:
```go
type Counter struct {
    mu    sync.Mutex
    count int
}

func copyCounter(c *Counter) *Counter {  // ✅ Pass by pointer
    return c
}
```

---

## Bad vs Good Examples from This Project

### Example 1: Context Propagation

**✅ GOOD** (from [contest_service.go:42-44](file:///c:/Users/rsrsr/Programs/contest-maker-150/backend/internal/service/contest_service.go#L42-L44)):
```go
func (s *ContestService) CreateContest(ctx context.Context, userID uuid.UUID, req *domain.CreateContestRequest) (*domain.Contest, error) {
    ctx, span := s.tracer.Start(ctx, "ContestService.CreateContest")
    defer span.End()
    // Context properly propagated to child operations
}
```

**⚠️ MISSING** (repository methods don't accept context):
```go
// Current - context not passed to DB
func (r *contestRepository) FindByID(id uuid.UUID) (*domain.Contest, error) {
    result := r.db.Where("id = ?", id).First(&contest)  // ❌ No context
}

// Should be:
func (r *contestRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Contest, error) {
    result := r.db.WithContext(ctx).Where("id = ?", id).First(&contest)  // ✅
}
```

---

### Example 2: Prepared Statements

**✅ GOOD** (from [database.go:39](file:///c:/Users/rsrsr/Programs/contest-maker-150/backend/internal/infrastructure/database.go#L39)):
```go
db, err := gorm.Open(postgres.Open(config.DSN()), &gorm.Config{
    PrepareStmt: true,  // ✅ Cache prepared statements
})
```

---

### Example 3: Goroutine Worker Pool

**✅ GOOD** (from [problem_service.go:116-140](file:///c:/Users/rsrsr/Programs/contest-maker-150/backend/internal/service/problem_service.go#L116-L140)):
```go
resultChan := make(chan difficultyResult, len(difficulties))  // ✅ Buffered channel
var wg sync.WaitGroup

for _, diff := range difficulties {
    wg.Add(1)
    go fetchProblems(diff)
}

go func() {
    wg.Wait()
    close(resultChan)  // ✅ Proper channel cleanup
}()
```

**⚠️ IMPROVEMENT NEEDED**: Add timeout handling:
```go
select {
case result := <-resultChan:
    // process
case <-ctx.Done():
    return nil, ctx.Err()  // ✅ Respect cancellation
case <-time.After(5 * time.Second):
    return nil, ErrTimeout  // ✅ Prevent hanging
}
```

---

### Example 4: Error Wrapping

**⚠️ INCONSISTENT** (from [database.go:42](file:///c:/Users/rsrsr/Programs/contest-maker-150/backend/internal/infrastructure/database.go#L42)):
```go
return nil, fmt.Errorf("failed to connect to database: %w", err)  // ✅ Good
```

But in some places:
```go
return result.Error  // ❌ Raw error, no context
```

**Better**:
```go
if result.Error != nil {
    return fmt.Errorf("contestRepository.FindByID(%s): %w", id, result.Error)
}
```

---

### Example 5: Graceful Shutdown

**✅ GOOD** (from [main.go:186-211](file:///c:/Users/rsrsr/Programs/contest-maker-150/backend/cmd/api/main.go#L186-L211)):
```go
// Start server in goroutine
go func() {
    if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        logger.Fatal("Failed to start server", zap.Error(err))
    }
}()

// Wait for interrupt signal
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit

// Graceful shutdown with timeout
shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
defer shutdownCancel()
server.Shutdown(shutdownCtx)  // ✅ Drain connections gracefully
```

---

## HTTP Server Tuning

### Current Configuration

```go
server := &http.Server{
    Addr:         fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port),
    Handler:      router,
    ReadTimeout:  config.Server.ReadTimeout,   // 10s
    WriteTimeout: config.Server.WriteTimeout,  // 30s
}
```

### Production-Grade Configuration

```go
server := &http.Server{
    Addr:              addr,
    Handler:           router,
    ReadTimeout:       10 * time.Second,
    ReadHeaderTimeout: 5 * time.Second,   // NEW: Prevent slowloris attacks
    WriteTimeout:      30 * time.Second,
    IdleTimeout:       120 * time.Second, // NEW: Keep-alive timeout
    MaxHeaderBytes:    1 << 20,           // NEW: 1MB max header size
}
```

### HTTP/2 Support

```go
import "golang.org/x/net/http2"

func configureHTTP2(server *http.Server) {
    http2.ConfigureServer(server, &http2.Server{
        MaxConcurrentStreams: 250,
        IdleTimeout:          30 * time.Second,
    })
}
```

---

## Database Query Optimization

### Add Database Indexes

```go
// In migrations or AutoMigrate
type Contest struct {
    ID        uuid.UUID `gorm:"primaryKey"`
    UserID    uuid.UUID `gorm:"index:idx_contests_user_status,priority:1"` // Composite index
    Status    string    `gorm:"index:idx_contests_user_status,priority:2"`
    StartedAt time.Time `gorm:"index"`
}
```

### Query Analysis

```sql
-- Enable slow query logging in PostgreSQL
ALTER DATABASE contestmaker SET log_min_duration_statement = 100;  -- Log queries > 100ms

-- Analyze query plans
EXPLAIN ANALYZE 
SELECT * FROM contests 
WHERE user_id = '...' AND status = 'active';
```

---

## Observability Overhead

### Issue: Tracing Every Request

**Current**: Every handler creates spans.

**Problem at Scale**: 1B requests = 1B spans = massive storage/cost.

**Solution: Sampling**:
```go
import "go.opentelemetry.io/otel/sdk/trace"

tp := trace.NewTracerProvider(
    trace.WithSampler(trace.ParentBased(
        trace.TraceIDRatioBased(0.01),  // Sample 1% of traces
    )),
    // ...
)
```

### Issue: Logging Every Request

**Current** (from [middleware/logging.go](file:///c:/Users/rsrsr/Programs/contest-maker-150/backend/internal/middleware/logging.go)):
```go
// Logs every request in detail
logger.Info("Request completed", ...)
```

**Fix for Scale**:
```go
// Only log errors and slow requests in production
if c.Writer.Status() >= 400 || latency > 500*time.Millisecond {
    logger.Info("Request", 
        zap.Int("status", c.Writer.Status()),
        zap.Duration("latency", latency),
    )
}
```

---

## Summary: Priority Fixes for Billion-Scale

| Priority | Issue | Impact | Fix Complexity |
|----------|-------|--------|----------------|
| 🔴 Critical | Race in contest creation | Data corruption | Medium |
| 🔴 Critical | N+1 queries | DB overload | Medium |
| 🟠 High | No pagination | Memory exhaustion | Low |
| 🟠 High | Missing context in repos | No cancellation | Low |
| 🟡 Medium | Connection pool sizing | Timeouts | Low |
| 🟡 Medium | Trace sampling | Cost/performance | Low |
| 🟢 Low | String allocations | Minor GC pressure | Low |

---

## Quick Reference: Commands

```bash
# Profile memory allocations
go test -bench=. -benchmem ./...

# Profile CPU
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30

# Check for data races
go test -race ./...

# Detect potential issues
go vet ./...
staticcheck ./...
golangci-lint run

# Memory analysis
go tool pprof http://localhost:8080/debug/pprof/heap

# Goroutine dump (find leaks)
curl http://localhost:8080/debug/pprof/goroutine?debug=2
```

---

## Security Analysis & Hardening

This section identifies security vulnerabilities in the codebase, where Go's patterns can lead to security issues, and how to write secure Go code.

---

## Attack Surface Analysis

### 1. Authentication & Session Management

#### Vulnerability: JWT Secret in Code/Config

**Current Code** ([config.go:78](file:///c:/Users/rsrsr/Programs/contest-maker-150/backend/internal/infrastructure/config.go#L78)):
```go
SecretKey: getEnv("JWT_SECRET", "your-super-secret-key-change-in-production")
```

**Problem**: Default secret exposed in source code. If `.env` is missing, the app runs with a known secret.

**Fix**:
```go
func LoadConfig() *Config {
    jwtSecret := os.Getenv("JWT_SECRET")
    if jwtSecret == "" {
        log.Fatal("JWT_SECRET environment variable is required")
    }
    if len(jwtSecret) < 32 {
        log.Fatal("JWT_SECRET must be at least 32 characters")
    }
    // ...
}
```

---

#### Vulnerability: No JWT Token Blacklist/Revocation

**Current Code** ([user_service.go:139-174](file:///c:/Users/rsrsr/Programs/contest-maker-150/backend/internal/service/user_service.go#L139-L174)):
```go
func (s *UserService) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
    // Just validates signature and expiry
    claims, err := s.validateToken(refreshToken)
    // ... generates new tokens
}
```

**Problem**: If a refresh token is compromised, it can't be revoked until expiry.

**Fix: Implement Token Revocation**:
```go
// Store refresh tokens in database or Redis
type RefreshToken struct {
    ID        uuid.UUID `gorm:"primaryKey"`
    UserID    uuid.UUID `gorm:"index"`
    TokenHash string    `gorm:"uniqueIndex"`  // Store hash, not token
    ExpiresAt time.Time
    Revoked   bool
    CreatedAt time.Time
}

func (s *UserService) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
    tokenHash := sha256Hash(refreshToken)
    
    // Check if token exists and is not revoked
    var stored RefreshToken
    if err := s.db.Where("token_hash = ? AND revoked = false AND expires_at > NOW()", 
        tokenHash).First(&stored).Error; err != nil {
        return nil, domain.ErrInvalidToken
    }
    
    // Revoke old token (rotation)
    s.db.Model(&stored).Update("revoked", true)
    
    // Generate new tokens
    return s.generateTokenPair(user)
}

// Logout revokes all user tokens
func (s *UserService) Logout(ctx context.Context, userID uuid.UUID) error {
    return s.db.Model(&RefreshToken{}).
        Where("user_id = ?", userID).
        Update("revoked", true).Error
}
```

---

#### Vulnerability: Algorithm Confusion Attack

**Current Code** ([user_service.go:311-316](file:///c:/Users/rsrsr/Programs/contest-maker-150/backend/internal/service/user_service.go#L311-L316)):
```go
token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
    if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
        return nil, domain.ErrInvalidToken  // ✅ Good: Validates algorithm
    }
    return []byte(s.jwtConfig.SecretKey), nil
})
```

**What's Good**: Algorithm validation is already implemented.

**Additional Hardening**:
```go
// Be more explicit about allowed algorithms
if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
    return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
}
```

---

### 2. SQL Injection Prevention

#### ✅ Current Implementation is Safe

**Good Pattern** (from [contest_repository.go:31](file:///c:/Users/rsrsr/Programs/contest-maker-150/backend/internal/repository/contest_repository.go#L31)):
```go
result := r.db.Where("id = ?", id).First(&contest)  // ✅ Parameterized
```

**Why It's Safe**: GORM uses prepared statements with placeholders.

**Dangerous Pattern to NEVER Use**:
```go
// ❌ NEVER DO THIS - SQL INJECTION
query := fmt.Sprintf("SELECT * FROM users WHERE email = '%s'", email)
db.Raw(query).Scan(&user)

// ❌ String concatenation
db.Where("email = '" + email + "'").First(&user)
```

**Safe Raw Query When Needed**:
```go
// ✅ Safe - parameterized raw query
db.Raw("SELECT * FROM users WHERE email = ? AND active = ?", email, true).Scan(&user)
```

---

### 3. Rate Limiting (MISSING)

**Current State**: No rate limiting implemented.

**Attack Vector**: Brute force attacks on login, resource exhaustion.

**Fix: Add Rate Limiting Middleware**:
```go
package middleware

import (
    "net/http"
    "sync"
    "time"
    
    "github.com/gin-gonic/gin"
    "golang.org/x/time/rate"
)

// IPRateLimiter stores rate limiters per IP
type IPRateLimiter struct {
    ips    map[string]*rate.Limiter
    mu     sync.RWMutex
    rate   rate.Limit
    burst  int
    cleanup time.Duration
}

func NewIPRateLimiter(r rate.Limit, burst int) *IPRateLimiter {
    limiter := &IPRateLimiter{
        ips:     make(map[string]*rate.Limiter),
        rate:    r,
        burst:   burst,
        cleanup: 10 * time.Minute,
    }
    go limiter.cleanupLoop()
    return limiter
}

func (l *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
    l.mu.Lock()
    defer l.mu.Unlock()
    
    if limiter, exists := l.ips[ip]; exists {
        return limiter
    }
    
    limiter := rate.NewLimiter(l.rate, l.burst)
    l.ips[ip] = limiter
    return limiter
}

func RateLimitMiddleware(limiter *IPRateLimiter) gin.HandlerFunc {
    return func(c *gin.Context) {
        ip := c.ClientIP()
        
        if !limiter.GetLimiter(ip).Allow() {
            c.JSON(http.StatusTooManyRequests, gin.H{
                "error": "Rate limit exceeded. Try again later.",
            })
            c.Abort()
            return
        }
        
        c.Next()
    }
}

// Usage in main.go:
// General: 100 requests/second with burst of 200
generalLimiter := NewIPRateLimiter(100, 200)
router.Use(RateLimitMiddleware(generalLimiter))

// Stricter for auth endpoints: 5 requests/second
authLimiter := NewIPRateLimiter(5, 10)
auth.Use(RateLimitMiddleware(authLimiter))
```

---

### 4. Password Security

#### ✅ Good: Using bcrypt

**Current Code** ([user_service.go:69](file:///c:/Users/rsrsr/Programs/contest-maker-150/backend/internal/service/user_service.go#L69)):
```go
hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
```

**What's Good**: Using bcrypt with default cost (10).

**Improvement: Adaptive Cost**:
```go
// Increase cost as hardware improves
const bcryptCost = 12  // Takes ~250ms on modern hardware

// For high-security applications
func hashPassword(password string) (string, error) {
    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
    if err != nil {
        return "", err
    }
    return string(hash), nil
}
```

#### Missing: Password Policy Validation

```go
// Add password validation
func validatePassword(password string) error {
    if len(password) < 8 {
        return errors.New("password must be at least 8 characters")
    }
    
    var hasUpper, hasLower, hasDigit, hasSpecial bool
    for _, char := range password {
        switch {
        case unicode.IsUpper(char):
            hasUpper = true
        case unicode.IsLower(char):
            hasLower = true
        case unicode.IsDigit(char):
            hasDigit = true
        case unicode.IsPunct(char) || unicode.IsSymbol(char):
            hasSpecial = true
        }
    }
    
    if !hasUpper || !hasLower || !hasDigit {
        return errors.New("password must contain uppercase, lowercase, and digit")
    }
    
    return nil
}
```

---

### 5. Information Disclosure

#### Vulnerability: Detailed Error Messages

**Current Code** ([auth_handler.go:41-44](file:///c:/Users/rsrsr/Programs/contest-maker-150/backend/internal/handler/auth_handler.go#L41-L44)):
```go
c.JSON(http.StatusBadRequest, gin.H{
    "error":   "Invalid request body",
    "details": err.Error(),  // ⚠️ Leaks internal details
})
```

**Problem**: `err.Error()` may contain internal information (field names, types).

**Fix**:
```go
func (h *AuthHandler) Register(c *gin.Context) {
    var req domain.UserCreateRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        // Log full error for debugging
        h.logger.Debug("Invalid request", zap.Error(err))
        
        // Return sanitized message to client
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Invalid request format",
        })
        return
    }
}
```

#### Vulnerability: User Enumeration

**Current Code**: Different responses for "user not found" vs "wrong password".

**Attack**: Attacker can determine valid email addresses.

**Fix: Consistent Error Messages**:
```go
func (h *AuthHandler) Login(c *gin.Context) {
    // ...
    user, tokens, err := h.userService.Login(ctx, req.Email, req.Password)
    if err != nil {
        // Same message for all auth failures
        c.JSON(http.StatusUnauthorized, gin.H{
            "error": "Invalid email or password",
        })
        return
    }
}
```

---

### 6. CORS Security

#### Current Code ([cors.go:20-46](file:///c:/Users/rsrsr/Programs/contest-maker-150/backend/internal/middleware/cors.go#L20-L46)):
```go
func DefaultCORSConfig() CORSConfig {
    return CORSConfig{
        AllowOrigins: []string{
            "http://localhost:3000",
            "http://localhost:5173",
        },
        AllowCredentials: true,  // ⚠️ Be careful with this
    }
}
```

**What's Good**: Not using `*` for origins.

**Production Hardening**:
```go
func ProductionCORSConfig() CORSConfig {
    allowedOrigins := strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")
    if len(allowedOrigins) == 0 {
        log.Fatal("ALLOWED_ORIGINS environment variable required")
    }
    
    // Validate origins are proper URLs
    for _, origin := range allowedOrigins {
        if _, err := url.Parse(origin); err != nil {
            log.Fatalf("Invalid origin URL: %s", origin)
        }
    }
    
    return CORSConfig{
        AllowOrigins:     allowedOrigins,
        AllowCredentials: true,
        // ...
    }
}
```

---

### 7. Request Body Limits (MISSING)

**Attack Vector**: OOM through massive request bodies.

**Fix**:
```go
func BodyLimitMiddleware(maxBytes int64) gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
        c.Next()
    }
}

// Usage: 1MB limit
router.Use(BodyLimitMiddleware(1 << 20))
```

---

### 8. Timing Attacks in Authentication

**Current Code** ([user_service.go:119](file:///c:/Users/rsrsr/Programs/contest-maker-150/backend/internal/service/user_service.go#L119)):
```go
if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
    return nil, nil, domain.ErrInvalidCredentials
}
```

**What's Good**: bcrypt.CompareHashAndPassword is constant-time.

**But**: Early return on "user not found" leaks info.

**Fix: Constant-time comparison for non-existent users**:
```go
func (s *UserService) Login(ctx context.Context, email, password string) (*domain.User, *TokenPair, error) {
    user, err := s.userRepo.FindByEmail(email)
    if err != nil {
        if err == domain.ErrUserNotFound {
            // Perform a dummy bcrypt comparison to prevent timing attacks
            bcrypt.CompareHashAndPassword(
                []byte("$2a$10$dummyhashvaluetopreventtimingattacks"),
                []byte(password),
            )
            return nil, nil, domain.ErrInvalidCredentials
        }
        return nil, nil, err
    }
    
    if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
        return nil, nil, domain.ErrInvalidCredentials
    }
    // ...
}
```

---

### 9. Secrets in Logs

**Problem**: Accidentally logging sensitive data.

**Current Risk** ([user_service.go:56](file:///c:/Users/rsrsr/Programs/contest-maker-150/backend/internal/service/user_service.go#L56)):
```go
span.SetAttributes(attribute.String("user.email", req.Email))  // ⚠️ PII in traces
```

**Fix: Sanitize Logs**:
```go
// Never log full emails
func sanitizeEmail(email string) string {
    parts := strings.Split(email, "@")
    if len(parts) != 2 {
        return "***"
    }
    if len(parts[0]) <= 2 {
        return "**@" + parts[1]
    }
    return parts[0][:2] + "***@" + parts[1]
}

// Log only necessary info
span.SetAttributes(
    attribute.String("user.email_domain", getDomain(req.Email)),
)
```

---

### 10. Insecure Direct Object Reference (IDOR)

**Current Code** ([contest_service.go:196-204](file:///c:/Users/rsrsr/Programs/contest-maker-150/backend/internal/service/contest_service.go#L196-L204)):
```go
// Get the contest
contest, err := s.contestRepo.FindByID(contestID)
if err != nil {
    return err
}

// Verify ownership ✅ Good!
if contest.UserID != userID {
    return domain.ErrForbidden
}
```

**What's Good**: Ownership check is present.

**Best Practice: Always verify ownership**:
```go
// Helper function to ensure consistent checks
func (s *ContestService) getContestWithOwnershipCheck(ctx context.Context, contestID, userID uuid.UUID) (*domain.Contest, error) {
    contest, err := s.contestRepo.FindByID(contestID)
    if err != nil {
        return nil, err
    }
    
    if contest.UserID != userID {
        // Log potential attack attempt
        s.logger.Warn("IDOR attempt detected",
            zap.String("contest_id", contestID.String()),
            zap.String("user_id", userID.String()),
            zap.String("owner_id", contest.UserID.String()),
        )
        return nil, domain.ErrForbidden
    }
    
    return contest, nil
}
```

---

## Security Headers (MISSING)

Add security headers middleware:

```go
func SecurityHeadersMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Prevent MIME type sniffing
        c.Header("X-Content-Type-Options", "nosniff")
        
        // XSS protection (legacy but still useful)
        c.Header("X-XSS-Protection", "1; mode=block")
        
        // Prevent clickjacking
        c.Header("X-Frame-Options", "DENY")
        
        // Control referrer information
        c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
        
        // Content Security Policy (for API, minimal)
        c.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
        
        // HSTS (only enable if you're sure about HTTPS)
        // c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        
        c.Next()
    }
}
```

---

## Secure Coding Patterns in Go

### Pattern 1: Never Log Secrets

```go
// ❌ BAD
log.Printf("Connecting with password: %s", password)
log.Printf("JWT: %s", token)

// ✅ GOOD
log.Printf("Connecting to database")
log.Printf("Token validated for user: %s", userID)
```

### Pattern 2: Clear Sensitive Data from Memory

```go
// ❌ BAD - password stays in memory
func login(password string) {
    // password string can't be cleared
}

// ✅ BETTER - use byte slice and clear it
func login(password []byte) {
    defer func() {
        for i := range password {
            password[i] = 0
        }
    }()
    // use password...
}
```

### Pattern 3: Secure Random Generation

```go
import "crypto/rand"

// ❌ BAD - predictable
token := fmt.Sprintf("%d", time.Now().UnixNano())

// ✅ GOOD - cryptographically secure
func generateSecureToken(length int) (string, error) {
    bytes := make([]byte, length)
    if _, err := rand.Read(bytes); err != nil {
        return "", err
    }
    return base64.URLEncoding.EncodeToString(bytes), nil
}
```

### Pattern 4: Constant-Time Comparison

```go
import "crypto/subtle"

// ❌ BAD - timing attack vulnerable
if apiKey == storedKey {
    // ...
}

// ✅ GOOD - constant-time
if subtle.ConstantTimeCompare([]byte(apiKey), []byte(storedKey)) == 1 {
    // ...
}
```

### Pattern 5: Input Validation

```go
// Always validate and sanitize input
func validateContestRequest(req *CreateContestRequest) error {
    if req.ProblemCount < 1 || req.ProblemCount > 150 {
        return errors.New("problem count must be between 1 and 150")
    }
    if req.DurationMinutes < 5 || req.DurationMinutes > 300 {
        return errors.New("duration must be between 5 and 300 minutes")
    }
    return nil
}
```

---

## Security Priority Fixes

| Priority | Vulnerability | Risk Level | Fix Complexity |
|----------|--------------|------------|----------------|
| 🔴 Critical | No rate limiting | Brute force | Medium |
| 🔴 Critical | JWT secret in defaults | Token forgery | Low |
| 🔴 Critical | No token revocation | Session hijacking | Medium |
| 🟠 High | No security headers | Various | Low |
| 🟠 High | No request size limits | DoS | Low |
| 🟠 High | Timing attack on login | User enumeration | Low |
| 🟡 Medium | Error message disclosure | Info leak | Low |
| 🟡 Medium | PII in traces | Privacy | Low |
| 🟢 Low | Password policy | Weak passwords | Low |

---

## Security Scanning Tools

```bash
# Static analysis for security vulnerabilities
go install github.com/securego/gosec/v2/cmd/gosec@latest
gosec ./...

# Dependency vulnerability scanner
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...

# Secret scanning
trufflehog git file://. --only-verified

# OWASP dependency check
# docker run --rm -v $(pwd):/src owasp/dependency-check --scan /src
```

---

## Security Checklist

- [ ] JWT secrets loaded from environment with validation
- [ ] Rate limiting on all endpoints (stricter on auth)
- [ ] Token revocation/blacklist implemented
- [ ] Security headers middleware added
- [ ] Request body size limits
- [ ] Password strength validation
- [ ] No sensitive data in logs/traces
- [ ] All user input validated
- [ ] IDOR checks on all resource access
- [ ] CORS configured for production origins
- [ ] HTTPS enforced (via reverse proxy)
- [ ] Regular dependency vulnerability scans

