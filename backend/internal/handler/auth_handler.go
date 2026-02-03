package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/contest-maker-150/backend/internal/domain"
	"github.com/contest-maker-150/backend/internal/service"
)

// AuthHandler handles authentication-related HTTP requests
type AuthHandler struct {
	userService *service.UserService
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(userService *service.UserService) *AuthHandler {
	return &AuthHandler{
		userService: userService,
	}
}

// LoginRequest represents the login request body
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// AuthResponse represents the authentication response
type AuthResponse struct {
	User   domain.UserResponse `json:"user"`
	Tokens *service.TokenPair  `json:"tokens"`
}

// Register handles user registration
// POST /api/auth/signup
func (h *AuthHandler) Register(c *gin.Context) {
	var req domain.UserCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	user, tokens, err := h.userService.Register(c.Request.Context(), &req)
	if err != nil {
		switch err {
		case domain.ErrUserAlreadyExists:
			c.JSON(http.StatusConflict, gin.H{
				"error": "User with this email already exists",
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to create user",
			})
		}
		return
	}

	c.JSON(http.StatusCreated, AuthResponse{
		User:   user.ToResponse(),
		Tokens: tokens,
	})
}

// Login handles user login
// POST /api/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	user, tokens, err := h.userService.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		switch err {
		case domain.ErrInvalidCredentials:
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid email or password",
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to login",
			})
		}
		return
	}

	c.JSON(http.StatusOK, AuthResponse{
		User:   user.ToResponse(),
		Tokens: tokens,
	})
}

// RefreshRequest represents the token refresh request body
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Refresh handles token refresh
// POST /api/auth/refresh
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	tokens, err := h.userService.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid or expired refresh token",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tokens": tokens,
	})
}
