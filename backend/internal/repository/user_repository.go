package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/contest-maker-150/backend/internal/domain"
)

// userRepository implements domain.UserRepository using GORM
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *gorm.DB) domain.UserRepository {
	return &userRepository{db: db}
}

// Create creates a new user in the database
func (r *userRepository) Create(user *domain.User) error {
	result := r.db.Create(user)
	if result.Error != nil {
		// Check for unique constraint violation
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return domain.ErrUserAlreadyExists
		}
		return result.Error
	}
	return nil
}

// FindByID finds a user by their ID
func (r *userRepository) FindByID(id uuid.UUID) (*domain.User, error) {
	var user domain.User
	result := r.db.Where("id = ?", id).First(&user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, domain.ErrUserNotFound
		}
		return nil, result.Error
	}
	return &user, nil
}

// FindByEmail finds a user by their email address
func (r *userRepository) FindByEmail(email string) (*domain.User, error) {
	var user domain.User
	result := r.db.Where("email = ?", email).First(&user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, domain.ErrUserNotFound
		}
		return nil, result.Error
	}
	return &user, nil
}

// Update updates an existing user
func (r *userRepository) Update(user *domain.User) error {
	result := r.db.Save(user)
	return result.Error
}

// Delete deletes a user by their ID
func (r *userRepository) Delete(id uuid.UUID) error {
	result := r.db.Delete(&domain.User{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

// GetSolvedProblemIDs returns a list of problem IDs that the user has solved
func (r *userRepository) GetSolvedProblemIDs(userID uuid.UUID) ([]uuid.UUID, error) {
	var problemIDs []uuid.UUID
	result := r.db.Model(&domain.Submission{}).
		Where("user_id = ?", userID).
		Distinct("problem_id").
		Pluck("problem_id", &problemIDs)
	if result.Error != nil {
		return nil, result.Error
	}
	return problemIDs, nil
}

// WithContext returns a repository with the given context for tracing
func (r *userRepository) WithContext(ctx context.Context) domain.UserRepository {
	return &userRepository{db: r.db.WithContext(ctx)}
}
