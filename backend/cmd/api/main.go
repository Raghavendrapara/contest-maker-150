package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/contest-maker-150/backend/internal/data"
	"github.com/contest-maker-150/backend/internal/handler"
	"github.com/contest-maker-150/backend/internal/infrastructure"
	"github.com/contest-maker-150/backend/internal/middleware"
	"github.com/contest-maker-150/backend/internal/repository"
	"github.com/contest-maker-150/backend/internal/service"
)

func main() {
	// Load configuration
	config := infrastructure.LoadConfig()

	// Initialize logger
	logger, err := infrastructure.NewLogger(config.Server.Environment)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer infrastructure.SyncLogger(logger)

	logger.Info("Starting Contest Maker 150 API",
		zap.String("environment", config.Server.Environment),
		zap.Int("port", config.Server.Port),
	)

	// Initialize context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize telemetry
	telemetry, err := infrastructure.NewTelemetry(ctx, &config.Telemetry, logger)
	if err != nil {
		logger.Error("Failed to initialize telemetry", zap.Error(err))
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		telemetry.Shutdown(shutdownCtx)
	}()

	// Create metrics
	metrics, err := telemetry.CreateMetrics()
	if err != nil {
		logger.Error("Failed to create metrics", zap.Error(err))
		os.Exit(1)
	}

	// Initialize database
	database, err := infrastructure.NewDatabase(&config.Database, logger)
	if err != nil {
		logger.Error("Failed to connect to database", zap.Error(err))
		os.Exit(1)
	}
	defer database.Close()

	// Run migrations
	if err := database.AutoMigrate(); err != nil {
		logger.Error("Failed to run migrations", zap.Error(err))
		os.Exit(1)
	}

	// Seed problems
	seeder := data.NewSeeder(database.DB, logger)
	if err := seeder.SeedProblems(); err != nil {
		logger.Error("Failed to seed problems", zap.Error(err))
		os.Exit(1)
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(database.DB)
	problemRepo := repository.NewProblemRepository(database.DB)
	contestRepo := repository.NewContestRepository(database.DB)
	submissionRepo := repository.NewSubmissionRepository(database.DB)

	// Initialize services
	userService := service.NewUserService(userRepo, submissionRepo, &config.JWT, telemetry.Tracer, logger)
	problemService := service.NewProblemService(problemRepo, userRepo, telemetry.Tracer, logger)
	contestService := service.NewContestService(contestRepo, problemService, submissionRepo, telemetry.Tracer, logger)

	// Initialize handlers
	authHandler := handler.NewAuthHandler(userService)
	userHandler := handler.NewUserHandler(userService)
	problemHandler := handler.NewProblemHandler(problemService)
	contestHandler := handler.NewContestHandler(contestService)

	// Setup Gin router
	if config.Server.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Add global middleware
	router.Use(middleware.RecoveryMiddleware(logger))
	router.Use(middleware.LoggingMiddleware(logger))
	router.Use(middleware.CORSMiddleware(middleware.DefaultCORSConfig()))
	router.Use(middleware.TracingMiddleware(telemetry.Tracer))
	router.Use(middleware.MetricsMiddleware(metrics))

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		if err := database.HealthCheck(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unhealthy",
				"error":  "database connection failed",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"version": config.Telemetry.ServiceVersion,
		})
	})

	// Metrics endpoint for Prometheus
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API routes
	api := router.Group("/api")
	{
		// Auth routes (public)
		auth := api.Group("/auth")
		{
			auth.POST("/signup", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/refresh", authHandler.Refresh)
		}

		// Problem routes (public for listing, protected for some features)
		problems := api.Group("/problems")
		{
			problems.GET("", problemHandler.GetProblems)
			problems.GET("/stats", problemHandler.GetProblemStats)
			problems.GET("/:id", problemHandler.GetProblem)
		}

		// Protected routes
		protected := api.Group("")
		protected.Use(middleware.AuthMiddleware(userService))
		{
			// User routes
			users := protected.Group("/users")
			{
				users.GET("/me", userHandler.GetCurrentUser)
				users.GET("/me/progress", userHandler.GetUserProgress)
			}

			// Contest routes
			contests := protected.Group("/contests")
			{
				contests.POST("", contestHandler.CreateContest)
				contests.GET("", contestHandler.GetContests)
				contests.GET("/active", contestHandler.GetActiveContest)
				contests.GET("/:id", contestHandler.GetContest)
				contests.PATCH("/:id/problems/:problemId", contestHandler.MarkProblemComplete)
				contests.POST("/:id/complete", contestHandler.CompleteContest)
				contests.POST("/:id/abandon", contestHandler.AbandonContest)
			}
		}
	}

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port),
		Handler:      router,
		ReadTimeout:  config.Server.ReadTimeout,
		WriteTimeout: config.Server.WriteTimeout,
	}

	// Start server in goroutine
	go func() {
		logger.Info("HTTP server starting",
			zap.String("address", server.Addr),
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}
