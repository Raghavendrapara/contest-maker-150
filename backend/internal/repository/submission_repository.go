package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/contest-maker-150/backend/internal/domain"
)

// submissionRepository implements domain.SubmissionRepository using GORM
type submissionRepository struct {
	db *gorm.DB
}

// NewSubmissionRepository creates a new submission repository
func NewSubmissionRepository(db *gorm.DB) domain.SubmissionRepository {
	return &submissionRepository{db: db}
}

// Create creates a new submission record
func (r *submissionRepository) Create(submission *domain.Submission) error {
	return r.db.Create(submission).Error
}

// FindByID finds a submission by its ID
func (r *submissionRepository) FindByID(id uuid.UUID) (*domain.Submission, error) {
	var submission domain.Submission
	result := r.db.Preload("Problem").Where("id = ?", id).First(&submission)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, domain.ErrSubmissionNotFound
		}
		return nil, result.Error
	}
	return &submission, nil
}

// FindByUserID returns all submissions for a user
func (r *submissionRepository) FindByUserID(userID uuid.UUID) ([]domain.Submission, error) {
	var submissions []domain.Submission
	result := r.db.
		Preload("Problem").
		Where("user_id = ?", userID).
		Order("solved_at DESC").
		Find(&submissions)
	return submissions, result.Error
}

// FindByUserAndProblem finds a specific submission by user and problem
func (r *submissionRepository) FindByUserAndProblem(userID, problemID uuid.UUID) (*domain.Submission, error) {
	var submission domain.Submission
	result := r.db.
		Preload("Problem").
		Where("user_id = ? AND problem_id = ?", userID, problemID).
		First(&submission)
	
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil // Not found is not an error here
		}
		return nil, result.Error
	}
	return &submission, nil
}

// FindByContestID returns all submissions for a contest
func (r *submissionRepository) FindByContestID(contestID uuid.UUID) ([]domain.Submission, error) {
	var submissions []domain.Submission
	result := r.db.
		Preload("Problem").
		Where("contest_id = ?", contestID).
		Order("solved_at ASC").
		Find(&submissions)
	return submissions, result.Error
}

// ExistsByUserAndProblem checks if a user has already solved a problem
func (r *submissionRepository) ExistsByUserAndProblem(userID, problemID uuid.UUID) (bool, error) {
	var count int64
	result := r.db.Model(&domain.Submission{}).
		Where("user_id = ? AND problem_id = ?", userID, problemID).
		Count(&count)
	return count > 0, result.Error
}

// CountByUserID returns the total number of submissions for a user
func (r *submissionRepository) CountByUserID(userID uuid.UUID) (int64, error) {
	var count int64
	result := r.db.Model(&domain.Submission{}).
		Where("user_id = ?", userID).
		Distinct("problem_id").
		Count(&count)
	return count, result.Error
}

// CountByUserAndDifficulty returns the count of solved problems by difficulty
func (r *submissionRepository) CountByUserAndDifficulty(userID uuid.UUID, difficulty domain.Difficulty) (int64, error) {
	var count int64
	result := r.db.Model(&domain.Submission{}).
		Joins("JOIN problems ON submissions.problem_id = problems.id").
		Where("submissions.user_id = ? AND problems.difficulty = ?", userID, difficulty).
		Distinct("submissions.problem_id").
		Count(&count)
	return count, result.Error
}

// Delete deletes a submission by its ID
func (r *submissionRepository) Delete(id uuid.UUID) error {
	result := r.db.Delete(&domain.Submission{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrSubmissionNotFound
	}
	return nil
}

// WithContext returns a repository with the given context for tracing
func (r *submissionRepository) WithContext(ctx context.Context) domain.SubmissionRepository {
	return &submissionRepository{db: r.db.WithContext(ctx)}
}
