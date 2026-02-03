package service

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/contest-maker-150/backend/internal/domain"
	"github.com/contest-maker-150/backend/internal/infrastructure"
)

// UserService handles user-related business logic
type UserService struct {
	userRepo  domain.UserRepository
	subRepo   domain.SubmissionRepository
	jwtConfig *infrastructure.JWTConfig
	tracer    trace.Tracer
	logger    *zap.Logger
}

// NewUserService creates a new user service
func NewUserService(
	userRepo domain.UserRepository,
	subRepo domain.SubmissionRepository,
	jwtConfig *infrastructure.JWTConfig,
	tracer trace.Tracer,
	logger *zap.Logger,
) *UserService {
	return &UserService{
		userRepo:  userRepo,
		subRepo:   subRepo,
		jwtConfig: jwtConfig,
		tracer:    tracer,
		logger:    logger,
	}
}

// TokenPair represents access and refresh tokens
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Register creates a new user account
func (s *UserService) Register(ctx context.Context, req *domain.UserCreateRequest) (*domain.User, *TokenPair, error) {
	ctx, span := s.tracer.Start(ctx, "UserService.Register")
	defer span.End()

	span.SetAttributes(attribute.String("user.email", req.Email))

	// Check if user already exists
	existing, err := s.userRepo.FindByEmail(req.Email)
	if err != nil && err != domain.ErrUserNotFound {
		s.logger.Error("Failed to check existing user", zap.Error(err))
		return nil, nil, err
	}
	if existing != nil {
		return nil, nil, domain.ErrUserAlreadyExists
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("Failed to hash password", zap.Error(err))
		return nil, nil, domain.ErrInternalServer
	}

	// Create user
	user := &domain.User{
		Email:        req.Email,
		Username:     req.Username,
		PasswordHash: string(hashedPassword),
	}

	if err := s.userRepo.Create(user); err != nil {
		s.logger.Error("Failed to create user", zap.Error(err))
		return nil, nil, err
	}

	// Generate tokens
	tokens, err := s.generateTokenPair(user)
	if err != nil {
		return nil, nil, err
	}

	s.logger.Info("User registered successfully",
		zap.String("user_id", user.ID.String()),
		zap.String("email", user.Email),
	)

	span.SetAttributes(attribute.String("user.id", user.ID.String()))
	return user, tokens, nil
}

// Login authenticates a user and returns tokens
func (s *UserService) Login(ctx context.Context, email, password string) (*domain.User, *TokenPair, error) {
	ctx, span := s.tracer.Start(ctx, "UserService.Login")
	defer span.End()

	span.SetAttributes(attribute.String("user.email", email))

	// Find user by email
	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		if err == domain.ErrUserNotFound {
			return nil, nil, domain.ErrInvalidCredentials
		}
		return nil, nil, err
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, nil, domain.ErrInvalidCredentials
	}

	// Generate tokens
	tokens, err := s.generateTokenPair(user)
	if err != nil {
		return nil, nil, err
	}

	s.logger.Info("User logged in",
		zap.String("user_id", user.ID.String()),
		zap.String("email", user.Email),
	)

	span.SetAttributes(attribute.String("user.id", user.ID.String()))
	return user, tokens, nil
}

// RefreshToken generates a new access token from a refresh token
func (s *UserService) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	ctx, span := s.tracer.Start(ctx, "UserService.RefreshToken")
	defer span.End()

	// Parse and validate refresh token
	claims, err := s.validateToken(refreshToken)
	if err != nil {
		return nil, domain.ErrInvalidToken
	}

	// Check token type
	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "refresh" {
		return nil, domain.ErrInvalidToken
	}

	// Get user ID from claims
	userIDStr, ok := claims["sub"].(string)
	if !ok {
		return nil, domain.ErrInvalidToken
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, domain.ErrInvalidToken
	}

	// Find user
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}

	// Generate new tokens
	return s.generateTokenPair(user)
}

// GetUserByID retrieves a user by their ID
func (s *UserService) GetUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	ctx, span := s.tracer.Start(ctx, "UserService.GetUserByID")
	defer span.End()

	span.SetAttributes(attribute.String("user.id", id.String()))
	return s.userRepo.FindByID(id)
}

// GetUserProgress retrieves the user's progress statistics
func (s *UserService) GetUserProgress(ctx context.Context, userID uuid.UUID) (*domain.UserProgress, error) {
	ctx, span := s.tracer.Start(ctx, "UserService.GetUserProgress")
	defer span.End()

	span.SetAttributes(attribute.String("user.id", userID.String()))

	// Count solved problems by difficulty (concurrent queries using goroutines)
	type countResult struct {
		difficulty domain.Difficulty
		count      int64
		err        error
	}

	difficulties := []domain.Difficulty{
		domain.DifficultyEasy,
		domain.DifficultyMedium,
		domain.DifficultyHard,
	}

	resultChan := make(chan countResult, len(difficulties))

	// Fan-out: Launch concurrent queries
	for _, diff := range difficulties {
		go func(d domain.Difficulty) {
			count, err := s.subRepo.CountByUserAndDifficulty(userID, d)
			resultChan <- countResult{difficulty: d, count: count, err: err}
		}(diff)
	}

	// Fan-in: Collect results
	progress := &domain.UserProgress{
		TopicProgress: make(map[string]domain.TopicStats),
	}

	for range difficulties {
		result := <-resultChan
		if result.err != nil {
			s.logger.Error("Failed to count submissions by difficulty",
				zap.String("difficulty", string(result.difficulty)),
				zap.Error(result.err),
			)
			continue
		}

		switch result.difficulty {
		case domain.DifficultyEasy:
			progress.EasySolved = int(result.count)
		case domain.DifficultyMedium:
			progress.MediumSolved = int(result.count)
		case domain.DifficultyHard:
			progress.HardSolved = int(result.count)
		}
	}

	progress.TotalSolved = progress.EasySolved + progress.MediumSolved + progress.HardSolved

	return progress, nil
}

// ValidateAccessToken validates an access token and returns the user ID
func (s *UserService) ValidateAccessToken(tokenString string) (uuid.UUID, error) {
	claims, err := s.validateToken(tokenString)
	if err != nil {
		return uuid.Nil, domain.ErrInvalidToken
	}

	// Check token type
	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "access" {
		return uuid.Nil, domain.ErrInvalidToken
	}

	// Get user ID from claims
	userIDStr, ok := claims["sub"].(string)
	if !ok {
		return uuid.Nil, domain.ErrInvalidToken
	}

	return uuid.Parse(userIDStr)
}

// generateTokenPair creates access and refresh tokens for a user
func (s *UserService) generateTokenPair(user *domain.User) (*TokenPair, error) {
	now := time.Now()
	accessExpiry := now.Add(s.jwtConfig.AccessTokenExpiry)
	refreshExpiry := now.Add(s.jwtConfig.RefreshTokenExpiry)

	// Generate access token
	accessClaims := jwt.MapClaims{
		"sub":   user.ID.String(),
		"email": user.Email,
		"type":  "access",
		"iat":   now.Unix(),
		"exp":   accessExpiry.Unix(),
		"iss":   s.jwtConfig.Issuer,
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(s.jwtConfig.SecretKey))
	if err != nil {
		return nil, err
	}

	// Generate refresh token
	refreshClaims := jwt.MapClaims{
		"sub":  user.ID.String(),
		"type": "refresh",
		"iat":  now.Unix(),
		"exp":  refreshExpiry.Unix(),
		"iss":  s.jwtConfig.Issuer,
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(s.jwtConfig.SecretKey))
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		ExpiresAt:    accessExpiry,
	}, nil
}

// validateToken validates a JWT token and returns its claims
func (s *UserService) validateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, domain.ErrInvalidToken
		}
		return []byte(s.jwtConfig.SecretKey), nil
	})

	if err != nil || !token.Valid {
		return nil, domain.ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, domain.ErrInvalidToken
	}

	return claims, nil
}
