package domain

import (
	"time"

	"github.com/google/uuid"
)

// ContestStatus represents the current state of a contest
type ContestStatus string

const (
	ContestStatusActive    ContestStatus = "active"
	ContestStatusCompleted ContestStatus = "completed"
	ContestStatusAbandoned ContestStatus = "abandoned"
)

// Contest represents a timed coding challenge session
type Contest struct {
	ID              uuid.UUID     `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID          uuid.UUID     `json:"user_id" gorm:"type:uuid;not null;index"`
	DurationMinutes int           `json:"duration_minutes" gorm:"not null"`
	StartedAt       time.Time     `json:"started_at" gorm:"not null"`
	EndedAt         *time.Time    `json:"ended_at"`
	Status          ContestStatus `json:"status" gorm:"type:varchar(20);not null;default:'active'"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`

	// Relationships
	User            User             `json:"-" gorm:"foreignKey:UserID"`
	ContestProblems []ContestProblem `json:"problems,omitempty" gorm:"foreignKey:ContestID"`
}

// TableName specifies the table name for GORM
func (Contest) TableName() string {
	return "contests"
}

// ContestProblem represents a problem within a specific contest
type ContestProblem struct {
	ContestID   uuid.UUID `json:"contest_id" gorm:"type:uuid;primaryKey"`
	ProblemID   uuid.UUID `json:"problem_id" gorm:"type:uuid;primaryKey"`
	Order       int       `json:"order" gorm:"not null"`
	IsCompleted bool      `json:"is_completed" gorm:"default:false"`

	// Relationships (for loading)
	Problem Problem `json:"problem" gorm:"foreignKey:ProblemID"`
}

// TableName specifies the table name for GORM
func (ContestProblem) TableName() string {
	return "contest_problems"
}

// ContestRepository defines the interface for contest data access
type ContestRepository interface {
	Create(contest *Contest) error
	FindByID(id uuid.UUID) (*Contest, error)
	FindByIDWithProblems(id uuid.UUID) (*Contest, error)
	FindByUserID(userID uuid.UUID) ([]Contest, error)
	FindActiveByUserID(userID uuid.UUID) (*Contest, error)
	Update(contest *Contest) error
	UpdateProblemStatus(contestID, problemID uuid.UUID, isCompleted bool) error
	Delete(id uuid.UUID) error
	AddProblems(contestID uuid.UUID, problems []ContestProblem) error
}

// CreateContestRequest represents the data needed to create a new contest
type CreateContestRequest struct {
	ProblemCount    int `json:"problem_count" binding:"required,min=1,max=20"`
	DurationMinutes int `json:"duration_minutes" binding:"required,min=10,max=300"`
}

// ContestResponse represents a contest in API responses
type ContestResponse struct {
	ID              uuid.UUID                `json:"id"`
	DurationMinutes int                      `json:"duration_minutes"`
	StartedAt       time.Time                `json:"started_at"`
	EndedAt         *time.Time               `json:"ended_at"`
	Status          ContestStatus            `json:"status"`
	Problems        []ContestProblemResponse `json:"problems"`
	TimeRemaining   int                      `json:"time_remaining_seconds"`
}

// ContestProblemResponse represents a problem within a contest response
type ContestProblemResponse struct {
	Order       int             `json:"order"`
	IsCompleted bool            `json:"is_completed"`
	Problem     ProblemResponse `json:"problem"`
}

// ToResponse converts a Contest to a ContestResponse
func (c *Contest) ToResponse() ContestResponse {
	problems := make([]ContestProblemResponse, len(c.ContestProblems))
	for i, cp := range c.ContestProblems {
		problems[i] = ContestProblemResponse{
			Order:       cp.Order,
			IsCompleted: cp.IsCompleted,
			Problem:     cp.Problem.ToResponse(),
		}
	}

	// Calculate remaining time
	var timeRemaining int
	if c.Status == ContestStatusActive {
		endTime := c.StartedAt.Add(time.Duration(c.DurationMinutes) * time.Minute)
		remaining := time.Until(endTime)
		if remaining > 0 {
			timeRemaining = int(remaining.Seconds())
		}
	}

	return ContestResponse{
		ID:              c.ID,
		DurationMinutes: c.DurationMinutes,
		StartedAt:       c.StartedAt,
		EndedAt:         c.EndedAt,
		Status:          c.Status,
		Problems:        problems,
		TimeRemaining:   timeRemaining,
	}
}

// IsExpired checks if the contest timer has expired
func (c *Contest) IsExpired() bool {
	if c.Status != ContestStatusActive {
		return false
	}
	endTime := c.StartedAt.Add(time.Duration(c.DurationMinutes) * time.Minute)
	return time.Now().After(endTime)
}

// MarkProblemCompleteRequest represents the request to mark a problem as complete
type MarkProblemCompleteRequest struct {
	IsCompleted bool `json:"is_completed"`
}
