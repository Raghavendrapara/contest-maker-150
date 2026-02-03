package data

import (
	_ "embed"
	"encoding/json"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/contest-maker-150/backend/internal/domain"
)

//go:embed neetcode150.json
var neetcode150Data []byte

// problemJSON represents the JSON structure for problems
type problemJSON struct {
	Title       string   `json:"title"`
	Slug        string   `json:"slug"`
	Difficulty  string   `json:"difficulty"`
	Topics      []string `json:"topics"`
	LeetCodeURL string   `json:"leetcode_url"`
	NeetCodeURL string   `json:"neetcode_url"`
	OrderIndex  int      `json:"order_index"`
}

// Seeder handles database seeding operations
type Seeder struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewSeeder creates a new database seeder
func NewSeeder(db *gorm.DB, logger *zap.Logger) *Seeder {
	return &Seeder{
		db:     db,
		logger: logger,
	}
}

// SeedProblems seeds the problems table with NeetCode 150 data
// It uses upsert semantics to avoid duplicates
func (s *Seeder) SeedProblems() error {
	s.logger.Info("Starting to seed problems...")

	// Check if problems already exist
	var count int64
	if err := s.db.Model(&domain.Problem{}).Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		s.logger.Info("Problems already seeded, skipping",
			zap.Int64("count", count),
		)
		return nil
	}

	// Parse embedded JSON
	var problemsJSON []problemJSON
	if err := json.Unmarshal(neetcode150Data, &problemsJSON); err != nil {
		return err
	}

	// Convert to domain models
	problems := make([]domain.Problem, len(problemsJSON))
	for i, p := range problemsJSON {
		problems[i] = domain.Problem{
			ID:          uuid.New(),
			Title:       p.Title,
			Slug:        p.Slug,
			Difficulty:  domain.Difficulty(p.Difficulty),
			Topics:      p.Topics,
			LeetCodeURL: p.LeetCodeURL,
			NeetCodeURL: p.NeetCodeURL,
			OrderIndex:  p.OrderIndex,
		}
	}

	// Batch insert
	if err := s.db.CreateInBatches(problems, 50).Error; err != nil {
		return err
	}

	s.logger.Info("Successfully seeded problems",
		zap.Int("count", len(problems)),
	)

	return nil
}

// GetEmbeddedProblems returns the embedded NeetCode 150 problems
// Useful for testing or direct access
func GetEmbeddedProblems() ([]domain.Problem, error) {
	var problemsJSON []problemJSON
	if err := json.Unmarshal(neetcode150Data, &problemsJSON); err != nil {
		return nil, err
	}

	problems := make([]domain.Problem, len(problemsJSON))
	for i, p := range problemsJSON {
		problems[i] = domain.Problem{
			ID:          uuid.New(),
			Title:       p.Title,
			Slug:        p.Slug,
			Difficulty:  domain.Difficulty(p.Difficulty),
			Topics:      p.Topics,
			LeetCodeURL: p.LeetCodeURL,
			NeetCodeURL: p.NeetCodeURL,
			OrderIndex:  p.OrderIndex,
		}
	}

	return problems, nil
}
