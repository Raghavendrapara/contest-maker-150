package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/contest-maker-150/backend/internal/domain"
	"github.com/contest-maker-150/backend/internal/middleware"
	"github.com/contest-maker-150/backend/internal/service"
)

// ContestHandler handles contest-related HTTP requests
type ContestHandler struct {
	contestService *service.ContestService
}

// NewContestHandler creates a new contest handler
func NewContestHandler(contestService *service.ContestService) *ContestHandler {
	return &ContestHandler{
		contestService: contestService,
	}
}

// CreateContest creates a new contest for the authenticated user
// POST /api/contests
func (h *ContestHandler) CreateContest(c *gin.Context) {
	userID, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	var req domain.CreateContestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	contest, err := h.contestService.CreateContest(c.Request.Context(), userID, &req)
	if err != nil {
		switch err {
		case domain.ErrActiveContestExists:
			c.JSON(http.StatusConflict, gin.H{
				"error": "You already have an active contest. Complete or abandon it first.",
			})
		case domain.ErrNotEnoughProblems:
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Not enough unsolved problems available. Try with fewer problems.",
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to create contest",
			})
		}
		return
	}

	c.JSON(http.StatusCreated, contest.ToResponse())
}

// GetContests returns all contests for the authenticated user
// GET /api/contests
func (h *ContestHandler) GetContests(c *gin.Context) {
	userID, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	contests, err := h.contestService.GetUserContests(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve contests",
		})
		return
	}

	// Convert to response format
	responses := make([]domain.ContestResponse, len(contests))
	for i, contest := range contests {
		responses[i] = contest.ToResponse()
	}

	c.JSON(http.StatusOK, gin.H{
		"contests": responses,
	})
}

// GetActiveContest returns the user's active contest if any
// GET /api/contests/active
func (h *ContestHandler) GetActiveContest(c *gin.Context) {
	userID, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	contest, err := h.contestService.GetActiveContest(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve active contest",
		})
		return
	}

	if contest == nil {
		c.JSON(http.StatusOK, gin.H{
			"contest": nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"contest": contest.ToResponse(),
	})
}

// GetContest returns a specific contest by ID
// GET /api/contests/:id
func (h *ContestHandler) GetContest(c *gin.Context) {
	userID, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	contestIDStr := c.Param("id")
	contestID, err := uuid.Parse(contestIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid contest ID",
		})
		return
	}

	contest, err := h.contestService.GetContestByID(c.Request.Context(), contestID)
	if err != nil {
		switch err {
		case domain.ErrContestNotFound:
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Contest not found",
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retrieve contest",
			})
		}
		return
	}

	// Verify ownership
	if contest.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "You don't have access to this contest",
		})
		return
	}

	c.JSON(http.StatusOK, contest.ToResponse())
}

// MarkProblemComplete marks a problem as completed in a contest
// PATCH /api/contests/:id/problems/:problemId
func (h *ContestHandler) MarkProblemComplete(c *gin.Context) {
	userID, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	contestIDStr := c.Param("id")
	contestID, err := uuid.Parse(contestIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid contest ID",
		})
		return
	}

	problemIDStr := c.Param("problemId")
	problemID, err := uuid.Parse(problemIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid problem ID",
		})
		return
	}

	var req domain.MarkProblemCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	err = h.contestService.MarkProblemComplete(c.Request.Context(), userID, contestID, problemID, req.IsCompleted)
	if err != nil {
		switch err {
		case domain.ErrContestNotFound:
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Contest not found",
			})
		case domain.ErrForbidden:
			c.JSON(http.StatusForbidden, gin.H{
				"error": "You don't have access to this contest",
			})
		case domain.ErrContestNotActive:
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Contest is not active",
			})
		case domain.ErrContestExpired:
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Contest has expired",
			})
		case domain.ErrProblemNotInContest:
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Problem not found in this contest",
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to update problem status",
			})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Problem status updated",
	})
}

// CompleteContest manually completes a contest
// POST /api/contests/:id/complete
func (h *ContestHandler) CompleteContest(c *gin.Context) {
	userID, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	contestIDStr := c.Param("id")
	contestID, err := uuid.Parse(contestIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid contest ID",
		})
		return
	}

	err = h.contestService.CompleteContest(c.Request.Context(), userID, contestID)
	if err != nil {
		switch err {
		case domain.ErrContestNotFound:
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Contest not found",
			})
		case domain.ErrForbidden:
			c.JSON(http.StatusForbidden, gin.H{
				"error": "You don't have access to this contest",
			})
		case domain.ErrContestNotActive:
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Contest is not active",
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to complete contest",
			})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Contest completed",
	})
}

// AbandonContest abandons a contest
// POST /api/contests/:id/abandon
func (h *ContestHandler) AbandonContest(c *gin.Context) {
	userID, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	contestIDStr := c.Param("id")
	contestID, err := uuid.Parse(contestIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid contest ID",
		})
		return
	}

	err = h.contestService.AbandonContest(c.Request.Context(), userID, contestID)
	if err != nil {
		switch err {
		case domain.ErrContestNotFound:
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Contest not found",
			})
		case domain.ErrForbidden:
			c.JSON(http.StatusForbidden, gin.H{
				"error": "You don't have access to this contest",
			})
		case domain.ErrContestNotActive:
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Contest is not active",
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to abandon contest",
			})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Contest abandoned",
	})
}
