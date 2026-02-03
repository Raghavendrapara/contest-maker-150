package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/contest-maker-150/backend/internal/service"
)

const (
	// AuthorizationHeader is the header key for the JWT token
	AuthorizationHeader = "Authorization"
	// BearerPrefix is the prefix for the JWT token
	BearerPrefix = "Bearer "
	// UserIDKey is the context key for the user ID
	UserIDKey = "userID"
)

// AuthMiddleware creates a new authentication middleware
func AuthMiddleware(userService *service.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader(AuthorizationHeader)
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header is required",
			})
			c.Abort()
			return
		}

		if !strings.HasPrefix(authHeader, BearerPrefix) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid authorization header format",
			})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, BearerPrefix)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Token is required",
			})
			c.Abort()
			return
		}

		userID, err := userService.ValidateAccessToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
			})
			c.Abort()
			return
		}

		// Set user ID in context for handlers to use
		c.Set(UserIDKey, userID)
		c.Next()
	}
}

// OptionalAuthMiddleware creates middleware that validates token if present but doesn't require it
func OptionalAuthMiddleware(userService *service.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader(AuthorizationHeader)
		if authHeader == "" {
			c.Next()
			return
		}

		if !strings.HasPrefix(authHeader, BearerPrefix) {
			c.Next()
			return
		}

		token := strings.TrimPrefix(authHeader, BearerPrefix)
		if token == "" {
			c.Next()
			return
		}

		userID, err := userService.ValidateAccessToken(token)
		if err == nil {
			c.Set(UserIDKey, userID)
		}

		c.Next()
	}
}

// GetUserID extracts the user ID from the gin context
func GetUserID(c *gin.Context) (uuid.UUID, bool) {
	userID, exists := c.Get(UserIDKey)
	if !exists {
		return uuid.Nil, false
	}
	id, ok := userID.(uuid.UUID)
	return id, ok
}

// RequireUser ensures a user is authenticated and returns their ID
// If not authenticated, it aborts the request
func RequireUser(c *gin.Context) (uuid.UUID, bool) {
	userID, ok := GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Authentication required",
		})
		c.Abort()
		return uuid.Nil, false
	}
	return userID, true
}
