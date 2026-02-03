package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/contest-maker-150/backend/internal/domain"
)

// ContestService handles contest-related business logic
type ContestService struct {
	contestRepo    domain.ContestRepository
	problemService *ProblemService
	subRepo        domain.SubmissionRepository
	tracer         trace.Tracer
	logger         *zap.Logger
}

// NewContestService creates a new contest service
func NewContestService(
	contestRepo domain.ContestRepository,
	problemService *ProblemService,
	subRepo domain.SubmissionRepository,
	tracer trace.Tracer,
	logger *zap.Logger,
) *ContestService {
	return &ContestService{
		contestRepo:    contestRepo,
		problemService: problemService,
		subRepo:        subRepo,
		tracer:         tracer,
		logger:         logger,
	}
}

// CreateContest creates a new contest for a user
func (s *ContestService) CreateContest(ctx context.Context, userID uuid.UUID, req *domain.CreateContestRequest) (*domain.Contest, error) {
	ctx, span := s.tracer.Start(ctx, "ContestService.CreateContest")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userID.String()),
		attribute.Int("problem.count", req.ProblemCount),
		attribute.Int("duration.minutes", req.DurationMinutes),
	)

	// Check if user already has an active contest
	activeContest, err := s.contestRepo.FindActiveByUserID(userID)
	if err != nil {
		return nil, err
	}
	if activeContest != nil {
		// Check if it's expired
		if activeContest.IsExpired() {
			// Auto-complete expired contest
			now := time.Now()
			activeContest.Status = domain.ContestStatusCompleted
			activeContest.EndedAt = &now
			if err := s.contestRepo.Update(activeContest); err != nil {
				s.logger.Error("Failed to complete expired contest", zap.Error(err))
			}
		} else {
			return nil, domain.ErrActiveContestExists
		}
	}

	// Select problems for the contest
	problems, err := s.problemService.SelectProblemsForContest(ctx, userID, req.ProblemCount)
	if err != nil {
		return nil, err
	}

	// Create the contest
	contest := &domain.Contest{
		UserID:          userID,
		DurationMinutes: req.DurationMinutes,
		StartedAt:       time.Now(),
		Status:          domain.ContestStatusActive,
	}

	if err := s.contestRepo.Create(contest); err != nil {
		return nil, err
	}

	// Create contest problems with order
	contestProblems := make([]domain.ContestProblem, len(problems))
	for i, p := range problems {
		contestProblems[i] = domain.ContestProblem{
			ContestID:   contest.ID,
			ProblemID:   p.ID,
			Order:       i + 1,
			IsCompleted: false,
			Problem:     p, // Include problem data for response
		}
	}

	if err := s.contestRepo.AddProblems(contest.ID, contestProblems); err != nil {
		// Rollback: delete the contest
		_ = s.contestRepo.Delete(contest.ID)
		return nil, err
	}

	// Attach problems to contest for response
	contest.ContestProblems = contestProblems

	s.logger.Info("Contest created",
		zap.String("contest_id", contest.ID.String()),
		zap.String("user_id", userID.String()),
		zap.Int("problem_count", len(problems)),
	)

	return contest, nil
}

// GetContestByID retrieves a contest by ID
func (s *ContestService) GetContestByID(ctx context.Context, contestID uuid.UUID) (*domain.Contest, error) {
	ctx, span := s.tracer.Start(ctx, "ContestService.GetContestByID")
	defer span.End()

	span.SetAttributes(attribute.String("contest.id", contestID.String()))

	contest, err := s.contestRepo.FindByIDWithProblems(contestID)
	if err != nil {
		return nil, err
	}

	// Check and update expired status
	if contest.IsExpired() {
		now := time.Now()
		contest.Status = domain.ContestStatusCompleted
		contest.EndedAt = &now
		if err := s.contestRepo.Update(contest); err != nil {
			s.logger.Error("Failed to complete expired contest", zap.Error(err))
		}
	}

	return contest, nil
}

// GetUserContests retrieves all contests for a user
func (s *ContestService) GetUserContests(ctx context.Context, userID uuid.UUID) ([]domain.Contest, error) {
	ctx, span := s.tracer.Start(ctx, "ContestService.GetUserContests")
	defer span.End()

	span.SetAttributes(attribute.String("user.id", userID.String()))
	return s.contestRepo.FindByUserID(userID)
}

// GetActiveContest retrieves the user's active contest if any
func (s *ContestService) GetActiveContest(ctx context.Context, userID uuid.UUID) (*domain.Contest, error) {
	ctx, span := s.tracer.Start(ctx, "ContestService.GetActiveContest")
	defer span.End()

	span.SetAttributes(attribute.String("user.id", userID.String()))

	contest, err := s.contestRepo.FindActiveByUserID(userID)
	if err != nil {
		return nil, err
	}

	if contest == nil {
		return nil, nil
	}

	// Check and update expired status
	if contest.IsExpired() {
		now := time.Now()
		contest.Status = domain.ContestStatusCompleted
		contest.EndedAt = &now
		if err := s.contestRepo.Update(contest); err != nil {
			s.logger.Error("Failed to complete expired contest", zap.Error(err))
		}
	}

	return contest, nil
}

// MarkProblemComplete marks a problem as completed in a contest
func (s *ContestService) MarkProblemComplete(ctx context.Context, userID, contestID, problemID uuid.UUID, isCompleted bool) error {
	ctx, span := s.tracer.Start(ctx, "ContestService.MarkProblemComplete")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userID.String()),
		attribute.String("contest.id", contestID.String()),
		attribute.String("problem.id", problemID.String()),
		attribute.Bool("is_completed", isCompleted),
	)

	// Get the contest
	contest, err := s.contestRepo.FindByID(contestID)
	if err != nil {
		return err
	}

	// Verify ownership
	if contest.UserID != userID {
		return domain.ErrForbidden
	}

	// Check if contest is active
	if contest.Status != domain.ContestStatusActive {
		return domain.ErrContestNotActive
	}

	// Check if contest is expired
	if contest.IsExpired() {
		return domain.ErrContestExpired
	}

	// Update problem status
	if err := s.contestRepo.UpdateProblemStatus(contestID, problemID, isCompleted); err != nil {
		return err
	}

	// If marking as complete, also create a submission record
	if isCompleted {
		// Check if already submitted
		existing, err := s.subRepo.FindByUserAndProblem(userID, problemID)
		if err != nil {
			s.logger.Error("Failed to check existing submission", zap.Error(err))
		}

		if existing == nil {
			submission := &domain.Submission{
				UserID:    userID,
				ProblemID: problemID,
				ContestID: &contestID,
				SolvedAt:  time.Now(),
			}
			if err := s.subRepo.Create(submission); err != nil {
				s.logger.Error("Failed to create submission", zap.Error(err))
			}
		}
	}

	s.logger.Info("Problem marked as complete",
		zap.String("contest_id", contestID.String()),
		zap.String("problem_id", problemID.String()),
		zap.Bool("is_completed", isCompleted),
	)

	return nil
}

// CompleteContest manually completes a contest
func (s *ContestService) CompleteContest(ctx context.Context, userID, contestID uuid.UUID) error {
	ctx, span := s.tracer.Start(ctx, "ContestService.CompleteContest")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userID.String()),
		attribute.String("contest.id", contestID.String()),
	)

	contest, err := s.contestRepo.FindByID(contestID)
	if err != nil {
		return err
	}

	// Verify ownership
	if contest.UserID != userID {
		return domain.ErrForbidden
	}

	// Check if contest is already completed
	if contest.Status != domain.ContestStatusActive {
		return domain.ErrContestNotActive
	}

	// Complete the contest
	now := time.Now()
	contest.Status = domain.ContestStatusCompleted
	contest.EndedAt = &now

	return s.contestRepo.Update(contest)
}

// AbandonContest abandons a contest
func (s *ContestService) AbandonContest(ctx context.Context, userID, contestID uuid.UUID) error {
	ctx, span := s.tracer.Start(ctx, "ContestService.AbandonContest")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userID.String()),
		attribute.String("contest.id", contestID.String()),
	)

	contest, err := s.contestRepo.FindByID(contestID)
	if err != nil {
		return err
	}

	// Verify ownership
	if contest.UserID != userID {
		return domain.ErrForbidden
	}

	// Check if contest is active
	if contest.Status != domain.ContestStatusActive {
		return domain.ErrContestNotActive
	}

	// Abandon the contest
	now := time.Now()
	contest.Status = domain.ContestStatusAbandoned
	contest.EndedAt = &now

	return s.contestRepo.Update(contest)
}
