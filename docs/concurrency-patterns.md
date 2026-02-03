# Concurrency Patterns in Contest Maker 150

This document details the Go concurrency patterns used in the Contest Maker 150 backend, explaining the rationale, implementation, and best practices.

## Overview

The backend leverages Go's concurrency primitives to achieve high performance:

- **Goroutines**: Lightweight threads for concurrent execution
- **Channels**: Type-safe communication between goroutines
- **Context**: Request-scoped cancellation and timeouts
- **sync Package**: Mutexes, WaitGroups, and other synchronization

## Pattern 1: Fan-Out/Fan-In for User Progress

**Location**: `internal/service/user_service.go`

**Problem**: Calculating user progress requires multiple independent database queries (count by Easy, Medium, Hard). Sequential execution is slow.

**Solution**: Fan-out to parallel goroutines, fan-in results.

```go
func (s *UserService) GetUserProgress(ctx context.Context, userID uuid.UUID) (*domain.UserProgress, error) {
    ctx, span := s.tracer.Start(ctx, "UserService.GetUserProgress")
    defer span.End()

    difficulties := []domain.Difficulty{
        domain.DifficultyEasy,
        domain.DifficultyMedium,
        domain.DifficultyHard,
    }

    type result struct {
        difficulty domain.Difficulty
        count      int64
        err        error
    }

    // Create buffered channel for results
    results := make(chan result, len(difficulties))

    // Fan-out: Launch concurrent queries
    for _, diff := range difficulties {
        go func(d domain.Difficulty) {
            count, err := s.submissionRepo.CountByUserAndDifficulty(ctx, userID, d)
            results <- result{difficulty: d, count: count, err: err}
        }(diff)
    }

    // Fan-in: Collect results
    progress := &domain.UserProgress{}
    for i := 0; i < len(difficulties); i++ {
        r := <-results
        if r.err != nil {
            return nil, r.err
        }
        switch r.difficulty {
        case domain.DifficultyEasy:
            progress.EasySolved = int(r.count)
        case domain.DifficultyMedium:
            progress.MediumSolved = int(r.count)
        case domain.DifficultyHard:
            progress.HardSolved = int(r.count)
        }
    }

    progress.TotalSolved = progress.EasySolved + progress.MediumSolved + progress.HardSolved
    return progress, nil
}
```

**Key Points:**

1. **Buffered Channel**: Size equals number of goroutines to prevent blocking
2. **Closure Variable Capture**: `diff` is passed as parameter to avoid data races
3. **Error Handling**: Any error fails the entire operation
4. **Context Propagation**: Same context passed to all queries for cancellation

**Performance**: 3 sequential queries → 1 parallel batch (latency of slowest query)

## Pattern 2: Worker Pool for Problem Selection

**Location**: `internal/service/problem_service.go`

**Problem**: Selecting problems for a contest requires querying unsolved problems by difficulty. We need controlled parallelism to avoid overwhelming the database.

**Solution**: Worker pool with fixed concurrency.

```go
func (s *ProblemService) SelectProblemsForContest(
    ctx context.Context,
    userID uuid.UUID,
    count int,
) ([]domain.Problem, error) {
    ctx, span := s.tracer.Start(ctx, "ProblemService.SelectProblemsForContest")
    defer span.End()

    distribution := s.calculateDistribution(count)

    type workItem struct {
        difficulty domain.Difficulty
        needed     int
    }

    type workResult struct {
        difficulty domain.Difficulty
        problems   []domain.Problem
        err        error
    }

    jobs := make(chan workItem, 3)
    results := make(chan workResult, 3)

    // Start workers (bounded concurrency)
    const numWorkers = 3
    var wg sync.WaitGroup
    for w := 0; w < numWorkers; w++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for job := range jobs {
                problems, err := s.problemRepo.FindUnsolvedByUserAndDifficulty(
                    ctx, userID, job.difficulty, job.needed,
                )
                results <- workResult{
                    difficulty: job.difficulty,
                    problems:   problems,
                    err:        err,
                }
            }
        }()
    }

    // Send jobs
    go func() {
        if distribution.easy > 0 {
            jobs <- workItem{domain.DifficultyEasy, distribution.easy}
        }
        if distribution.medium > 0 {
            jobs <- workItem{domain.DifficultyMedium, distribution.medium}
        }
        if distribution.hard > 0 {
            jobs <- workItem{domain.DifficultyHard, distribution.hard}
        }
        close(jobs)
    }()

    // Wait for workers and close results
    go func() {
        wg.Wait()
        close(results)
    }()

    // Collect results
    var selected []domain.Problem
    for r := range results {
        if r.err != nil {
            return nil, r.err
        }
        selected = append(selected, r.problems...)
    }

    // Sort by difficulty for gradual progression
    sort.Slice(selected, func(i, j int) bool {
        return selected[i].DifficultyWeight() < selected[j].DifficultyWeight()
    })

    return selected, nil
}
```

**Key Points:**

1. **Bounded Concurrency**: `numWorkers = 3` limits database connections
2. **Channel Closing**: Jobs channel closed after sending all work
3. **WaitGroup**: Ensures all workers complete before closing results
4. **Goroutine for Sending**: Prevents deadlock when sending jobs

## Pattern 3: Thread-Safe Random Selection

**Location**: `internal/service/problem_service.go`

**Problem**: `math/rand.Rand` is not safe for concurrent use. Multiple goroutines selecting random problems could cause data races.

**Solution**: Per-operation random source with unique seed.

```go
import (
    "math/rand"
    "time"
)

// randomSelect performs Fisher-Yates shuffle and returns first n elements
func (s *ProblemService) randomSelect(problems []domain.Problem, n int) []domain.Problem {
    if n >= len(problems) {
        return problems
    }

    // Thread-safe: create new random source per call
    rng := rand.New(rand.NewSource(time.Now().UnixNano()))

    // Fisher-Yates shuffle
    for i := len(problems) - 1; i > 0; i-- {
        j := rng.Intn(i + 1)
        problems[i], problems[j] = problems[j], problems[i]
    }

    return problems[:n]
}
```

**Key Points:**

1. **New Source Per Call**: Each invocation gets unique random sequence
2. **UnixNano Seed**: High-resolution timestamp for uniqueness
3. **Fisher-Yates**: O(n) unbiased shuffle algorithm

## Pattern 4: Context Cancellation

**Location**: Throughout all layers

**Problem**: Long-running operations should stop when the request is cancelled (client disconnect, timeout).

**Solution**: Context propagation with cancellation checks.

```go
func (s *ContestService) CreateContest(
    ctx context.Context,
    userID uuid.UUID,
    req *domain.CreateContestRequest,
) (*domain.Contest, error) {
    ctx, span := s.tracer.Start(ctx, "ContestService.CreateContest")
    defer span.End()

    // Check if context is already cancelled
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    // Check for existing active contest
    existingContest, err := s.contestRepo.FindActiveByUserID(ctx, userID)
    if err != nil {
        return nil, err
    }
    if existingContest != nil {
        // Handle expired contest
        if existingContest.IsExpired() {
            if err := s.autoCompleteContest(ctx, existingContest); err != nil {
                return nil, err
            }
        } else {
            return nil, domain.ErrActiveContestExists
        }
    }

    // Check again after potentially long operation
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    // Select problems (concurrent operation)
    problems, err := s.problemService.SelectProblemsForContest(ctx, userID, req.ProblemCount)
    if err != nil {
        return nil, err
    }

    // ... create contest
}
```

**Key Points:**

1. **Early Check**: Fail fast if context already cancelled
2. **Check After Long Operations**: Re-check before expensive work
3. **Non-blocking Select**: `default` case prevents blocking

## Pattern 5: Graceful Shutdown

**Location**: `cmd/api/main.go`

**Problem**: Server should complete in-flight requests before stopping.

**Solution**: Signal handling with timeout-based shutdown.

```go
func main() {
    // ... setup code

    server := &http.Server{
        Addr:    fmt.Sprintf(":%d", config.Server.Port),
        Handler: router,
    }

    // Start server in goroutine
    go func() {
        if err := server.ListenAndServe(); err != http.ErrServerClosed {
            logger.Fatal("Server error", zap.Error(err))
        }
    }()

    // Wait for interrupt signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    logger.Info("Shutting down server...")

    // Graceful shutdown with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := server.Shutdown(ctx); err != nil {
        logger.Error("Forced shutdown", zap.Error(err))
    }

    // Cleanup telemetry
    telemetry.Shutdown(ctx)

    logger.Info("Server exited")
}
```

**Key Points:**

1. **Buffered Signal Channel**: Prevents signal loss
2. **30-Second Timeout**: Balance between graceful and forced shutdown
3. **Ordered Cleanup**: Server first, then telemetry

## Pattern 6: Database Connection Pooling

**Location**: `internal/infrastructure/database.go`

**Problem**: Each request creating a new database connection is expensive.

**Solution**: GORM connection pool with configurable limits.

```go
func NewDatabase(config *DatabaseConfig, logger *zap.Logger) (*Database, error) {
    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
    if err != nil {
        return nil, err
    }

    sqlDB, err := db.DB()
    if err != nil {
        return nil, err
    }

    // Connection pool settings
    sqlDB.SetMaxOpenConns(config.MaxOpenConns)     // Default: 25
    sqlDB.SetMaxIdleConns(config.MaxIdleConns)     // Default: 5
    sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime) // Default: 1 hour

    return &Database{DB: db}, nil
}
```

**Key Points:**

1. **MaxOpenConns**: Limits total connections to prevent database overload
2. **MaxIdleConns**: Keeps connections ready for reuse
3. **ConnMaxLifetime**: Prevents stale connections

## Concurrency Best Practices

### DO ✅

```go
// Pass loop variables explicitly to goroutines
for _, item := range items {
    go func(i Item) {
        process(i)
    }(item)
}

// Use buffered channels to prevent goroutine leaks
results := make(chan Result, len(items))

// Always propagate context
func (s *Service) Work(ctx context.Context) error {
    return s.repo.Query(ctx)
}

// Use defer for cleanup
ctx, cancel := context.WithTimeout(parent, timeout)
defer cancel()
```

### DON'T ❌

```go
// Don't capture loop variables by reference
for _, item := range items {
    go func() {
        process(item) // BUG: data race
    }()
}

// Don't ignore context cancellation
func (s *Service) Work(ctx context.Context) error {
    // This: BAD - ignores cancellation
    return s.repo.Query(context.Background())
}

// Don't create unbounded goroutines
for _, item := range hugeList {
    go process(item) // May spawn millions of goroutines
}
```

## Testing Concurrent Code

### Race Detection

Always run tests with race detector:

```bash
go test -race ./...
```

### Testing with Channels

```go
func TestFanOut(t *testing.T) {
    results := make(chan int, 3)
    
    // Launch concurrent workers
    for i := 0; i < 3; i++ {
        go func(n int) {
            results <- n * 2
        }(i)
    }
    
    // Collect with timeout
    var collected []int
    for i := 0; i < 3; i++ {
        select {
        case r := <-results:
            collected = append(collected, r)
        case <-time.After(time.Second):
            t.Fatal("timeout waiting for results")
        }
    }
    
    sort.Ints(collected)
    assert.Equal(t, []int{0, 2, 4}, collected)
}
```

## Performance Considerations

| Pattern | When to Use | Overhead |
|---------|-------------|----------|
| Fan-Out/Fan-In | Independent I/O operations | Low (goroutine + channel) |
| Worker Pool | Bounded parallelism needed | Medium (coordination) |
| Mutex | Protecting shared state | Very low |
| Channel | Communication between goroutines | Low |

## Debugging Tips

1. **Goroutine Leaks**: Use `runtime.NumGoroutine()` in tests
2. **Deadlocks**: Run with `GODEBUG=asyncpreemptoff=1` for clearer stack traces
3. **Race Conditions**: Always use `go test -race`
4. **Profiling**: Use `pprof` for goroutine flamegraphs
