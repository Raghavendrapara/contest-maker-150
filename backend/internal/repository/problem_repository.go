package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/contest-maker-150/backend/internal/domain"
)

// problemRepository implements domain.ProblemRepository using GORM
type problemRepository struct {
	db *gorm.DB
}

// NewProblemRepository creates a new problem repository
func NewProblemRepository(db *gorm.DB) domain.ProblemRepository {
	return &problemRepository{db: db}
}

// Create creates a new problem in the database
func (r *problemRepository) Create(problem *domain.Problem) error {
	return r.db.Create(problem).Error
}

// CreateBatch creates multiple problems in a single transaction
func (r *problemRepository) CreateBatch(problems []domain.Problem) error {
	return r.db.CreateInBatches(problems, 50).Error
}

// FindByID finds a problem by its ID
func (r *problemRepository) FindByID(id uuid.UUID) (*domain.Problem, error) {
	var problem domain.Problem
	result := r.db.Where("id = ?", id).First(&problem)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, domain.ErrProblemNotFound
		}
		return nil, result.Error
	}
	return &problem, nil
}

// FindBySlug finds a problem by its slug
func (r *problemRepository) FindBySlug(slug string) (*domain.Problem, error) {
	var problem domain.Problem
	result := r.db.Where("slug = ?", slug).First(&problem)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, domain.ErrProblemNotFound
		}
		return nil, result.Error
	}
	return &problem, nil
}

// FindAll returns all problems ordered by order_index
func (r *problemRepository) FindAll() ([]domain.Problem, error) {
	var problems []domain.Problem
	result := r.db.Order("order_index ASC").Find(&problems)
	return problems, result.Error
}

// FindByDifficulty returns all problems with the specified difficulty
func (r *problemRepository) FindByDifficulty(difficulty domain.Difficulty) ([]domain.Problem, error) {
	var problems []domain.Problem
	result := r.db.Where("difficulty = ?", difficulty).Order("order_index ASC").Find(&problems)
	return problems, result.Error
}

// FindByTopics returns all problems that match any of the given topics
func (r *problemRepository) FindByTopics(topics []string) ([]domain.Problem, error) {
	var problems []domain.Problem
	result := r.db.Where("topics && ?", topics).Order("order_index ASC").Find(&problems)
	return problems, result.Error
}

// FindUnsolvedByUser returns all problems not yet solved by the user
func (r *problemRepository) FindUnsolvedByUser(userID uuid.UUID) ([]domain.Problem, error) {
	var problems []domain.Problem
	
	// Subquery to get solved problem IDs
	solvedSubquery := r.db.Model(&domain.Submission{}).
		Select("problem_id").
		Where("user_id = ?", userID)
	
	result := r.db.Where("id NOT IN (?)", solvedSubquery).
		Order("order_index ASC").
		Find(&problems)
	
	return problems, result.Error
}

// FindUnsolvedByUserAndDifficulty returns unsolved problems for a user filtered by difficulty
func (r *problemRepository) FindUnsolvedByUserAndDifficulty(userID uuid.UUID, difficulty domain.Difficulty) ([]domain.Problem, error) {
	var problems []domain.Problem
	
	// Subquery to get solved problem IDs
	solvedSubquery := r.db.Model(&domain.Submission{}).
		Select("problem_id").
		Where("user_id = ?", userID)
	
	result := r.db.Where("id NOT IN (?)", solvedSubquery).
		Where("difficulty = ?", difficulty).
		Order("RANDOM()"). // Randomize selection within difficulty
		Find(&problems)
	
	return problems, result.Error
}

// Count returns the total number of problems
func (r *problemRepository) Count() (int64, error) {
	var count int64
	result := r.db.Model(&domain.Problem{}).Count(&count)
	return count, result.Error
}

// WithContext returns a repository with the given context for tracing
func (r *problemRepository) WithContext(ctx context.Context) domain.ProblemRepository {
	return &problemRepository{db: r.db.WithContext(ctx)}
}
