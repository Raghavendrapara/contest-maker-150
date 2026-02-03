package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/contest-maker-150/backend/internal/domain"
)

// contestRepository implements domain.ContestRepository using GORM
type contestRepository struct {
	db *gorm.DB
}

// NewContestRepository creates a new contest repository
func NewContestRepository(db *gorm.DB) domain.ContestRepository {
	return &contestRepository{db: db}
}

// Create creates a new contest in the database
func (r *contestRepository) Create(contest *domain.Contest) error {
	return r.db.Create(contest).Error
}

// FindByID finds a contest by its ID (without problems)
func (r *contestRepository) FindByID(id uuid.UUID) (*domain.Contest, error) {
	var contest domain.Contest
	result := r.db.Where("id = ?", id).First(&contest)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, domain.ErrContestNotFound
		}
		return nil, result.Error
	}
	return &contest, nil
}

// FindByIDWithProblems finds a contest with all its problems loaded
func (r *contestRepository) FindByIDWithProblems(id uuid.UUID) (*domain.Contest, error) {
	var contest domain.Contest
	result := r.db.
		Preload("ContestProblems", func(db *gorm.DB) *gorm.DB {
			return db.Order("contest_problems.order ASC")
		}).
		Preload("ContestProblems.Problem").
		Where("id = ?", id).
		First(&contest)
	
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, domain.ErrContestNotFound
		}
		return nil, result.Error
	}
	return &contest, nil
}

// FindByUserID returns all contests for a user ordered by creation date
func (r *contestRepository) FindByUserID(userID uuid.UUID) ([]domain.Contest, error) {
	var contests []domain.Contest
	result := r.db.
		Preload("ContestProblems", func(db *gorm.DB) *gorm.DB {
			return db.Order("contest_problems.order ASC")
		}).
		Preload("ContestProblems.Problem").
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&contests)
	
	return contests, result.Error
}

// FindActiveByUserID finds the active contest for a user (if any)
func (r *contestRepository) FindActiveByUserID(userID uuid.UUID) (*domain.Contest, error) {
	var contest domain.Contest
	result := r.db.
		Preload("ContestProblems", func(db *gorm.DB) *gorm.DB {
			return db.Order("contest_problems.order ASC")
		}).
		Preload("ContestProblems.Problem").
		Where("user_id = ? AND status = ?", userID, domain.ContestStatusActive).
		First(&contest)
	
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil // No active contest is not an error
		}
		return nil, result.Error
	}
	return &contest, nil
}

// Update updates an existing contest
func (r *contestRepository) Update(contest *domain.Contest) error {
	return r.db.Save(contest).Error
}

// UpdateProblemStatus marks a problem as completed or not completed
func (r *contestRepository) UpdateProblemStatus(contestID, problemID uuid.UUID, isCompleted bool) error {
	result := r.db.Model(&domain.ContestProblem{}).
		Where("contest_id = ? AND problem_id = ?", contestID, problemID).
		Update("is_completed", isCompleted)
	
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrProblemNotInContest
	}
	return nil
}

// Delete deletes a contest by its ID
func (r *contestRepository) Delete(id uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Delete contest problems first (cascade)
		if err := tx.Delete(&domain.ContestProblem{}, "contest_id = ?", id).Error; err != nil {
			return err
		}
		// Delete the contest
		result := tx.Delete(&domain.Contest{}, "id = ?", id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return domain.ErrContestNotFound
		}
		return nil
	})
}

// AddProblems adds problems to a contest
func (r *contestRepository) AddProblems(contestID uuid.UUID, problems []domain.ContestProblem) error {
	for i := range problems {
		problems[i].ContestID = contestID
	}
	return r.db.Create(&problems).Error
}

// WithContext returns a repository with the given context for tracing
func (r *contestRepository) WithContext(ctx context.Context) domain.ContestRepository {
	return &contestRepository{db: r.db.WithContext(ctx)}
}
