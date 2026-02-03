package domain

import (
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Difficulty represents the difficulty level of a problem
type Difficulty string

const (
	DifficultyEasy   Difficulty = "Easy"
	DifficultyMedium Difficulty = "Medium"
	DifficultyHard   Difficulty = "Hard"
)

// DifficultyWeight returns a numeric weight for sorting by difficulty
func (d Difficulty) Weight() int {
	switch d {
	case DifficultyEasy:
		return 1
	case DifficultyMedium:
		return 2
	case DifficultyHard:
		return 3
	default:
		return 0
	}
}

// Problem represents a coding problem from NeetCode 150
type Problem struct {
	ID          uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Title       string         `json:"title" gorm:"not null"`
	Slug        string         `json:"slug" gorm:"uniqueIndex;not null"`
	Difficulty  Difficulty     `json:"difficulty" gorm:"type:varchar(10);not null"`
	Topics      pq.StringArray `json:"topics" gorm:"type:text[]"`
	LeetCodeURL string         `json:"leetcode_url" gorm:"not null"`
	NeetCodeURL string         `json:"neetcode_url"`
	OrderIndex  int            `json:"order_index" gorm:"not null"` // Original order in NeetCode 150

	// Relationships
	ContestProblems []ContestProblem `json:"-" gorm:"foreignKey:ProblemID"`
	Submissions     []Submission     `json:"-" gorm:"foreignKey:ProblemID"`
}

// TableName specifies the table name for GORM
func (Problem) TableName() string {
	return "problems"
}

// ProblemRepository defines the interface for problem data access
type ProblemRepository interface {
	Create(problem *Problem) error
	CreateBatch(problems []Problem) error
	FindByID(id uuid.UUID) (*Problem, error)
	FindBySlug(slug string) (*Problem, error)
	FindAll() ([]Problem, error)
	FindByDifficulty(difficulty Difficulty) ([]Problem, error)
	FindByTopics(topics []string) ([]Problem, error)
	FindUnsolvedByUser(userID uuid.UUID) ([]Problem, error)
	FindUnsolvedByUserAndDifficulty(userID uuid.UUID, difficulty Difficulty) ([]Problem, error)
	Count() (int64, error)
}

// ProblemResponse represents a problem in API responses
type ProblemResponse struct {
	ID          uuid.UUID  `json:"id"`
	Title       string     `json:"title"`
	Slug        string     `json:"slug"`
	Difficulty  Difficulty `json:"difficulty"`
	Topics      []string   `json:"topics"`
	LeetCodeURL string     `json:"leetcode_url"`
	NeetCodeURL string     `json:"neetcode_url"`
}

// ToResponse converts a Problem to a ProblemResponse
func (p *Problem) ToResponse() ProblemResponse {
	return ProblemResponse{
		ID:          p.ID,
		Title:       p.Title,
		Slug:        p.Slug,
		Difficulty:  p.Difficulty,
		Topics:      p.Topics,
		LeetCodeURL: p.LeetCodeURL,
		NeetCodeURL: p.NeetCodeURL,
	}
}

// ProblemStats represents statistics about the problem set
type ProblemStats struct {
	Total      int            `json:"total"`
	ByDifficulty map[Difficulty]int `json:"by_difficulty"`
	ByTopic    map[string]int `json:"by_topic"`
}

// ProblemFilter represents filtering options for problem queries
type ProblemFilter struct {
	Difficulty *Difficulty
	Topics     []string
	ExcludeIDs []uuid.UUID
	Limit      int
}
