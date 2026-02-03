package domain

import "errors"

// Domain errors - these are business logic errors that should be translated
// to appropriate HTTP status codes by the handler layer

var (
	// User errors
	ErrUserNotFound       = errors.New("user not found")
	ErrUserAlreadyExists  = errors.New("user with this email already exists")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidToken       = errors.New("invalid or expired token")

	// Problem errors
	ErrProblemNotFound     = errors.New("problem not found")
	ErrNotEnoughProblems   = errors.New("not enough unsolved problems available")
	ErrInvalidDifficulty   = errors.New("invalid difficulty level")

	// Contest errors
	ErrContestNotFound     = errors.New("contest not found")
	ErrContestNotActive    = errors.New("contest is not active")
	ErrContestExpired      = errors.New("contest has expired")
	ErrActiveContestExists = errors.New("user already has an active contest")
	ErrProblemNotInContest = errors.New("problem not found in this contest")

	// Submission errors
	ErrSubmissionNotFound     = errors.New("submission not found")
	ErrAlreadySolved          = errors.New("problem already solved by user")

	// General errors
	ErrInternalServer = errors.New("internal server error")
	ErrBadRequest     = errors.New("bad request")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrForbidden      = errors.New("forbidden")
)

// DomainError wraps an error with additional context
type DomainError struct {
	Err     error
	Message string
	Code    string
}

func (e *DomainError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Err.Error()
}

func (e *DomainError) Unwrap() error {
	return e.Err
}

// NewDomainError creates a new DomainError with the given error and message
func NewDomainError(err error, message string) *DomainError {
	return &DomainError{
		Err:     err,
		Message: message,
	}
}

// WrapError wraps an error with additional context
func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	return &DomainError{
		Err:     err,
		Message: message,
	}
}
