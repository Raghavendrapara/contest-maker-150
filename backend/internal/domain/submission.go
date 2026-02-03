package domain

import (
	"time"

	"github.com/google/uuid"
)

// Submission represents a user's completion of a problem
// This tracks when a user marks a problem as solved, for avoiding repeats
type Submission struct {
	ID        uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID  `json:"user_id" gorm:"type:uuid;not null;index"`
	ProblemID uuid.UUID  `json:"problem_id" gorm:"type:uuid;not null;index"`
	ContestID *uuid.UUID `json:"contest_id" gorm:"type:uuid;index"` // Optional, can solve outside contest
	SolvedAt  time.Time  `json:"solved_at" gorm:"not null"`

	// Relationships
	User    User    `json:"-" gorm:"foreignKey:UserID"`
	Problem Problem `json:"problem" gorm:"foreignKey:ProblemID"`
	Contest *Contest `json:"-" gorm:"foreignKey:ContestID"`
}

// TableName specifies the table name for GORM
func (Submission) TableName() string {
	return "submissions"
}

// SubmissionRepository defines the interface for submission data access
type SubmissionRepository interface {
	Create(submission *Submission) error
	FindByID(id uuid.UUID) (*Submission, error)
	FindByUserID(userID uuid.UUID) ([]Submission, error)
	FindByUserAndProblem(userID, problemID uuid.UUID) (*Submission, error)
	FindByContestID(contestID uuid.UUID) ([]Submission, error)
	ExistsByUserAndProblem(userID, problemID uuid.UUID) (bool, error)
	CountByUserID(userID uuid.UUID) (int64, error)
	CountByUserAndDifficulty(userID uuid.UUID, difficulty Difficulty) (int64, error)
	Delete(id uuid.UUID) error
}

// SubmissionResponse represents a submission in API responses
type SubmissionResponse struct {
	ID        uuid.UUID       `json:"id"`
	Problem   ProblemResponse `json:"problem"`
	ContestID *uuid.UUID      `json:"contest_id"`
	SolvedAt  time.Time       `json:"solved_at"`
}

// ToResponse converts a Submission to a SubmissionResponse
func (s *Submission) ToResponse() SubmissionResponse {
	return SubmissionResponse{
		ID:        s.ID,
		Problem:   s.Problem.ToResponse(),
		ContestID: s.ContestID,
		SolvedAt:  s.SolvedAt,
	}
}
