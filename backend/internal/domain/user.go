package domain

import (
	"time"

	"github.com/google/uuid"
)

// User represents a registered user of the platform
type User struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Email        string    `json:"email" gorm:"uniqueIndex;not null"`
	Username     string    `json:"username" gorm:"not null"`
	PasswordHash string    `json:"-" gorm:"not null"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Relationships
	Contests    []Contest    `json:"contests,omitempty" gorm:"foreignKey:UserID"`
	Submissions []Submission `json:"submissions,omitempty" gorm:"foreignKey:UserID"`
}

// TableName specifies the table name for GORM
func (User) TableName() string {
	return "users"
}

// UserRepository defines the interface for user data access
// This abstraction allows for easy testing and swapping implementations
type UserRepository interface {
	Create(user *User) error
	FindByID(id uuid.UUID) (*User, error)
	FindByEmail(email string) (*User, error)
	Update(user *User) error
	Delete(id uuid.UUID) error
	GetSolvedProblemIDs(userID uuid.UUID) ([]uuid.UUID, error)
}

// UserCreateRequest represents the data needed to create a new user
type UserCreateRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=8"`
}

// UserResponse represents the public user data returned by the API
type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

// ToResponse converts a User to a UserResponse (hides sensitive data)
func (u *User) ToResponse() UserResponse {
	return UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		Username:  u.Username,
		CreatedAt: u.CreatedAt,
	}
}

// UserProgress represents the user's overall progress statistics
type UserProgress struct {
	TotalSolved   int                    `json:"total_solved"`
	EasySolved    int                    `json:"easy_solved"`
	MediumSolved  int                    `json:"medium_solved"`
	HardSolved    int                    `json:"hard_solved"`
	TopicProgress map[string]TopicStats  `json:"topic_progress"`
	ContestStats  ContestStatistics      `json:"contest_stats"`
}

// TopicStats represents progress within a specific topic
type TopicStats struct {
	Total  int `json:"total"`
	Solved int `json:"solved"`
}

// ContestStatistics represents a user's contest history stats
type ContestStatistics struct {
	TotalContests     int `json:"total_contests"`
	CompletedContests int `json:"completed_contests"`
	AbandonedContests int `json:"abandoned_contests"`
}
