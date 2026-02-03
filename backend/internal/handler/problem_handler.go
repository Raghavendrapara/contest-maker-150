package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/contest-maker-150/backend/internal/domain"
	"github.com/contest-maker-150/backend/internal/service"
)

// ProblemHandler handles problem-related HTTP requests
type ProblemHandler struct {
	problemService *service.ProblemService
}

// NewProblemHandler creates a new problem handler
func NewProblemHandler(problemService *service.ProblemService) *ProblemHandler {
	return &ProblemHandler{
		problemService: problemService,
	}
}

// GetProblems returns all problems
// GET /api/problems
func (h *ProblemHandler) GetProblems(c *gin.Context) {
	problems, err := h.problemService.GetAllProblems(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve problems",
		})
		return
	}

	// Convert to response format
	responses := make([]domain.ProblemResponse, len(problems))
	for i, problem := range problems {
		responses[i] = problem.ToResponse()
	}

	c.JSON(http.StatusOK, gin.H{
		"problems": responses,
		"count":    len(responses),
	})
}

// GetProblem returns a specific problem by ID
// GET /api/problems/:id
func (h *ProblemHandler) GetProblem(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid problem ID",
		})
		return
	}

	problem, err := h.problemService.GetProblemByID(c.Request.Context(), id)
	if err != nil {
		switch err {
		case domain.ErrProblemNotFound:
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Problem not found",
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retrieve problem",
			})
		}
		return
	}

	c.JSON(http.StatusOK, problem.ToResponse())
}

// GetProblemStats returns statistics about the problem set
// GET /api/problems/stats
func (h *ProblemHandler) GetProblemStats(c *gin.Context) {
	stats, err := h.problemService.GetProblemStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve problem statistics",
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}
