package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/contest-maker-150/backend/internal/middleware"
	"github.com/contest-maker-150/backend/internal/service"
)

// UserHandler handles user-related HTTP requests
type UserHandler struct {
	userService *service.UserService
}

// NewUserHandler creates a new user handler
func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

// GetCurrentUser returns the currently authenticated user
// GET /api/users/me
func (h *UserHandler) GetCurrentUser(c *gin.Context) {
	userID, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	user, err := h.userService.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve user",
		})
		return
	}

	c.JSON(http.StatusOK, user.ToResponse())
}

// GetUserProgress returns the user's progress statistics
// GET /api/users/me/progress
func (h *UserHandler) GetUserProgress(c *gin.Context) {
	userID, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	progress, err := h.userService.GetUserProgress(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve progress",
		})
		return
	}

	c.JSON(http.StatusOK, progress)
}
