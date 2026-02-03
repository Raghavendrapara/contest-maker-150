package service

import (
	"context"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/contest-maker-150/backend/internal/domain"
)

// ProblemService handles problem-related business logic
type ProblemService struct {
	problemRepo domain.ProblemRepository
	userRepo    domain.UserRepository
	tracer      trace.Tracer
	logger      *zap.Logger
	rng         *rand.Rand
	rngMu       sync.Mutex // Protects rng for concurrent access
}

// NewProblemService creates a new problem service
func NewProblemService(
	problemRepo domain.ProblemRepository,
	userRepo domain.UserRepository,
	tracer trace.Tracer,
	logger *zap.Logger,
) *ProblemService {
	return &ProblemService{
		problemRepo: problemRepo,
		userRepo:    userRepo,
		tracer:      tracer,
		logger:      logger,
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GetAllProblems returns all problems
func (s *ProblemService) GetAllProblems(ctx context.Context) ([]domain.Problem, error) {
	ctx, span := s.tracer.Start(ctx, "ProblemService.GetAllProblems")
	defer span.End()

	return s.problemRepo.FindAll()
}

// GetProblemByID returns a specific problem
func (s *ProblemService) GetProblemByID(ctx context.Context, id uuid.UUID) (*domain.Problem, error) {
	ctx, span := s.tracer.Start(ctx, "ProblemService.GetProblemByID")
	defer span.End()

	span.SetAttributes(attribute.String("problem.id", id.String()))
	return s.problemRepo.FindByID(id)
}

// GetProblemStats returns statistics about the problem set
func (s *ProblemService) GetProblemStats(ctx context.Context) (*domain.ProblemStats, error) {
	ctx, span := s.tracer.Start(ctx, "ProblemService.GetProblemStats")
	defer span.End()

	problems, err := s.problemRepo.FindAll()
	if err != nil {
		return nil, err
	}

	stats := &domain.ProblemStats{
		Total:        len(problems),
		ByDifficulty: make(map[domain.Difficulty]int),
		ByTopic:      make(map[string]int),
	}

	for _, p := range problems {
		stats.ByDifficulty[p.Difficulty]++
		for _, topic := range p.Topics {
			stats.ByTopic[topic]++
		}
	}

	return stats, nil
}

// SelectProblemsForContest selects n problems with gradual difficulty increase
// The algorithm:
// 1. Exclude previously solved problems for the user
// 2. Group remaining problems by difficulty
// 3. Distribute across difficulties based on n (Easy → Medium → Hard progression)
// 4. Randomize within each difficulty bucket
// 5. Sort final list by difficulty (ascending)
func (s *ProblemService) SelectProblemsForContest(ctx context.Context, userID uuid.UUID, count int) ([]domain.Problem, error) {
	ctx, span := s.tracer.Start(ctx, "ProblemService.SelectProblemsForContest")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userID.String()),
		attribute.Int("problem.count", count),
	)

	// Use worker pool pattern for parallel fetching of problems by difficulty
	type difficultyResult struct {
		difficulty domain.Difficulty
		problems   []domain.Problem
		err        error
	}

	difficulties := []domain.Difficulty{
		domain.DifficultyEasy,
		domain.DifficultyMedium,
		domain.DifficultyHard,
	}

	resultChan := make(chan difficultyResult, len(difficulties))
	var wg sync.WaitGroup

	// Worker function to fetch problems by difficulty
	fetchProblems := func(diff domain.Difficulty) {
		defer wg.Done()
		problems, err := s.problemRepo.FindUnsolvedByUserAndDifficulty(userID, diff)
		resultChan <- difficultyResult{
			difficulty: diff,
			problems:   problems,
			err:        err,
		}
	}

	// Launch workers
	for _, diff := range difficulties {
		wg.Add(1)
		go fetchProblems(diff)
	}

	// Wait for all workers and close channel
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	problemsByDifficulty := make(map[domain.Difficulty][]domain.Problem)
	for result := range resultChan {
		if result.err != nil {
			s.logger.Error("Failed to fetch problems by difficulty",
				zap.String("difficulty", string(result.difficulty)),
				zap.Error(result.err),
			)
			continue
		}
		problemsByDifficulty[result.difficulty] = result.problems
	}

	// Calculate distribution based on count
	distribution := s.calculateDistribution(count)

	span.SetAttributes(
		attribute.Int("distribution.easy", distribution[domain.DifficultyEasy]),
		attribute.Int("distribution.medium", distribution[domain.DifficultyMedium]),
		attribute.Int("distribution.hard", distribution[domain.DifficultyHard]),
	)

	// Select problems according to distribution
	var selectedProblems []domain.Problem
	shortfall := 0

	for _, diff := range []domain.Difficulty{domain.DifficultyEasy, domain.DifficultyMedium, domain.DifficultyHard} {
		needed := distribution[diff] + shortfall
		available := problemsByDifficulty[diff]

		if len(available) < needed {
			// Not enough problems at this difficulty, carry over to next
			shortfall = needed - len(available)
			selectedProblems = append(selectedProblems, available...)
		} else {
			// Randomly select from available
			selected := s.randomSelect(available, needed)
			selectedProblems = append(selectedProblems, selected...)
			shortfall = 0
		}
	}

	// Check if we have enough problems
	if len(selectedProblems) < count {
		s.logger.Warn("Not enough unsolved problems available",
			zap.Int("requested", count),
			zap.Int("available", len(selectedProblems)),
		)
		if len(selectedProblems) == 0 {
			return nil, domain.ErrNotEnoughProblems
		}
	}

	// Sort by difficulty (for proper progression)
	sort.Slice(selectedProblems, func(i, j int) bool {
		return selectedProblems[i].Difficulty.Weight() < selectedProblems[j].Difficulty.Weight()
	})

	s.logger.Info("Problems selected for contest",
		zap.String("user_id", userID.String()),
		zap.Int("count", len(selectedProblems)),
	)

	return selectedProblems, nil
}

// calculateDistribution determines how many problems of each difficulty to select
// The idea is to have a gradual progression from easy to hard
func (s *ProblemService) calculateDistribution(count int) map[domain.Difficulty]int {
	distribution := make(map[domain.Difficulty]int)

	switch {
	case count <= 3:
		// For very small contests, mostly easy with maybe one medium
		distribution[domain.DifficultyEasy] = count
		if count >= 2 {
			distribution[domain.DifficultyEasy] = count - 1
			distribution[domain.DifficultyMedium] = 1
		}
	case count <= 5:
		// 2 Easy, 2 Medium, 1 Hard (or proportional)
		distribution[domain.DifficultyEasy] = 2
		distribution[domain.DifficultyMedium] = 2
		distribution[domain.DifficultyHard] = count - 4
	case count <= 10:
		// 30% Easy, 40% Medium, 30% Hard
		distribution[domain.DifficultyEasy] = count * 3 / 10
		distribution[domain.DifficultyMedium] = count * 4 / 10
		distribution[domain.DifficultyHard] = count - distribution[domain.DifficultyEasy] - distribution[domain.DifficultyMedium]
	default:
		// For larger contests: 25% Easy, 50% Medium, 25% Hard
		distribution[domain.DifficultyEasy] = count / 4
		distribution[domain.DifficultyMedium] = count / 2
		distribution[domain.DifficultyHard] = count - distribution[domain.DifficultyEasy] - distribution[domain.DifficultyMedium]
	}

	// Ensure at least one problem per difficulty if count allows
	if count >= 3 {
		for _, diff := range []domain.Difficulty{domain.DifficultyEasy, domain.DifficultyMedium, domain.DifficultyHard} {
			if distribution[diff] == 0 {
				distribution[diff] = 1
			}
		}
	}

	return distribution
}

// randomSelect randomly selects n problems from the given slice
// Uses Fisher-Yates shuffle (thread-safe)
func (s *ProblemService) randomSelect(problems []domain.Problem, n int) []domain.Problem {
	if n >= len(problems) {
		return problems
	}

	// Make a copy to avoid modifying the original
	shuffled := make([]domain.Problem, len(problems))
	copy(shuffled, problems)

	// Fisher-Yates shuffle (partial, only need first n elements)
	s.rngMu.Lock()
	for i := 0; i < n; i++ {
		j := i + s.rng.Intn(len(shuffled)-i)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}
	s.rngMu.Unlock()

	return shuffled[:n]
}
